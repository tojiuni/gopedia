# Phloem — Pipeline details (code-aligned)

## 1. gRPC surface

| RPC | Role | Key code |
|-----|------|----------|
| `IngestMarkdown` | Single-document ingest | `internal/phloem/server.go` |
| `RegisterProject` | Ensure `projects` row for `root_path` / ingest | Same; requires PostgreSQL on server |

Pipelines are registered at startup: **`wiki`** and **`code`** only (`cmd/phloem/main.go`).

## 2. Per-domain flow

### 2.1 Wiki (`domain.Wiki` = `"wiki"`)

1. **Parse**: `toc.MarkdownTOCParser` → section tree.  
2. **TOC JSON**: `toc.TOCToJSON(roots)` for `RhizomeMessage.toc`.  
3. **Chunk**: `chunker.ByHeadingChunker` from `req.Content` + roots.  
4. **Sink**: `DefaultSink.Write` with `RhizomeMessage` + chunks.

**Code**: `internal/phloem/domain/wiki.go`

### 2.2 Code (`domain.Code` = `"code"`)

1. **Language**: From `source_metadata["language"]` or filename heuristic (`detectLangFromTitle`).  
2. **Parse / chunk**: `toc.CodeTOCParser` (tree-sitter via Python, `GOPEDIA_REPO_ROOT`, `GOPEDIA_PYTHON`) + `chunker.CodeChunker`.  
3. **Sink**: same `DefaultSink.Write`.

**Code**: `internal/phloem/domain/code.go`, `internal/phloem/chunker/code.go`

### 2.3 PDF (`domain.PDF` = `"pdf"`)

`internal/phloem/domain/pdf.go` implements a PDF-oriented pipeline, but **it is not registered** in the default Phloem binary. Treat as library-ready, not product-wired.

## 3. `DefaultSink.Write` (core persistence)

**File**: `internal/phloem/sink/writer.go`

At a high level (when PostgreSQL is configured):

1. **Idempotency / dedup**: `l2_child_hash`, title+hash short-circuit, machine_id head match (see IMP-01 style logic near the start of `Write`).  
2. **`documents`**: upsert by `machine_id`; captures `project_id` from metadata when present.  
3. **`knowledge_l1`**: new revision row with TOC, summary, `l2_child_hash`, etc.  
4. **`knowledge_l2` / `knowledge_l3`**: per chunk; optional **NLP worker** for some L2 types (`GOPEDIA_NLP_WORKER_GRPC_ADDR`), else local splitting.  
5. **Qdrant**: L3 (and related) vectors with payload including `l3_id`, `l2_id`, `project_id`, `doc_id` as implemented in writer — used by Xylem for retrieval.  
6. **Redis / Tuber**: optional keyword SoT cache (`tuberGetOrCreate`, `kw:*` keys) when Redis is configured.

**Optional components**: If `pg`, `qdrant`, or `embed` is nil, the corresponding steps are skipped (service can “succeed” with limited persistence; see run/troubleshooting docs).

## 4. Environment (representative)

| Variable | Effect |
|----------|--------|
| `GOPEDIA_PHLOEM_GRPC_ADDR` | Listen address (default `:50051`) |
| `POSTGRES_*` / DSN | PostgreSQL via `env.PostgresConnString()` |
| `QDRANT_HOST`, `QDRANT_GRPC_PORT` / `QDRANT_PORT` | Qdrant gRPC client |
| `QDRANT_COLLECTION` | Default collection name (e.g. `gopedia_markdown`) |
| `REDIS_HOST` | Tuber cache |
| `GOPEDIA_NLP_WORKER_GRPC_ADDR` | Optional NLP gRPC for L3 splits |
| `GOPEDIA_REPO_ROOT`, `GOPEDIA_PYTHON` | Code tree-sitter path |

## 5. Gaps to document or close (engineering checklist)

- **Register PDF** (or remove unused pipeline from shipping scope).  
- **Document ingest-time embedding model** next to Qdrant collection so operators match Xylem `resolve_retrieval_settings` / `GOPEDIA_EMBEDDING_*`.  
- **TypeDB**: either document a separate sync job or add to sink if graph RAG becomes first-class.  
- **Index reset** for automated eval (see `doc/design/plan/TODO.md`).

## 6. Cross-link: Rev4

Rev4 describes **target** chunking and metadata strategies (`doc/design/Rev4/`). When Rev4 and `writer.go` differ, **code wins** for behavior; use Rev4 as the design delta list.
