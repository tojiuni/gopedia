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
| POST | `/api/ingest` | Synchronous ingest (body `{"path":"..."}`) |
| POST | `/api/ingest/jobs` | Async ingest job |
| GET | `/api/jobs/{id}` | Job status |

## Search JSON: `detail` presets and `fields`

Only applies when `format=json`. Markdown responses ignore `detail` and `fields`.

| Query | Meaning |
|-------|---------|
| *(omit)* or `detail=full` | Full hit objects (all keys; same as before this feature). Includes `surrounding_context` when present. |
| `detail=summary` | Compact hits: `doc_id`, `l3_id`, `score`, `title`, `snippet`, `source_path`. |
| `detail=standard` | `summary` plus `project_id`, `l1_id`, `l2_id`, `section_heading`, `breadcrumb`. |
| `fields=a,b,c` | Sparse fieldset (comma-separated JSON keys). **Overrides** `detail` when non-empty. |

Allowed `fields` keys: `doc_id`, `project_id`, `l1_id`, `l2_id`, `l3_id`, `score`, `title`, `section_heading`, `snippet`, `source_path`, `breadcrumb`, `surrounding_context`.

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
- `gopedia ingest path/to/dir --json` — prints the full JSON body from `POST /api/ingest`.

## Backward compatibility

- Omitting `format` on search keeps **markdown**-oriented responses (`markdown` field).
- Omitting `detail` and `fields` with `format=json` keeps the previous **full** `results[]` shape (all `SearchHit` fields).
- Existing clients that expect non-200 on failure should be updated: ingest/search failures may return **200** with `ok: false` / `failure` populated.

See also [run.md](run.md) for stack bring-up.
