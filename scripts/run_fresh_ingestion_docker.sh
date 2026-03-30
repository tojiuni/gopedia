#!/usr/bin/env bash
# Fresh Docker E2E (PostgreSQL documents.id = UUID; documents.current_l1_id = 리비전 헤드;
# L1–L3 UUID PKs; Qdrant payload l1_id = knowledge_l1.id, doc_id = documents.id):
#  1) stop any existing phloem-e2e container
#  2) reset/drop existing Postgres/Qdrant/TypeDB data
#  3) start a fresh phloem container built from current code
#  4) run DBInitializer.init_all() to recreate schemas/collections
#  5) ingest sample + run transpiration verification
#  6) direct Qdrant/TypeDB/Postgres row-count verification
#  7) Xylem flow: markdown restore + Qdrant/PG enriched context (verify_xylem_flow.py)
# QDRANT_COLLECTION: use .env (e.g. gopedia); fallback below matches doc/.env template.

set -e
cd "$(dirname "$0")/.."

[[ -f .env ]] || { echo "No .env" >&2; exit 1; }
set -a
# shellcheck source=/dev/null
source ./.env
set +a

# Docker network where postgres_db, qdrant, typedb are reachable (see .env DOCKER_NETWORK_EXTERNAL).
DOCKER_NETWORK="${DOCKER_NETWORK_EXTERNAL:-neunexus}"

# Python E2E image from Dockerfile.ingestion (reset, DBInitializer, verify_*). Phloem uses GOPEDIA_PHLOEM_E2E_IMAGE.
# This image must exist before any "docker run ... $IMAGE" — build via the "Build ingestion image" step above, or:
#   docker build --pull -f Dockerfile.ingestion -t gopedia-ingestion:test .
# Override registry/tag: export GOPEDIA_INGESTION_IMAGE=myregistry/gopedia-ingestion:1.2.3
IMAGE="${GOPEDIA_INGESTION_IMAGE:-gopedia-ingestion:test}"
PHLOEM_E2E_IMAGE="${GOPEDIA_PHLOEM_E2E_IMAGE:-gopedia-phloem:e2e}"
PHLOEM_E2E_NAME="${GOPEDIA_PHLOEM_E2E_NAME:-phloem-e2e}"

SAMPLE_MD="${1:-tests/fixtures/sample.md}"
KEYWORD="${2:-Introduction}"

