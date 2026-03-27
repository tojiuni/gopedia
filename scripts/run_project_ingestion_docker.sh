#!/usr/bin/env bash
# Project ingestion only (Docker): Root → Phloem gRPC. No Postgres/Qdrant/TypeDB reset, no DBInitializer.
#
# Prerequisites:
#   - .env (or GOPEDIA_ENV_FILE) with POSTGRES_*, TYPEDB_HOST, QDRANT_*, OPENAI_* as needed for ingest + TypeDB sync
#   - Docker network DOCKER_NETWORK_EXTERNAL (default neunexus) where Phloem and data stores are reachable
#   - Phloem gRPC listening (default GOPEDIA_PHLOEM_GRPC_ADDR=phloem-e2e:50051), OR pass --start-phloem to build+run a local Phloem container
#   - Ingestion image: gopedia-ingestion:test (build once: docker build -f Dockerfile.ingestion -t gopedia-ingestion:test .)
#     or export GOPEDIA_INGESTION_IMAGE=...
#
# Usage:
#   ./scripts/run_project_ingestion_docker.sh [options] <host-path-to-project-dir-or-file>
#
# Example (wiki project on host):
#   ./scripts/run_project_ingestion_docker.sh /morphogen/neunexus/project_skills/wiki/universitas/gopedia
#
# Options:
#   --build-ingestion-image   docker build Dockerfile.ingestion before run
#   --start-phloem            docker build Phloem image and run detached container GOPEDIA_PHLOEM_E2E_NAME (default phloem-e2e)
#   -h, --help                show this help

set -e
cd "$(dirname "$0")/.."
REPO_ROOT="$PWD"

usage() {
  echo "Usage: $(basename "$0") [options] <project-path>"
  echo "  Docker-only project ingest (Root → Phloem). No DB reset / no DBInitializer."
  echo "  Example: $(basename "$0") /morphogen/neunexus/project_skills/wiki/universitas/gopedia"
  echo "Options:"
  echo "  --build-ingestion-image   Build Dockerfile.ingestion image first"
  echo "  --start-phloem            Build and run Phloem container on the stack network"
  echo "  -h, --help                Show this help"
}

BUILD_INGESTION=0
START_PHLOEM=0
PROJECT_PATH=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build-ingestion-image) BUILD_INGESTION=1; shift ;;
    --start-phloem) START_PHLOEM=1; shift ;;
    -h|--help) usage; exit 0 ;;
    -*)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
    *)
      if [[ -n "$PROJECT_PATH" ]]; then
        echo "Extra argument: $1 (only one project path allowed)" >&2
        exit 1
      fi
      PROJECT_PATH="$1"
      shift
      ;;
  esac
done

if [[ -z "$PROJECT_PATH" ]]; then
  echo "Error: missing project path." >&2
  usage >&2
  exit 1
fi

