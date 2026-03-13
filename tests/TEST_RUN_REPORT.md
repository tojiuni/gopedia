# 연동 테스트 실행 보고 — 로컬 / Docker

로컬 테스트 → Docker 이미지 푸시 및 compose 테스트 순으로 실행한 결과와 원인 정리.

**최신 (Postgres DDL 적용 후)**: neunexus 네트워크 연동 테스트 **전체 PASS**. traefik-net은 동일 Postgres 접근 불가 시 documents 미존재로 FAIL 가능.

---

## 1. 로컬 테스트 결과 (실패)

### 1.1 Go (`go test ./tests/integration/ -v`)

| 테스트 | 결과 | 원인 |
|--------|------|------|
| TestDockerNetworkExternalValid | PASS | - |
| TestConnectivityOverNeunexus | **FAIL** | 호스트에서 실행 시 `.env`의 내부 호스트명(`typedb`, `qdrant`, `postgres_db`, `phloem-flow`)이 DNS로 해석되지 않음. 해당 이름은 Docker 네트워크(neunexus/traefik-net) 내부에서만 유효. |
| TestInitAllFromEnv | **FAIL** | `open core/ontology_so/postgres_ddl.sql: no such file or directory` — `GOPEDIA_REPO_ROOT` 또는 테스트 실행 CWD가 레포 루트가 아니어서 DDL 경로를 찾지 못함. |
| TestTypeDBReachable | **FAIL** | `typedb:1729` 호스트명 미해석 (로컬 호스트에는 typedb 서버 없음). |
| TestQdrantConnect | **FAIL** | `qdrant` 호스트명 미해석 → `name resolver error: produced zero addresses`. |
| TestPostgresConnect | **FAIL** | `postgres_db` 호스트명 미해석 → `lookup postgres_db on ... no such host`. |
| TestPhloemGRPCReachable | **FAIL** | 로컬에서 Phloem 프로세스 미기동 → `localhost:50051` 연결 불가. |
| TestPhloemIngestMarkdown | **FAIL** | 동일. |
| TestQdrantEnsureCollection | **FAIL** | Qdrant 호스트명 미해석. |

**로컬 Go 실패 요약**: `.env`가 Docker 내부용(typedb, qdrant, postgres_db, phloem-flow)으로 설정되어 있어, **호스트에서는 해당 호스트명이 없고** Phloem도 호스트에서 띄우지 않아 전부 실패.

---

### 1.2 Python (`pytest tests/ -v -m integration`)

| 테스트 | 결과 | 원인 |
|--------|------|------|
| test_db_initializer_init_postgres | **FAIL** | `init_postgres()` 반환 `False` — 호스트에서 `postgres_db` 접속 불가. |
| test_phloem_grpc_reachable | **FAIL** | Phloem gRPC 서버 미기동 → `localhost:50051` 연결 불가. |
| test_phloem_ingest_markdown | **FAIL** | 동일. |
| 기타 (typedb, qdrant 등) | SKIP | 해당 env 미설정 또는 의존성 미설치로 스킵. |

**로컬 Python 실패 요약**: Postgres/Phloem이 호스트에서 접근 불가한 환경 설정이라 실패.

---

## 2. Docker 이미지 푸시 및 compose 테스트 결과

### 2.1 수행한 작업

- `docker build -t registry.toji.homes/gopedia-phloem:0.0.1 .` → **성공**
- `docker push registry.toji.homes/gopedia-phloem:0.0.1` → **성공**
- `docker compose --env-file .env up -d` → **성공** (phloem-flow 컨테이너 기동)
- `./scripts/run_integration_tests_docker_networks.sh` (neunexus 네트워크에서 연동 테스트) → **일부 실패**

### 2.2 Docker 네트워크(neunexus) 테스트 결과

| 항목 | 결과 | 비고 |
|------|------|------|
| TestDockerNetworkExternalValid | PASS | - |
| TypeDB (typedb:1729) | **연결 성공** | - |
| Qdrant | **연결 성공** | - |
| Phloem (phloem-flow:50051) | **연결 성공** | compose로 기동된 컨테이너 도달 |
| Postgres | **연결 성공** | - |
| **documents 테이블** | **FAIL** | `documents table not found (run core/ontology_so/postgres_ddl.sql)` |
| TestConnectivityOverTraefikNet | SKIP | DOCKER_NETWORK_EXTERNAL=neunexus 로만 실행되어 스킵 |

**Docker 실패 요약**: 네트워크·서비스 연결은 모두 성공. **Postgres에 `documents` 테이블이 없어서** 해당 검사만 실패.

---

## 3. 원인 정리

| 구분 | 원인 | 조치 |
|------|------|------|
| **로컬 테스트 전반** | `.env`가 Docker 내부 호스트명(typedb, qdrant, postgres_db, phloem-flow) 기준이라, **호스트에서는 이름 해석·접근 불가**. | 로컬에서 돌릴 때는 `localhost` 또는 실제 접근 가능한 호스트로 별도 env 사용하거나, **Docker 네트워크 테스트만 사용** (아래 스크립트). |
| **로컬 Phloem** | 호스트에서 `go run ./cmd/phloem` 또는 포트 포워딩된 프로세스가 없음. | 로컬에서 Phloem 기동하거나, Docker compose로 띄운 뒤 **Docker 네트워크 테스트**로 검증. |
| **TestInitAllFromEnv (로컬)** | DDL 경로 `core/ontology_so/postgres_ddl.sql`을 **레포 루트 기준**으로 찾는데, CWD 또는 `GOPEDIA_REPO_ROOT`가 레포 루트가 아님. | `go test`를 **레포 루트(gopedia)**에서 실행하고 `GOPEDIA_REPO_ROOT` 비우거나 `"."` 로 두기. **수정됨**: `InitPostgresFromEnv`에서 `repoRoot`가 비어 있거나 `"."` 이면 상위 디렉터리를 탐색해 `core/ontology_so/postgres_ddl.sql`이 있는 디렉터리를 레포 루트로 사용함. |
| **Docker 테스트 유일 실패** | Postgres DB에는 연결되지만 **`documents` 테이블이 없음**. | **한 번만** DDL 적용: `tests/initialize.py`의 `DBInitializer().init_all()` 또는 `core/ontology_so/postgres_ddl.sql`을 해당 Postgres에 실행. |

---

## 4. 권장 절차

1. **Postgres 초기화 (최초 1회)**  
   - Docker/로컬 중 Postgres에 접근 가능한 환경에서:
   - Python: `from tests.initialize import DBInitializer; DBInitializer().init_all()`  
   - 또는: `psql -h <POSTGRES_HOST> -U <POSTGRES_USER> -d <POSTGRES_DB> -f core/ontology_so/postgres_ddl.sql`

2. **연동 검증은 Docker 네트워크 테스트로**  
   - `./scripts/run_integration_tests_docker_networks.sh`  
   - (이미지 푸시 후) `docker compose up -d` 로 phloem-flow 기동한 뒤 실행.

3. **로컬에서만 테스트할 때**  
   - `.env`를 호스트용으로 바꾸거나,  
   - TypeDB/Qdrant/Postgres/Phloem을 로컬에 띄우고 `localhost` 등으로 설정 후 테스트.

이후 위 순서대로 진행하면 Docker compose 기준 연동 테스트는 `documents` 생성 후 통과 가능함.
