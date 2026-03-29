# Overview: local development stack

Gopedia can run as a **Fuego HTTP API** (`cmd/api`) that embeds **Phloem** (gRPC ingestion) and drives **Python** for ingest and **Xylem** (semantic search). Data lives in **PostgreSQL**, **Qdrant**, and **TypeDB**.

This guide family focuses on the **local developer stack** defined in [`docker-compose.dev.yml`](../../docker-compose.dev.yml), which avoids external networks such as `neunexus` / `traefik-net` required by the root [`docker-compose.yml`](../../docker-compose.yml).

## Components

| Layer | Role |
|-------|------|
| **HTTP API** | Listens on port **18787** (configurable). Endpoints include `POST /api/ingest` and `GET /api/search`. |
| **Phloem** | gRPC server inside the same API process (default **50051** in dev compose). Markdown ingest flows through Python `property.root_props.run` → Phloem → stores. |
| **Xylem** | Invoked as a Python subprocess for `GET /api/search` (`flows.xylem_flow.cli`). |
| **PostgreSQL** | Relational metadata and document tables (`postgres_db` service). |
| **Qdrant** | Vector store for chunks and document embeddings (`qdrant` service). |
| **TypeDB** | Graph / document-section model (`typedb` service). |

## Two Compose files

| File | Use case |
|------|----------|
| [`docker-compose.dev.yml`](../../docker-compose.dev.yml) | **Local dev**: starts Postgres, TypeDB, Qdrant on bridge network **`gopedia-dev`**. Optional **`app`** profile adds the Gopedia API container. |
| [`docker-compose.yml`](../../docker-compose.yml) | **Integrated / server** layout: expects pre-created external networks and separate DB hosts; mounts `/morphogen` when used as documented elsewhere. |

## CLI vs container

The [`gopedia`](../../cmd/gopedia/main.go) CLI is a thin **HTTP client** to the API (`GOPEDIA_API_URL`, default `http://127.0.0.1:18787`). It does not bundle the server.

The [`Dockerfile`](../../Dockerfile) builds and runs the **`api`** binary only. For CLI workflows, build the CLI on your machine (see [install.md](install.md)) or add your own tooling.

## Environment and scripts

- Copy [`.env.local.example`](../../.env.local.example) to **`.env`** at the repo root for the dev stack. See also [`.env.example`](../../.env.example) for all variables used by production compose and shell scripts.
- Shell scripts under [`scripts/`](../../scripts/) (for example `run_fresh_ingestion_docker.sh`) attach one-off containers to a Docker network. For the dev stack, set:

  ```bash
  export DOCKER_NETWORK_EXTERNAL=gopedia-dev
  ```

- Schema and collections are created with Python [`DBInitializer`](../../tests/initialize.py) (`init_all`).

## Further reading

- [run.md](run.md) — step-by-step bring-up and smoke tests.
- [install.md](install.md) — OS-specific Docker and toolchain setup.
- [../docker/local-dev-docker.md](../docker/local-dev-docker.md) — Korean notes on the same dev compose file (duplicate topics in English here).
