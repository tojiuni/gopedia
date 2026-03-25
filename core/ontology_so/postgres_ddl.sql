-- Gopedia Rhizome DDL: UUID primary keys for documents / knowledge_l1 / knowledge_l2 / knowledge_l3.
-- documents.machine_id (BIGINT) is the ingestion identity from identity_so; documents.id is the canonical UUID.
-- Column semantics and design context: doc/design/05-storage-and-payloads.md (also 01-overview, 02-idempotency-and-tuber).
-- Apply on fresh DB (e.g. psql -f postgres_ddl.sql). For dev reset use scripts/reset_rhizome_docker.py.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS pipeline_version (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  bytea_metadata JSONB NOT NULL DEFAULT '{}',
  preprocessing_metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Document root: one row per ingest; id is the canonical doc UUID (IngestResponse.doc_id).
CREATE TABLE IF NOT EXISTS documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  machine_id BIGINT NOT NULL UNIQUE,
  title TEXT NOT NULL DEFAULT '',
  source_metadata JSONB DEFAULT '{}',
  version INT NOT NULL DEFAULT 1,
  version_id BIGINT REFERENCES pipeline_version(id),
  project_id BIGINT,
  source_type TEXT NOT NULL DEFAULT 'md',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_documents_project_id ON documents (project_id) WHERE project_id IS NOT NULL;

-- current_l1_id is added after knowledge_l1 exists (FK). See bottom of this file.

-- L1: logical document / folder root (single-table model per design).
CREATE TABLE IF NOT EXISTS knowledge_l1 (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  parent_id UUID REFERENCES knowledge_l1(id) ON DELETE SET NULL,
  project_id BIGINT,
  source_type TEXT NOT NULL DEFAULT 'md',
  title TEXT NOT NULL DEFAULT '',
  source_metadata JSONB NOT NULL DEFAULT '{}',
  version_id BIGINT REFERENCES pipeline_version(id),
  summary BYTEA,
  summary_hash BYTEA,
  toc JSONB NOT NULL DEFAULT '[]'::jsonb,
  l2_child_hash BYTEA,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  modified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_l1_document_id ON knowledge_l1 (document_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_l1_project_id ON knowledge_l1 (project_id) WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_knowledge_l1_source_type ON knowledge_l1 (source_type);

-- L2: sections / headers under L1.
CREATE TABLE IF NOT EXISTS knowledge_l2 (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  l1_id UUID NOT NULL REFERENCES knowledge_l1(id) ON DELETE CASCADE,
  parent_id UUID REFERENCES knowledge_l2(id) ON DELETE CASCADE,
  summary TEXT NOT NULL DEFAULT '',
  version INT NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  section_id TEXT NOT NULL DEFAULT '',
  version_id BIGINT REFERENCES pipeline_version(id),
  summary_bin BYTEA,
  summary_hash BYTEA,
  l3_child_hash BYTEA,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  modified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_l2_l1_id ON knowledge_l2 (l1_id);

-- L3: atomic content (sentence, etc.).
CREATE TABLE IF NOT EXISTS knowledge_l3 (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  l2_id UUID NOT NULL REFERENCES knowledge_l2(id) ON DELETE CASCADE,
  parent_id UUID REFERENCES knowledge_l3(id) ON DELETE SET NULL,
  content TEXT NOT NULL DEFAULT '',
  content_hash TEXT,
  version INT NOT NULL DEFAULT 1,
  version_id BIGINT REFERENCES pipeline_version(id),
  sort_order INT NOT NULL DEFAULT 1000,
  qdrant_point_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  modified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_l3_l2_id ON knowledge_l3 (l2_id);

-- keyword_so: Tuber keyword entity (keyword text -> machine_id). Not the same namespace as documents.machine_id.
CREATE TABLE IF NOT EXISTS keyword_so (
  machine_id BIGINT PRIMARY KEY,
  canonical_name TEXT NOT NULL DEFAULT '',
  wikidata_id TEXT NOT NULL DEFAULT '',
  aliases TEXT[] NOT NULL DEFAULT '{}',
  lang TEXT NOT NULL DEFAULT '',
  -- Full SHA-256 digest of the normalized key (e.g. kw:<lowercase keyword>) used to derive machine_id from the first 8 bytes.
  -- Purpose: detect suspected machine_id collisions, audit rows, and re-verify after changing derivation/normalization rules by comparing stored digest vs recomputed hash.
  content_hash_bin BYTEA,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  modified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON COLUMN keyword_so.content_hash_bin IS
'정규화 키워드 입력(예: prefix ''kw:'' + 소문자)에 대한 SHA-256 전체 32바이트. PK machine_id는 이 해시 앞 8바이트에서 파생된다. machine_id 충돌 의심·파생 규칙 변경 후 재검증·감사 시 저장된 전체 해시와 재계산 값을 비교하는 데 쓴다.';

-- Revision head (doc/design §1.3 option C): which knowledge_l1 row is the active snapshot for this documents row.
ALTER TABLE documents
  ADD COLUMN IF NOT EXISTS current_l1_id UUID REFERENCES knowledge_l1(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_documents_current_l1_id ON documents (current_l1_id) WHERE current_l1_id IS NOT NULL;
