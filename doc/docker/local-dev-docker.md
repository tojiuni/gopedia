# 로컬 개발용 Docker 스택 (`docker-compose.dev.yml`)

외부 `neunexus` / `traefik-net` 없이 **PostgreSQL, TypeDB, Qdrant**를 한 브리지 네트워크(`gopedia-dev`)에서 띄우는 구성입니다. 서비스 DNS 이름은 기존 스크립트 기본값과 동일합니다(`postgres_db`, `typedb`, `qdrant`).

## 사전 준비

1. **환경 파일**: [`.env.local.example`](../.env.local.example)를 복사해 프로젝트 루트의 `.env`로 쓰거나, 동일 내용을 기존 `.env`에 합칩니다.
2. **`OPENAI_API_KEY`**: 인게스트·임베딩·검증 스크립트에 필요합니다.
3. **`POSTGRES_PASSWORD`**: `docker-compose.dev.yml`에서 필수입니다(`:?` 보간).

## 기동

**DB만** (기본):

```bash
docker compose -f docker-compose.dev.yml --env-file .env up -d
```

**API + Phloem**까지 (프로필 `app`):

```bash
docker compose -f docker-compose.dev.yml --env-file .env --profile app up -d --build
```

- HTTP: `18787`, Phloem gRPC(호스트에서 접근 시): `50051`
- Postgres: `5432`, Qdrant: `6333`/`6334`, TypeDB: `1729`/`8000` (호스트 포트는 [docker-compose.dev.yml](../docker-compose.dev.yml)의 `*_PUBLISH_*` 변수로 바꿀 수 있음)

## E2E / `docker run` 스크립트용 네트워크

이 compose 파일은 Docker 네트워크 이름을 **`gopedia-dev`**로 고정합니다. [scripts/run_fresh_ingestion_docker.sh](../scripts/run_fresh_ingestion_docker.sh) 등은 다음을 셸에 설정한 뒤 실행합니다.

```bash
export DOCKER_NETWORK_EXTERNAL=gopedia-dev
```

(기본값 `neunexus` 대신 위 값을 씁니다.)

## DB 초기화

스토어가 뜬 뒤 **한 번** 스키마·컬렉션을 맞춥니다.

**호스트에서 Python** (레포 루트, `.env` 로드):

```bash
python -c "
from dotenv import load_dotenv
load_dotenv('.env')
from tests.initialize import DBInitializer
print(DBInitializer().init_all(skip_missing=True))
"
```

또는 [scripts/run_ingestion_docker.sh](../scripts/run_ingestion_docker.sh) / [scripts/run_fresh_ingestion_docker.sh](../scripts/run_fresh_ingestion_docker.sh)는 내부에서 `DBInitializer`를 호출합니다. 위 `DOCKER_NETWORK_EXTERNAL`만 맞추면 됩니다.

## TypeDB 이미지

기본 이미지는 `typedb/typedb:3.8.2`입니다(`typedb-driver` 3.x와 맞춤). 다른 태그를 쓰려면 `.env`에 `TYPEDB_IMAGE=...`를 설정합니다. 데이터 볼륨은 TypeDB 3.7.1+ 기준 경로 `/var/lib/typedb/data`를 사용합니다([TypeDB CE Docker 문서](https://typedb.com/docs/home/install/ce)).

## `QDRANT_COLLECTION` 정리

`docker-compose.yml` 기본은 `gopedia_markdown`, 일부 스크립트 기본은 `gopedia`입니다. 로컬에서는 `.env`에서 **하나로 통일**하는 것을 권장합니다([.env.local.example](../.env.local.example)는 `gopedia_markdown` 예시).

## 데이터 초기화 (볼륨 삭제)

```bash
docker compose -f docker-compose.dev.yml --env-file .env down -v
```

## 호스트에서만 API 실행하는 경우

DB 컨테이너만 띄운 뒤, 호스트의 `go`/`python`이 **포트 포워딩**으로 붙이려면 `.env`에서 `POSTGRES_HOST`, `TYPEDB_HOST`, `QDRANT_HOST`를 `127.0.0.1`로 바꿉니다. 이 경우 `DOCKER_NETWORK_EXTERNAL`을 쓰는 Docker 기반 E2E와는 호스트 설정이 다르므로, [.env.local.example](../.env.local.example) 하단 주석을 참고합니다.

## 배포용 compose와의 차이

| 항목 | [docker-compose.yml](../docker-compose.yml) | [docker-compose.dev.yml](../docker-compose.dev.yml) |
|------|-----------------------------------------------|-----------------------------------------------------|
| 네트워크 | `neunexus`, `traefik-net` (external) | 내부 `gopedia-dev` |
| DB | 외부 가정 | `postgres_db`, `typedb`, `qdrant` 포함 |
| morphogen | `/morphogen` 마운트 | 없음 (필요 시 로컬 경로로 별도 마운트) |

더 넓은 테스트·E2E 설명은 [test-initialize-and-run.md](test-initialize-and-run.md)를 참고합니다.
