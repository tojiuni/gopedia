# Run guide

Start the local stack, initialize databases, and exercise the API and CLI.

All commands assume the **repository root** as the current working directory unless noted.

## 1. Environment file

```bash
cp .env.local.example .env
```

Required for [`docker-compose.dev.yml`](../../docker-compose.dev.yml):

- `POSTGRES_PASSWORD`
- `OPENAI_API_KEY`

Keep **service hostnames** as `postgres_db`, `typedb`, `qdrant` when the API runs **inside** Compose on network `gopedia-dev`. For host-only processes talking to published ports, switch those to `127.0.0.1` (see [`.env.local.example`](../../.env.local.example) comments).

Set:

```bash
export DOCKER_NETWORK_EXTERNAL=gopedia-dev
```

before running shell scripts that use `docker run --network …` (for example [`scripts/run_fresh_ingestion_docker.sh`](../../scripts/run_fresh_ingestion_docker.sh)).

> **Note:** `scripts/run_ingestion_docker.sh` hardcodes the network name `neunexus` and does **not** respect `DOCKER_NETWORK_EXTERNAL`. Use `run_fresh_ingestion_docker.sh` instead for `gopedia-dev` setups.

**`QDRANT_COLLECTION`:** use one value consistently (for example `gopedia_markdown`) across `.env`, compose, and scripts to avoid mismatches.

---

## 2. Start Docker services

### Databases only

```bash
docker compose -f docker-compose.dev.yml --env-file .env up -d
```

If your CLI uses `docker-compose` (hyphen):

```bash
docker-compose -f docker-compose.dev.yml --env-file .env up -d
```

### Databases + Gopedia API (Phloem + HTTP)

```bash
docker compose -f docker-compose.dev.yml --env-file .env --profile app up -d --build
```

The first build can take several minutes (Python dependencies and optional PyTorch CPU wheel in the image).

### Colima note

If pulls failed with a desktop credential helper, export `DOCKER_HOST` and a minimal `DOCKER_CONFIG` as described in [install.md](install.md) for macOS Colima.

### Default published ports (host)

| Service | Ports |
|---------|--------|
| HTTP API | 18787 |
| Phloem gRPC | 50051 |
| PostgreSQL | 5432 |
| Qdrant HTTP / gRPC | 6333 / 6334 |
| Qdrant Web UI (nginx → Qdrant) | 6335 |
| TypeDB | 1729 / 8000 |

Override with `POSTGRES_PUBLISH_PORT`, `QDRANT_PUBLISH_HTTP`, `QDRANT_UI_PUBLISH_PORT`, `TYPEDB_PUBLISH_PORT`, `GOPEDIA_PUBLISH_HTTP`, etc. in `.env` if these conflict locally.

### Browser UIs (optional)

**Qdrant** — after the DB stack is up, start the proxy service and open the dashboard:

```bash
docker compose -f docker-compose.dev.yml --env-file .env up -d qdrant-ui
```

If your CLI uses `docker-compose` (hyphen), use that instead of `docker compose`. Then open **http://localhost:6335/dashboard** (or the host port you set with `QDRANT_UI_PUBLISH_PORT`).

