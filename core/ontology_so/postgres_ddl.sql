-- Gopedia 0.0.1: document master table (Rhizome).
-- Run once per database (e.g. psql -f postgres_ddl.sql).

CREATE TABLE IF NOT EXISTS documents (
  id BIGINT PRIMARY KEY,
  machine_id BIGINT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  source_metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_documents_machine_id ON documents (machine_id);