# Repo is mounted as /app; samples outside PWD are invisible unless we bind-mount their parent dir.
EXTRA_DOCKER_MOUNTS=()
SAMPLE_FOR_DOCKER="$SAMPLE_MD"
if [[ -f "$SAMPLE_MD" ]]; then
  _abs_sample=$(realpath "$SAMPLE_MD")
  _abs_repo=$(realpath "$PWD")
  case "$_abs_sample" in
  "$_abs_repo"/*) ;;
  *)
    _sdir=$(dirname "$_abs_sample")
    _sbase=$(basename "$_abs_sample")
    EXTRA_DOCKER_MOUNTS=( -v "$_sdir:/_gopedia_e2e_sample:ro" )
    SAMPLE_FOR_DOCKER="/_gopedia_e2e_sample/$_sbase"
    ;;
  esac
fi

cleanup_phloem() {
  docker stop "$PHLOEM_E2E_NAME" 2>/dev/null || true
  docker rm "$PHLOEM_E2E_NAME" 2>/dev/null || true
}
trap cleanup_phloem EXIT

echo "=== [fresh] Build ingestion image (current code, refresh base via --pull) ==="
docker build --pull -f Dockerfile.ingestion -t "$IMAGE" .

echo "=== [fresh] Reset stores (postgres/qdrant/typedb) ==="
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --env-file .env \
  -e POSTGRES_HOST="${POSTGRES_HOST:-postgres_db}" \
  -e POSTGRES_PORT="${POSTGRES_PORT:-5432}" \
  -e POSTGRES_USER="${POSTGRES_USER:-admin_gopedia}" \
  -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  -e POSTGRES_DB="${POSTGRES_DB:-gopedia}" \
  -e TYPEDB_HOST="${TYPEDB_HOST:-typedb}" \
  -e TYPEDB_PORT="${TYPEDB_PORT:-1729}" \
  -e TYPEDB_DATABASE="${TYPEDB_DATABASE:-gopedia}" \
  -e QDRANT_HOST="${QDRANT_HOST:-qdrant}" \
  -e QDRANT_PORT="${QDRANT_PORT:-6333}" \
  -e QDRANT_COLLECTION="${QDRANT_COLLECTION:-gopedia}" \
  -e QDRANT_DOC_COLLECTION="${QDRANT_DOC_COLLECTION:-gopedia_document}" \
  -v "$PWD:/app" \
  -w /app \
  "$IMAGE" \
  python scripts/reset_rhizome_docker.py

echo "=== [fresh] Build Phloem from current code (--pull) ==="
docker build --pull -t "$PHLOEM_E2E_IMAGE" .

echo "=== [fresh] Start Phloem container ==="
cleanup_phloem
docker run -d \
  --name "$PHLOEM_E2E_NAME" \
  --network "$DOCKER_NETWORK" \
  --env-file .env \
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

echo "=== [fresh] Initialize DBs/collections/schemas ==="
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --env-file .env \
  -e POSTGRES_HOST="${POSTGRES_HOST:-postgres_db}" \
  -e TYPEDB_HOST="${TYPEDB_HOST:-typedb}" \
  -e QDRANT_HOST="${QDRANT_HOST:-qdrant}" \
  -e GOPEDIA_PHLOEM_GRPC_ADDR="${GOPEDIA_PHLOEM_GRPC_ADDR:-$PHLOEM_E2E_NAME:50051}" \
  -v "$PWD:/app" \
  -w /app \
  "$IMAGE" \
  python -c "from tests.initialize import DBInitializer; print(DBInitializer().init_all(skip_missing=True))"

echo "=== [fresh] Run ingestion + transpiration ==="
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --env-file .env \
  -e POSTGRES_HOST="${POSTGRES_HOST:-postgres_db}" \
  -e TYPEDB_HOST="${TYPEDB_HOST:-typedb}" \
  -e QDRANT_HOST="${QDRANT_HOST:-qdrant}" \
  -e GOPEDIA_PHLOEM_GRPC_ADDR="${GOPEDIA_PHLOEM_GRPC_ADDR:-$PHLOEM_E2E_NAME:50051}" \
  -v "$PWD:/app" \
  "${EXTRA_DOCKER_MOUNTS[@]}" \
  -w /app \
  "$IMAGE" \
  ./scripts/run_transpiration_e2e.sh "$SAMPLE_FOR_DOCKER" "$KEYWORD"

echo "=== [fresh] Verify Qdrant payload ==="
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --env-file .env \
  -e QDRANT_HOST="${QDRANT_HOST:-qdrant}" \
  -e QDRANT_PORT="${QDRANT_PORT:-6333}" \
  -e QDRANT_COLLECTION="${QDRANT_COLLECTION:-gopedia}" \
  -e QDRANT_DOC_COLLECTION="${QDRANT_DOC_COLLECTION:-gopedia_document}" \
  -v "$PWD:/app" \
  -w /app \
  "$IMAGE" \
  python scripts/verify_qdrant_payload.py

echo "=== [fresh] Verify TypeDB graph connectivity ==="
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --env-file .env \
  -e TYPEDB_HOST="${TYPEDB_HOST:-typedb}" \
  -e TYPEDB_PORT="${TYPEDB_PORT:-1729}" \
  -e TYPEDB_DATABASE="${TYPEDB_DATABASE:-gopedia}" \
  -v "$PWD:/app" \
  -w /app \
  "$IMAGE" \
  python scripts/verify_typedb_graph.py

echo "=== [fresh] Verify Postgres row counts ==="
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --env-file .env \
  -e POSTGRES_HOST="${POSTGRES_HOST:-postgres_db}" \
  -e POSTGRES_PORT="${POSTGRES_PORT:-5432}" \
  -e POSTGRES_USER="${POSTGRES_USER:-admin_gopedia}" \
  -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  -e POSTGRES_DB="${POSTGRES_DB:-gopedia}" \
  -v "$PWD:/app" \
  -w /app \
  "$IMAGE" \
  python scripts/print_postgres_counts.py

echo "=== [fresh] Verify projects table + documents.project_id FK ==="
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --env-file .env \
  -e POSTGRES_HOST="${POSTGRES_HOST:-postgres_db}" \
  -e POSTGRES_PORT="${POSTGRES_PORT:-5432}" \
  -e POSTGRES_USER="${POSTGRES_USER:-admin_gopedia}" \
  -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  -e POSTGRES_DB="${POSTGRES_DB:-gopedia}" \
  -v "$PWD:/app" \
  -w /app \
  "$IMAGE" \
  python scripts/verify_projects_after_ingest.py

echo "=== [fresh] Verify Xylem Flow (markdown restore + Qdrant/PG RAG context) ==="
# 이 단계만 로컬에서 돌리려면: ./scripts/run_verify_xylem_docker.sh [keyword] (이미지·네트워크·.env 필요)
# L3 컬렉션은 DBInitializer가 만든 단일 무명 벡터를 쓴다. .env의 QDRANT_VECTOR_NAME(dense-gopedia 등)은
# 문서용 컬렉션(QDRANT_DOC_*)용이므로 여기서는 비워 query_points에 using을 넣지 않는다.
docker run --rm \
  --network "$DOCKER_NETWORK" \
  --env-file .env \
  -e POSTGRES_HOST="${POSTGRES_HOST:-postgres_db}" \
  -e POSTGRES_PORT="${POSTGRES_PORT:-5432}" \
  -e POSTGRES_USER="${POSTGRES_USER:-admin_gopedia}" \
  -e POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
  -e POSTGRES_DB="${POSTGRES_DB:-gopedia}" \
  -e QDRANT_HOST="${QDRANT_HOST:-qdrant}" \
  -e QDRANT_PORT="${QDRANT_PORT:-6333}" \
  -e QDRANT_COLLECTION="${QDRANT_COLLECTION:-gopedia}" \
  -e QDRANT_VECTOR_NAME= \
  -e OPENAI_API_KEY="${OPENAI_API_KEY}" \
  -e OPENAI_EMBEDDING_MODEL="${OPENAI_EMBEDDING_MODEL:-text-embedding-3-small}" \
  -v "$PWD:/app" \
  -w /app \
  "$IMAGE" \
  python scripts/verify_xylem_flow.py "$KEYWORD"

echo "=== [fresh] Done ==="

