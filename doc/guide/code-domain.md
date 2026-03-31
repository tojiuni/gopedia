# Code domain: ingest and search

Gopedia Phase 1 extends Phloem and Xylem to handle **source code files** (Python, Go, TypeScript, etc.) alongside markdown. Each source file becomes a single `knowledge_l1` row; each function/class boundary becomes an L2 chunk; each source line becomes one L3 row with structural metadata.

---

## Architecture overview

```
Source file  →  CodeTOCParser (tree-sitter via Python subprocess)
             →  CodeChunker (L2 per top-level anchor boundary)
             →  Sink.insertCodeL3Lines (1 line = 1 L3, parent_id hierarchy)
             →  Qdrant (anchor lines only — selective embedding)
```

### L1 / L2 / L3 mapping

| Layer | Content | Key fields |
|-------|---------|------------|
| `knowledge_l1` | One row per source file | `title` = absolute file path, `source_type` = `code` |
| `knowledge_l2` | One chunk per top-level anchor (function / class / file preamble) | `section_id` = `pre` / `fn1` / `fn2` … |
| `knowledge_l3` | One row per source line (including blank lines) | `sort_order` = `line_num * 1000`, `source_metadata` JSONB, `parent_id` |

### `knowledge_l3.source_metadata` schema

```json
{
  "line_num":       26,
  "node_type":      "function_definition",
  "is_anchor":      true,
  "is_block_start": true
}
```

| Key | Type | Meaning |
|-----|------|---------|
| `line_num` | int | 1-based original line number in the source file |
| `node_type` | string | tree-sitter node type (`function_definition`, `comment`, `empty_line`, …) |
| `is_anchor` | bool | `true` for function/class definition lines — **only these lines get Qdrant vectors** |
| `is_block_start` | bool | `true` for the first line of a compound statement block |

### `parent_id` hierarchy

Inner lines point to their enclosing anchor. For a function `def foo():` at line 10, all subsequent indented lines have `parent_id` = UUID of the `def foo():` L3 row. Top-level anchors have `parent_id = NULL`.

```
L3: "def _pg_connect():"   → parent_id = NULL   (is_anchor=true)
L3: "    import psycopg"   → parent_id = <uuid of "def _pg_connect():">
L3: "    return …"         → parent_id = <uuid of "def _pg_connect():">
```

---

## Ingesting code files

### Via `property.root_props.run_code` (gRPC)

```bash
# Single file
GOPEDIA_PHLOEM_GRPC_ADDR=localhost:50051 \
POSTGRES_HOST=127.0.0.1 POSTGRES_USER=admin_gopedia \
POSTGRES_PASSWORD=<pw> POSTGRES_DB=gopedia \
python -m property.root_props.run_code path/to/file.py

# Whole directory (recursive — all .py .go .ts .tsx .js .jsx .java .rs .cpp .c .h)
python -m property.root_props.run_code path/to/src/
```

Environment variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `GOPEDIA_PHLOEM_GRPC_ADDR` | `localhost:50051` | Phloem gRPC endpoint |
| `GOPEDIA_PROJECT_ID` | (unset) | Associate ingest with a project ID |

The script auto-detects language from file extension and sets `domain=code`, `source_type=code`, `language=<lang>` in `source_metadata`.

### Via HTTP API (synchronous)

The HTTP ingest endpoint also routes to CodePipeline when the file path is passed — language and domain are inferred from the extension inside the container:

```bash
curl -s -X POST http://127.0.0.1:18787/api/ingest \
  -H "Content-Type: application/json" \
  -d '{"path":"scripts/verify_xylem_flow.py"}'
```

### Supported languages

| Extension | Language tag | Notes |
|-----------|-------------|-------|
| `.py` | `python` | tree-sitter-python |
| `.go` | `go` | tree-sitter-go |
| `.ts` `.tsx` | `typescript` | tree-sitter-typescript |
| `.js` `.jsx` | `typescript` | covered by typescript grammar |
| `.java` `.rs` `.cpp` `.c` `.h` | `python` (fallback) | regex fallback (no tree-sitter grammar yet) |

---

## Searching code

Code chunks and markdown chunks share the same search endpoint.

```bash
# Semantic search
curl -s "http://127.0.0.1:18787/api/search?q=postgres+connection&format=json"

# Filter to a specific project
curl -s "http://127.0.0.1:18787/api/search?q=CodeChunker&format=json&project_id=42"
```

### Identifying code hits

Code search hits have `source_type: "code"` in the Qdrant payload. In the JSON response, use `title` (absolute file path) and `snippet` (matched anchor line) to identify them:

```json
{
  "title":   "/app/scripts/verify_xylem_flow.py",
  "snippet": "def _pg_connect():",
  "score":   0.71,
  "surrounding_context": "def _pg_connect():\n\nimport psycopg\n\nreturn psycopg.connect(…)"
}
```