**TypeDB Studio** — with TypeDB publishing **8000** to the host (default in [`docker-compose.dev.yml`](../../docker-compose.dev.yml)), open [TypeDB Studio](https://studio.typedb.com/data), set the server to **`localhost:8000`**, choose **TypeDB CE 3.8.2** (matching the image tag in compose), and connect.

---

## 3. Check health

```bash
curl -s http://127.0.0.1:18787/api/health
# Expect: {"status":"ok"}
```

Dependency checks (for agents / ops):

```bash
curl -s http://127.0.0.1:18787/api/health/deps
# Expect: {"status":"ok"|"degraded","deps":{"postgres":{...},"qdrant":{...},"typedb":{...},"phloem":{...}},...}
```

JSON search (machine-readable hits):

```bash
curl -s "http://127.0.0.1:18787/api/search?q=Introduction&format=json"
```

Compact JSON for agents (fewer fields; see [agent-interop.md](agent-interop.md) for presets and `fields=`):

```bash
curl -s "http://127.0.0.1:18787/api/search?q=Introduction&format=json&detail=summary"
```

Synchronous ingest (returns result immediately, 30-minute timeout):

```bash
curl -s -X POST http://127.0.0.1:18787/api/ingest \
  -H "Content-Type: application/json" \
  -d '{"path":"tests/fixtures/sample.md"}'
```

Async ingest job (returns job ID; poll for completion):

```bash
curl -s -X POST http://127.0.0.1:18787/api/ingest/jobs \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: my-key-1" \
  -d '{"path":"tests/fixtures/sample.md"}'
# Then: curl -s http://127.0.0.1:18787/api/jobs/<job_id>
```

Both ingest endpoints accept an optional `"project_id": <integer>` field in the request body to associate the ingest with a project.

If you did not start the `app` profile, these will fail until you run the API on the host (see below).

---

## 4. Initialize databases (first time)

With the **`app`** profile running, execute inside the API container (name may differ; adjust via `docker ps`):

```bash
docker exec gopedia-local-gopedia-1 python -c "
from tests.initialize import DBInitializer
print(DBInitializer().init_all(skip_missing=True))
"
```

Expected: `{'postgres': True, 'typedb': True, 'qdrant': True}`.

**Alternative — host Python** (repo root, same `.env`, hosts must be `127.0.0.1` if only DB containers are published):

```bash
python -c "
from dotenv import load_dotenv
load_dotenv('.env')
from tests.initialize import DBInitializer
print(DBInitializer().init_all(skip_missing=True))
"
```

Requires `pip install -r requirements.txt` and `python-dotenv` on the host.

---

## 5. `gopedia` CLI (ingest, search, restore)

Build the CLI (Go 1.24+ or `GOTOOLCHAIN=auto`):

```bash
GOTOOLCHAIN=auto go build -o gopedia ./cmd/gopedia
export GOPEDIA_API_URL=http://127.0.0.1:18787   # default; optional
```

**Ingest** a file or directory (paths are relative to the repo; the container bind-mounts `.` to `/app`):

```bash
./gopedia ingest tests/fixtures/sample.md
./gopedia ingest doc/design/Rev2/references
./gopedia ingest tests/fixtures/sample.md --json   # print full JSON response
```

**Search** (Xylem):

```bash
./gopedia search Introduction
./gopedia search Introduction --json                        # full JSON response
./gopedia search Introduction --json --detail=summary      # compact fields only
./gopedia search Introduction --json --detail=standard     # standard fields
./gopedia search Introduction --json --fields=title,snippet,l3_id  # explicit fields
```

`--detail` and `--fields` require `--json`. `--fields` overrides `--detail` when both are given.

**Restore** (PostgreSQL snapshot):

```bash
./gopedia restore --l1-id <l1_uuid>             # restore full content (markdown)
./gopedia restore --l2-id <l2_uuid>             # restore one L2 section (markdown)
./gopedia restore --l2-id <l2_uuid> --json      # full JSON response
```

Exactly one of `--l1-id` or `--l2-id` is required.

**Search detail presets** (`?detail=` / `--detail`):

| Preset | Fields returned |
|--------|----------------|
| `full` (default) | all fields including `surrounding_context` |
| `standard` | `doc_id`, `project_id`, `doc_name`, `l1_id`, `l2_id`, `l3_id`, `score`, `title`, `section_heading`, `snippet`, `source_path`, `breadcrumb` |
| `summary` | `doc_id`, `doc_name`, `l3_id`, `score`, `title`, `snippet`, `source_path` |

**Filter by project** (HTTP API):

```bash
curl -s "http://127.0.0.1:18787/api/search?q=Introduction&project_id=123&format=json"
```

**Restore via HTTP API:**

```bash
curl -s "http://127.0.0.1:18787/api/restore?l1_id=<l1_uuid>"
curl -s "http://127.0.0.1:18787/api/restore?l2_id=<l2_uuid>&format=json"
```

**Run local API server:**

```bash
./gopedia server                         # listens on 127.0.0.1:8787 by default
./gopedia server --addr 0.0.0.0:18787   # match compose port
./gopedia service start                  # alias for `gopedia server`
```

> The default host port for `gopedia server` is **8787**, not 18787. 18787 is the port published by Compose. Set `GOPEDIA_HTTP_ADDR` or `--addr` to override.

---

## 6. Run API on the host (optional)

If you started **only** databases with Compose, run the API locally:

```bash
export GOPEDIA_HTTP_ADDR=0.0.0.0:18787
export GOPEDIA_PHLOEM_GRPC_ADDR=0.0.0.0:50051
# Point all *_HOST variables in .env to 127.0.0.1 for Postgres, Qdrant, TypeDB
set -a; source .env; set +a
go run ./cmd/api
```

Ensure Python and `requirements.txt` are available on the host (`internal/runner`).

---

## 7. E2E shell scripts

Examples (after `export DOCKER_NETWORK_EXTERNAL=gopedia-dev` and with `.env` present):

```bash
# Full ingest + Xylem E2E (resets DBs, runs DBInitializer, ingests, verifies search)
./scripts/run_fresh_ingestion_docker.sh tests/fixtures/sample.md Introduction

# Project-only ingest (no DB reset, no DBInitializer)
./scripts/run_project_ingestion_docker.sh tests/fixtures/sample.md Introduction

# Xylem verification only (assumes data already in DBs)
./scripts/run_verify_xylem_docker.sh

# Reset all Rhizome stores (PostgreSQL, Qdrant, TypeDB)
python scripts/reset_rhizome_docker.py
```

> `scripts/run_ingestion_docker.sh` hardcodes the Docker network to `neunexus` and does not use `DOCKER_NETWORK_EXTERNAL`. Prefer `run_fresh_ingestion_docker.sh` for the `gopedia-dev` stack.

These scripts build additional images (`Dockerfile.ingestion`, Phloem image) and reset or verify stores; read each script header for behavior.

**Other utility scripts:**

| Script | Purpose |
|--------|---------|
| `verify_qdrant_payload.py` | Validate Qdrant point payloads post-ingest |
| `verify_typedb_graph.py` | Validate TypeDB graph structure |
| `verify_transpiration.py` | Verify Transpiration pipeline output |
| `verify_projects_after_ingest.py` | Check project associations after ingest |
| `print_postgres_counts.py` | Print row counts for all Postgres tables |
| `sync_doc_to_typedb.py` | Sync documents to TypeDB manually |

---

## 8. Stop and reset data

Stop containers:

```bash
docker compose -f docker-compose.dev.yml --env-file .env down
```

Remove named volumes (destructive):

```bash
docker compose -f docker-compose.dev.yml --env-file .env down -v
```

---

## 9. Troubleshooting

| Symptom | Check |
|---------|--------|
| `docker compose` not found | Install Compose v2 plugin or `docker-compose` standalone ([install.md](install.md)). |
| Pull / auth errors on macOS | `credsStore` / missing `docker-credential-desktop` ([install.md](install.md)). |
| API 502 / ingest errors | `OPENAI_API_KEY`, DB connectivity, and `DBInitializer` completed successfully. |
| Wrong network in scripts | `DOCKER_NETWORK_EXTERNAL=gopedia-dev` matches [`docker-compose.dev.yml`](../../docker-compose.dev.yml) `networks.gopedia-dev.name`. `run_ingestion_docker.sh` ignores this — use `run_fresh_ingestion_docker.sh` instead. |
| Port already allocated | Change `*_PUBLISH_*` variables in `.env`. |
| `gopedia server` on wrong port | Default is 8787, not 18787. Set `--addr` or `GOPEDIA_HTTP_ADDR`. |

For more context, see [overview.md](overview.md) and the Korean companion [../docker/local-dev-docker.md](../docker/local-dev-docker.md).
