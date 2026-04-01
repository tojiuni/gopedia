# Agent interoperability (API contracts)

Gopedia exposes HTTP endpoints intended for **AI agents** and automation: structured errors, JSON search results (including staged / sparse fields), async ingest jobs, and dependency health.

## Recommended call sequence

1. `GET /api/health` — process liveness.
2. `GET /api/health/deps` — Postgres, Qdrant, TypeDB, Phloem gRPC readiness (`status`: `ok` | `degraded` when any dependency reports `error`).
3. `POST /api/ingest/jobs` — start long-running ingest; send header `Idempotency-Key` for safe retries.
4. `GET /api/jobs/{id}` — poll until `status` is `completed` or `failed`.
5. `GET /api/search?q=...&format=json` — machine-readable hits (`results[]`). For lower token use, start with `detail=summary` (see below), then repeat with `detail=standard` or `detail=full` only when you need more context.

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/health` | Liveness |
| GET | `/api/health/deps` | Dependency checks with latency |
| GET | `/api/search?q=&format=markdown\|json&project_id=&detail=&fields=` | Search (default `format` = markdown). When `format=json`, optional `detail` / `fields` shape `results[]`. |
| GET | `/api/restore?l1_id=&l2_id=&format=markdown\|json` | Restore stored content from PostgreSQL (`l1_id` full content or `l2_id` section/code). Exactly one id required. |
| POST | `/api/ingest` | Synchronous ingest (body `{"path":"..."}`) |
| POST | `/api/ingest/jobs` | Async ingest job |
| GET | `/api/jobs/{id}` | Job status |

## Search JSON: `detail` presets and `fields`

Only applies when `format=json`. Markdown responses ignore `detail` and `fields`.

| Query | Meaning |
|-------|---------|
| *(omit)* or `detail=full` | Full hit objects (all keys; same as before this feature). Includes `surrounding_context` when present. |
| `detail=summary` | Compact hits: `doc_id`, `doc_name`, `l3_id`, `score`, `title`, `snippet`, `source_path`. |
| `detail=standard` | `summary` plus `project_id`, `l1_id`, `l2_id`, `section_heading`, `breadcrumb`. |
| `fields=a,b,c` | Sparse fieldset (comma-separated JSON keys). **Overrides** `detail` when non-empty. |

Allowed `fields` keys: `doc_id`, `project_id`, `doc_name`, `l1_id`, `l2_id`, `l3_id`, `score`, `title`, `section_heading`, `snippet`, `source_path`, `breadcrumb`, `surrounding_context`.

Invalid `detail` or unknown `fields` key → **HTTP 400** with Fuego error body.

### Field notes

- **`snippet`**: Short excerpt for quick relevance checks (from `matched_content` when present, otherwise from `surrounding_context`), trimmed to a max length with sentence / word boundary preference when possible.
- **`source_path`**: Included in both `summary` and `standard` so agents can locate the file without pulling full context.

### Examples

Full JSON (default, backward compatible):

```bash
curl -s "http://127.0.0.1:18787/api/search?q=Introduction&format=json"
```

Staged — summary first:

```bash
curl -s "http://127.0.0.1:18787/api/search?q=Introduction&format=json&detail=summary"
```

Custom sparse fields (`fields` wins over `detail`):

```bash
curl -s "http://127.0.0.1:18787/api/search?q=Introduction&format=json&fields=title,snippet,l3_id,score"
```

## Restore API

Restore full content from PostgreSQL snapshots without re-embedding.

```bash
# Restore full L1 content (markdown response)
curl -s "http://127.0.0.1:18787/api/restore?l1_id=<L1_UUID>"

# Restore a specific L2 section (usually code section) as JSON
curl -s "http://127.0.0.1:18787/api/restore?l2_id=<L2_UUID>&format=json"
```

## Structured errors

On subprocess failures, ingest and search return **HTTP 200** with a JSON body that includes:

- `error` (string, legacy human message)
- `failure` (object, when present): `code`, `message`, `details`, `retryable`, `request_id`

Use `failure.retryable` for backoff retries (most ingest/search failures are `false`).

Pass `X-Request-ID` to correlate logs; the response echoes `request_id` when generated.

## CLI (`gopedia`)

- `gopedia search "query" --json` — prints the JSON body from `GET /api/search?format=json` (full `results` by default).
- `gopedia search "query" --json --detail summary` — same with `detail=summary`.
- `gopedia search "query" --json --fields title,snippet,l3_id` — sparse `fields` (overrides `--detail`).
- `gopedia restore --l1-id <uuid>` — restore full content for one `knowledge_l1` snapshot.
- `gopedia restore --l2-id <uuid> --json` — restore one `knowledge_l2` section as JSON payload.
- `gopedia ingest path/to/dir --json` — prints the full JSON body from `POST /api/ingest`.

## Backward compatibility

- Omitting `format` on search keeps **markdown**-oriented responses (`markdown` field).
- Omitting `detail` and `fields` with `format=json` keeps the previous **full** `results[]` shape (all `SearchHit` fields).
- Existing clients that expect non-200 on failure should be updated: ingest/search failures may return **200** with `ok: false` / `failure` populated.

See also [run.md](run.md) for stack bring-up.

---

## Code domain — ingest and search for source files

Gopedia Phase 1 adds a **code domain** pipeline. Source files (`.py`, `.go`, `.ts`, …) are ingested as `source_type=code` and each source line becomes one L3 row with structural metadata.

### Ingesting code files

```bash
# Via gRPC CLI (sets domain=code, source_type=code automatically)
GOPEDIA_PHLOEM_GRPC_ADDR=localhost:50051 \
python -m property.root_props.run_code path/to/file.py

# Via HTTP API (language inferred from extension)
curl -s -X POST http://127.0.0.1:18787/api/ingest \
  -H "Content-Type: application/json" \
  -d '{"path":"internal/phloem/chunker/code.go"}'
```

### Recognising code hits in search results

Code hits return `source_type: "code"` in the Qdrant payload. Use the `title` field (absolute file path) and `snippet` (matched anchor line — function/class definition) to identify them:

```json
{
  "title":              "/app/internal/phloem/chunker/code.go",
  "snippet":            "type CodeChunker struct {",
  "score":              0.745,
  "surrounding_context":"type CodeChunker struct {\n\tParser *toc.CodeTOCParser\n}"
}
```

Only anchor lines (function/class definitions) are embedded in Qdrant. `surrounding_context` contains neighboring lines from the same L2 chunk.

### Recommended call pattern for agents reading code

```bash
# 1. Find the function — summary detail is cheapest
curl -s "http://127.0.0.1:18787/api/search?q=<query>&format=json&detail=standard&fields=title,snippet,l2_id,l3_id,score"

# 2. Read full context from the hit (surrounding_context already in full detail)
curl -s "http://127.0.0.1:18787/api/search?q=<query>&format=json"
```

To reconstruct the full original file or a single function from PostgreSQL (no re-embedding needed), use the Python restore helpers — see [code-domain.md](code-domain.md#restoring-code-from-postgresql).

### `source_metadata` on L3 rows (Postgres)

Each `knowledge_l3` row for code files carries:

```json
{ "line_num": 26, "node_type": "function_definition", "is_anchor": true, "is_block_start": true }
```

Agents can query anchors directly:

```sql
SELECT content, source_metadata->>'line_num' AS line
FROM knowledge_l3
WHERE l2_id = '<l2_uuid>'
  AND (source_metadata->>'is_anchor')::bool = true
ORDER BY sort_order;
```

For full documentation on the code domain (architecture, quality testing with Gardener, restore API, SQL queries): [code-domain.md](code-domain.md).