`surrounding_context` contains neighboring L3 lines within the same L2 chunk (controlled by `neighbor_window` in the retriever).

### Detail presets for code

| `detail` | Useful for |
|----------|-----------|
| `summary` | File path + function signature; low token use |
| `standard` | Adds `l2_id`, `section_heading`, `breadcrumb`; enough to fetch full chunk via restore |
| `full` (default) | Includes `surrounding_context` for reading the implementation |

```bash
# Get function name + file path only
curl -s "http://127.0.0.1:18787/api/search?q=CodeChunker&format=json&detail=summary"

# Get L2 ID to fetch full function body
curl -s "http://127.0.0.1:18787/api/search?q=CodeChunker&format=json&detail=standard&fields=title,snippet,l2_id,score"
```

---

## Restoring code from PostgreSQL

Three restore functions are available in `flows/xylem_flow/restorer.py`:

### 1. Full file restore — `restore_content_for_l1`

Reconstructs the exact original source file from all L3 lines.

```python
from flows.xylem_flow.restorer import restore_content_for_l1

result = restore_content_for_l1(conn, l1_id)
# result["content"]      → full source text (byte-exact reconstruction)
# result["source_type"]  → "code"
# result["title"]        → file path
# result["toc"]          → list of TOC entries (functions/classes)
```

For `source_type="code"`, all L3 rows under the L1 are joined by `\n` in `sort_order` order — no markdown formatting applied.

### 2. Function / section restore — `restore_code_for_l2`

Returns source lines for one L2 chunk (one function or class).

```python
from flows.xylem_flow.restorer import restore_code_for_l2

code = restore_code_for_l2(conn, l2_id)
# returns: "def _pg_connect():\n    import psycopg\n    return …"
```

### 3. Snippet for a search hit — `fetch_code_snippet`

Given an `l3_id` from a search hit, walks the `parent_id` chain upward to the nearest top-level anchor and returns up to `max_lines` lines of context.

```python
from flows.xylem_flow.restorer import fetch_code_snippet

snippet = fetch_code_snippet(conn, l3_id, max_lines=20)
```

Use this to show readable code context around a search hit without fetching the entire file.

---

## Verify ingested structure (SQL)

```sql
-- L1/L2/L3 counts per file
SELECT k1.title, k1.source_type,
       COUNT(DISTINCT l2.id) AS l2_chunks,
       COUNT(l3.id)           AS l3_lines,
       COUNT(l3.id) FILTER (WHERE (l3.source_metadata->>'is_anchor')::bool) AS anchors
FROM knowledge_l1 k1
JOIN knowledge_l2 l2 ON l2.l1_id = k1.id
JOIN knowledge_l3 l3 ON l3.l2_id = l2.id
WHERE k1.source_type = 'code'
GROUP BY k1.id, k1.title, k1.source_type;

-- parent_id wiring: inner lines of a function
SELECT l3.content,
       l3.source_metadata->>'line_num'  AS line,
       l3.source_metadata->>'is_anchor' AS anchor,
       p.content AS parent_line
FROM knowledge_l3 l3
LEFT JOIN knowledge_l3 p ON l3.parent_id = p.id
WHERE l3.l2_id = '<l2_uuid>'
ORDER BY l3.sort_order;

-- Qdrant vectors created (anchor lines only)
SELECT COUNT(*) FROM knowledge_l3
WHERE l2_id IN (SELECT id FROM knowledge_l2 WHERE l1_id = '<l1_uuid>')
  AND qdrant_point_id IS NOT NULL;
```

---

## Quality testing with Gardener Gopedia

[Gardener Gopedia](../../../../../../dev/gardener_gopedia/README.md) (`/Users/dong-hoshin/Documents/dev/gardener_gopedia`) is the evaluation harness for search quality. The steps below create a code-domain dataset and run IR metrics.

### 1. Ingest target code files

From the gopedia repo root, ingest the files you want to evaluate:

```bash
GOPEDIA_PHLOEM_GRPC_ADDR=localhost:50051 \
POSTGRES_HOST=127.0.0.1 POSTGRES_USER=admin_gopedia \
POSTGRES_PASSWORD=changeme_local_only POSTGRES_DB=gopedia \
python -m property.root_props.run_code scripts/verify_xylem_flow.py

python -m property.root_props.run_code internal/phloem/chunker/code.go
```

Verify with:
```bash
curl -s "http://127.0.0.1:18787/api/search?q=pg_connect&format=json&detail=summary" | python3 -m json.tool
```

### 2. Author a code dataset for Gardener

Create `dataset/code_domain_smoke.json` in the gardener repo:

