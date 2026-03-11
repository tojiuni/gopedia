#!/usr/bin/env bash
# Run integration tests inside Docker on neunexus (and optionally traefik-net).
# Connects to phloem-flow, typedb, qdrant, postgres by internal hostname.
set -e
cd "$(dirname "$0")/.."

[[ -f .env ]] || { echo "No .env" >&2; exit 1; }

GO_IMAGE="${GO_IMAGE:-golang:1.24-alpine}"

run_on_network() {
  local net=$1
  echo "=== Integration tests on Docker network: $net ==="
  docker run --rm \
    --network "$net" \
    --env-file .env \
    -e DOCKER_NETWORK_EXTERNAL="$net" \
    -e POSTGRES_HOST=postgres_db \
    -e TYPEDB_HOST=typedb \
    -e QDRANT_HOST=qdrant \
    -e GOPEDIA_PHLOEM_GRPC_ADDR=phloem-flow:50051 \
    -v "$PWD:/app" -w /app \
    "$GO_IMAGE" \
    sh -c "go mod download && go test ./tests/integration/ -v -count=1 -run 'DockerNetwork|ConnectivityOver' -timeout 90s"
}

for net in neunexus traefik-net; do
  if docker network inspect "$net" &>/dev/null; then
    run_on_network "$net"
  else
    echo "Skipping network $net (not found)" >&2
  fi
done

echo "=== Docker network integration tests done ==="