ENV_FILE="${GOPEDIA_ENV_FILE:-.env}"
[[ "$ENV_FILE" == /* ]] || ENV_FILE="$REPO_ROOT/$ENV_FILE"
[[ -f "$ENV_FILE" ]] || {
  echo "No env file: $ENV_FILE" >&2
  echo "  Set GOPEDIA_ENV_FILE or create .env in $REPO_ROOT" >&2
  exit 1
}
set -a
# shellcheck source=/dev/null
source "$ENV_FILE"
set +a

DOCKER_NETWORK="${DOCKER_NETWORK_EXTERNAL:-neunexus}"
if ! docker network inspect "$DOCKER_NETWORK" >/dev/null 2>&1; then
  echo "ERROR: Docker network '$DOCKER_NETWORK' does not exist." >&2
  exit 1
fi

IMAGE="${GOPEDIA_INGESTION_IMAGE:-gopedia-ingestion:test}"
PHLOEM_E2E_IMAGE="${GOPEDIA_PHLOEM_E2E_IMAGE:-gopedia-phloem:e2e}"
PHLOEM_E2E_NAME="${GOPEDIA_PHLOEM_E2E_NAME:-phloem-e2e}"

EXTRA_VOLUME_ARGS=()
CONTAINER_PATH="$PROJECT_PATH"
_repo_abs=$(realpath "$REPO_ROOT" 2>/dev/null || echo "$REPO_ROOT")
if [[ "$PROJECT_PATH" == /* ]]; then
  _hp=$(realpath -m "$PROJECT_PATH")
else
  _hp=$(realpath -m "$REPO_ROOT/$PROJECT_PATH")
fi
if [[ ! -e "$_hp" ]]; then
  echo "Path does not exist: $_hp" >&2
  exit 1
fi
if [[ -e "$_hp" ]]; then
  case "$_hp" in
    "$_repo_abs" | "$_repo_abs"/*)
      CONTAINER_PATH="/app${_hp#$_repo_abs}"
      ;;
    *)
      EXTRA_VOLUME_ARGS=( -v "$_hp:/mnt/gopedia_project_ingest:ro" )
      CONTAINER_PATH="/mnt/gopedia_project_ingest"
      echo "=== [ingest] Mount project: $_hp -> /mnt/gopedia_project_ingest ==="
      ;;
  esac
fi

if [[ "$BUILD_INGESTION" -eq 1 ]]; then
  echo "=== [ingest] Build ingestion image ==="
  docker build --pull -f Dockerfile.ingestion -t "$IMAGE" .
fi

if [[ "$START_PHLOEM" -eq 1 ]]; then
  echo "=== [ingest] Build and start Phloem container ($PHLOEM_E2E_NAME) ==="
  docker build --pull -t "$PHLOEM_E2E_IMAGE" .
  docker stop "$PHLOEM_E2E_NAME" 2>/dev/null || true
  docker rm "$PHLOEM_E2E_NAME" 2>/dev/null || true
  docker run -d \
    --name "$PHLOEM_E2E_NAME" \
    --network "$DOCKER_NETWORK" \
    --env-file "$ENV_FILE" \
    -e POSTGRES_HOST="${POSTGRES_HOST:-postgres_db}" \
    -e POSTGRES_PORT="${POSTGRES_PORT:-5432}" \
    -e POSTGRES_USER="${POSTGRES_USER:-admin_gopedia}" \
    -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
    -e POSTGRES_DB="${POSTGRES_DB:-gopedia}" \
    -e POSTGRES_SSLMODE="${POSTGRES_SSLMODE:-disable}" \
    -e QDRANT_HOST="${QDRANT_HOST:-qdrant}" \
    -e QDRANT_GRPC_PORT="${QDRANT_GRPC_PORT:-6334}" \
    -e QDRANT_COLLECTION="${QDRANT_COLLECTION:-gopedia}" \
    -e OPENAI_API_KEY="${OPENAI_API_KEY}" \
    -e OPENAI_EMBEDDING_MODEL="${OPENAI_EMBEDDING_MODEL:-text-embedding-3-small}" \
    "$PHLOEM_E2E_IMAGE"
  sleep 2
fi

PHLOEM_ADDR="${GOPEDIA_PHLOEM_GRPC_ADDR:-$PHLOEM_E2E_NAME:50051}"

echo "=== [ingest] Run project ingestion (container path: $CONTAINER_PATH, Phloem: $PHLOEM_ADDR) ==="
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --env-file "$ENV_FILE" \
  -e POSTGRES_HOST="${POSTGRES_HOST:-postgres_db}" \
  -e POSTGRES_PORT="${POSTGRES_PORT:-5432}" \
  -e POSTGRES_USER="${POSTGRES_USER:-admin_gopedia}" \
  -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  -e POSTGRES_DB="${POSTGRES_DB:-gopedia}" \
  -e POSTGRES_SSLMODE="${POSTGRES_SSLMODE:-disable}" \
  -e TYPEDB_HOST="${TYPEDB_HOST:-typedb}" \
  -e TYPEDB_PORT="${TYPEDB_PORT:-1729}" \
  -e TYPEDB_DATABASE="${TYPEDB_DATABASE:-gopedia}" \
  -e QDRANT_HOST="${QDRANT_HOST:-qdrant}" \
  -e GOPEDIA_PHLOEM_GRPC_ADDR="$PHLOEM_ADDR" \
  -v "$REPO_ROOT:/app" \
  "${EXTRA_VOLUME_ARGS[@]}" \
  -w /app \
  "$IMAGE" \
  python -m property.root_props.run "$CONTAINER_PATH"

echo "=== [ingest] Done ==="