```json
{
  "name": "code_domain_smoke",
  "version": "1",
  "curation_tier": "bronze",
  "queries": [
    {
      "external_id": "q-pg-connect",
      "text": "postgres connection function",
      "tier": "easy"
    },
    {
      "external_id": "q-code-chunker",
      "text": "CodeChunker struct definition",
      "tier": "easy"
    },
    {
      "external_id": "q-l3-lines",
      "text": "1 line = 1 L3 code line ingestion",
      "tier": "medium"
    },
    {
      "external_id": "q-restore",
      "text": "restore original source code from postgres",
      "tier": "medium"
    }
  ],
  "qrels": [
    {
      "query_external_id": "q-pg-connect",
      "target_data": {
        "excerpt": "def _pg_connect():",
        "source_path_hint": "verify_xylem_flow.py"
      },
      "target_type": "l3_id",
      "relevance": 1
    },
    {
      "query_external_id": "q-code-chunker",
      "target_data": {
        "excerpt": "type CodeChunker struct",
        "source_path_hint": "chunker/code.go"
      },
      "target_type": "l3_id",
      "relevance": 1
    },
    {
      "query_external_id": "q-l3-lines",
      "target_data": {
        "excerpt": "L3Lines []CodeLine",
        "source_path_hint": "chunker/code.go"
      },
      "target_type": "l3_id",
      "relevance": 1
    },
    {
      "query_external_id": "q-restore",
      "target_data": {
        "excerpt": "restore_code_for_l2",
        "source_path_hint": "verify_xylem_flow.py"
      },
      "target_type": "l3_id",
      "relevance": 1
    }
  ]
}
```

> **`target_data` tips for code qrels:**
> - `excerpt` — use the exact anchor line (function signature or struct definition); this is what Qdrant matched.
> - `source_path_hint` — filename substring; helps Gardener disambiguate when the same text appears in multiple files.
> - Do **not** include leading whitespace in `excerpt` for inner lines — the L3 `content` is stored with original indentation.

### 3. Register dataset and resolve qrels

```bash
cd /Users/dong-hoshin/Documents/dev/gardener_gopedia

# Start Gardener (if not running)
export GARDENER_GOPEDIA_BASE_URL=http://127.0.0.1:18787
export GARDENER_DATABASE_URL=postgresql+psycopg://admin_gopedia:changeme_local_only@127.0.0.1:5432/gopedia
uvicorn gardener_gopedia.main:app --host 0.0.0.0 --port 18880 &

# Register dataset
DATASET_ID=$(curl -s -X POST http://127.0.0.1:18880/datasets \
  -H "Content-Type: application/json" \
  -d @dataset/code_domain_smoke.json | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

echo "dataset_id=$DATASET_ID"

# Resolve target_data → l3_id via Gopedia search
curl -s -X POST "http://127.0.0.1:18880/datasets/$DATASET_ID/resolve-qrels" | python3 -m json.tool
```

### 4. Run evaluation

```bash
RUN_ID=$(curl -s -X POST http://127.0.0.1:18880/runs \
  -H "Content-Type: application/json" \
  -d "{\"dataset_id\":\"$DATASET_ID\",\"top_k\":10}" \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

# Poll until done
curl -s "http://127.0.0.1:18880/runs/$RUN_ID" | python3 -m json.tool
```

Key metrics to check in `metrics`:

| Metric | Good baseline |
|--------|--------------|
| `recall_5` | ≥ 0.75 for `easy` tier code queries |
| `mrr` | ≥ 0.70 |
| `ndcg_10` | ≥ 0.70 |

### 5. Smoke test (quick)

```bash
cd /Users/dong-hoshin/Documents/dev/gardener_gopedia
gardener-smoke
```

This exercises the Gardener → Gopedia round-trip. With code files ingested, the smoke test will pick up code hits alongside markdown hits in its built-in queries.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `no pipeline registered for domain: code` | API container built before `api.go` fix | Rebuild image: `docker-compose -f docker-compose.dev.yml --env-file .env --profile app build gopedia` |
| `source_type = "md"` on code L1 | Old image without sink fix | Rebuild + re-ingest |
| `ModuleNotFoundError: No module named 'grpc'` | Missing deps in local venv | `pip install grpcio grpcio-tools protobuf` |
| Protobuf version error | Mismatch between generated proto and runtime | `pip install "protobuf>=7.34"` |
| Anchor count = 0 | tree-sitter grammar not installed | `pip install tree-sitter-python tree-sitter-go` in the container |
| `restore_content_for_l1` returns code wrapped in ` ``` ` | Running old `restorer.py` | Pull latest; the code path was added in commit `4563745` |
| Gardener `resolve-qrels` returns 0 resolved | Code files not yet ingested | Run `python -m property.root_props.run_code <file>` first |
