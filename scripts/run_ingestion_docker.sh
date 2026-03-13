#!/usr/bin/env bash
# Run ingestion + Transpiration E2E inside Docker on neunexus so containers can reach
# phloem-flow, typedb, qdrant, postgres_db by internal hostname.
# Failure points: doc/ingestion-docker-failures.md
set -e
cd "$(dirname "$0")/.."

[[ -f .env ]] || { echo "No .env" >&2; exit 1; }

IMAGE="${GOPEDIA_INGESTION_IMAGE:-gopedia-ingestion:test}"
SAMPLE_MD="${1:-doc/sample.md}"
KEYWORD="${2:-서론}"

echo "=== Build ingestion image (Python + protobuf>=4.25) ==="
docker build -f Dockerfile.ingestion -t "$IMAGE" .

echo "=== Run ingestion + E2E on network neunexus (sample=$SAMPLE_MD, keyword=$KEYWORD) ==="
docker run --rm \
  --network neunexus \
  --env-file .env \
  -e POSTGRES_HOST="${POSTGRES_HOST:-postgres_db}" \
  -e TYPEDB_HOST="${TYPEDB_HOST:-typedb}" \
  -e QDRANT_HOST="${QDRANT_HOST:-qdrant}" \
  -e GOPEDIA_PHLOEM_GRPC_ADDR="${GOPEDIA_PHLOEM_GRPC_ADDR:-phloem-flow:50051}" \
  -v "$PWD:/app" \
  -w /app \
  "$IMAGE" \
  ./scripts/run_transpiration_e2e.sh "$SAMPLE_MD" "$KEYWORD"
