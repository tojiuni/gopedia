#!/usr/bin/env bash
# Xylem만 검증: run_fresh_ingestion_docker.sh 의 마지막 단계와 동일(기본 모드: 전체).
# 이미 ingest·DB·Qdrant에 데이터가 있을 때 사용한다.
#
# Usage (저장소 루트에서):
#   ./scripts/run_verify_xylem_docker.sh
#       → 전체 검증, 검색어 기본 Introduction (또는 환경변수 KEYWORD)
#   ./scripts/run_verify_xylem_docker.sh Introduction
#       → 전체 검증, 검색어 Introduction
#   ./scripts/run_verify_xylem_docker.sh restore
#       → PostgreSQL만: 최신 L1 전체 마크다운 복원 (OpenAI/Qdrant 없음)
#   ./scripts/run_verify_xylem_docker.sh keyword [query]
#       → 시맨틱 검색 + 리치 컨텍스트만 (기본 query: Introduction)
#   ./scripts/run_verify_xylem_docker.sh all [query]
#       → 위와 동일하게 전체 검증 (명시적)
#
# 선행 조건:
#   - .env 존재 (POSTGRES_*, QDRANT_*, keyword 모드는 OPENAI_API_KEY)
#   - Docker 네트워크: DOCKER_NETWORK_EXTERNAL (기본 neunexus)
#   - 이미지: GOPEDIA_INGESTION_IMAGE (기본 gopedia-ingestion:test)
#     빌드: docker build --pull -f Dockerfile.ingestion -t gopedia-ingestion:test .
#
set -e
cd "$(dirname "$0")/.."

[[ -f .env ]] || { echo "No .env in repo root" >&2; exit 1; }
set -a
# shellcheck source=/dev/null
source ./.env
set +a

DOCKER_NETWORK="${DOCKER_NETWORK_EXTERNAL:-neunexus}"
IMAGE="${GOPEDIA_INGESTION_IMAGE:-gopedia-ingestion:test}"

case "${1:-}" in
  restore)
    PY_ARGS=(--restore-only)
    MODE_LABEL="restore-only"
    ;;
  keyword)
    shift
    PY_ARGS=(--keyword-only "${1:-Introduction}")
    MODE_LABEL="keyword-only"
    ;;
  all)
    shift
    PY_ARGS=("${1:-Introduction}")
    MODE_LABEL="all"
    ;;
  *)
    # 첫 인자가 검색어(전체 파이프라인). 인자 없으면 KEYWORD 또는 Introduction.
    PY_ARGS=("${1:-${KEYWORD:-Introduction}}")
    MODE_LABEL="all"
    ;;
esac

if ! docker image inspect "$IMAGE" &>/dev/null; then
  echo "Docker image not found: $IMAGE" >&2
  echo "Build: docker build --pull -f Dockerfile.ingestion -t $IMAGE ." >&2
  exit 1
fi

if ! docker network inspect "$DOCKER_NETWORK" &>/dev/null; then
  echo "Docker network not found: $DOCKER_NETWORK (set DOCKER_NETWORK_EXTERNAL in .env or create the network)" >&2
  exit 1
fi

echo "=== [xylem-only] mode=$MODE_LABEL image=$IMAGE network=$DOCKER_NETWORK ==="

# L3 컬렉션: 무명 벡터 — .env의 QDRANT_VECTOR_NAME은 문서 컬렉션용이므로 여기서 비움.
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
  python scripts/verify_xylem_flow.py "${PY_ARGS[@]}"
