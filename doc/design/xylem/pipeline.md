# Xylem — Pipeline details (code-aligned)

## 1. `retrieve_and_enrich` stages

**File**: `flows/xylem_flow/retriever.py`

1. **Project metadata** (if `project_id` set): `fetch_project_source_metadata(conn, project_id)` for Qdrant host/port/collection/vector name and embedding hints.  
2. **Resolve settings**: `resolve_retrieval_settings(...)` merges env, metadata, and call arguments (`flows/xylem_flow/project_config.py`).  
3. **Query vector**:  
   - `embedding_backend == "local"` → `embed_query_local` → HTTP `LOCAL_EMBEDDING_ADDR` (default `http://localhost:18789`), **prefix `query`**.  
   - Else → `embed_query_openai` using `OPENAI_API_KEY` and `OPENAI_EMBEDDING_MODEL` (or arg).  
4. **Dense retrieval**: `qdrant_search_l3_points` with `limit=candidate_limit` (default **30**), optional `using=vector_name`, optional `project_id` **payload filter** on `project_id`.  
5. **Build rows**: Each Qdrant point must have `l3_id` in payload; pairs `(point, l3_id)` collected.  
6. **Rerank (optional)**: If `use_reranker` and rows non-empty:  
   - Load texts from PG: `L3_BATCH_CONTENT_SQL` by UUID list.  
   - `CrossEncoder(model_name).predict` on `(query, content)` pairs.  
   - Sort by score descending.  
   - Default model: `BAAI/bge-reranker-v2-m3` or `GOPEDIA_RERANKER_MODEL` / `reranker_model` arg.  
7. **Truncate**: `rows[:final_limit]` (`final_limit` default 5; legacy `limit=` overrides).  
8. **Enrich**: For each remaining hit, `fetch_rich_context(conn, l3_id, neighbor_window, context_level, max_tokens, ...)`, merge Qdrant payload fields (`doc_id`, `project_id`, `l2_id`, `qdrant_score`).

## 2. `fetch_rich_context`

- **SQL**: `CONTEXT_FOR_L3_SQL` joins `knowledge_l3` → `knowledge_l2` → `knowledge_l1` and optional title L3.  
- **Neighbors**: `NEIGHBORS_SQL` for `sort_order` window within the same L2.  
- **`context_level`**: 0 = matched L3 only; 1/3 = neighbor window; 2 = tighter span + L2 summary emphasis; level ≥3 adds L1 summary + TOC.  
- **Token budget**: If `max_tokens` set, trims `surrounding_context` with `tiktoken` when available.  
- **Code vs markdown**: `source_path` / `doc_name` derived from `l1_source_type` and L2 `source_metadata`.

## 3. CLI (`flows/xylem_flow/cli.py`)

- **`search`**: Wires `retrieve_and_enrich` with `--limit`, `--neighbor-window`, `--context-level`, `--project-id`, `--reranker`, `--reranker-model`.  
- **JSON path**: For machine-readable output used by the Go API when `format=json`.  
- **`restore`**: Reconstructs markdown from PG via `restorer` (separate from search).

## 4. HTTP bridge

**File**: `internal/api/api.go` — `GET /api/search`

- Builds: `search --query <q> --format markdown|json` plus optional `--project-id`, `--limit` (from `top_k`), `--reranker`, `--reranker-model`.  
- Does **not** pass `candidate_limit`, `neighbor_window`, or `context_level` today (CLI defaults apply unless extended).

## 5. Restorer (post-retrieval, optional)

**File**: `flows/xylem_flow/restorer.py` — used for `restore` and optional `--restore-l1` in CLI to rebuild full L1 markdown from structured rows.

## 6. Gaps vs Rev4 (explicit)

- **No** implementation of “metadata-aware rerank” weights from `../Rev4/03-atomic-l3-metadata-strategy.md` inside `rerank_candidates`.  
- **No** hierarchical multi-vector search as in Rev4 Phase 3 unless added to `qdrant_search_l3_points` and ingest.

## 7. Cross-links

- Subprocess / rerank cost: `doc/design/plan/TODO.md` § Cross-Encoder.  
- Retrieval quality ideas: same file § chunking, embedding, hybrid search.
