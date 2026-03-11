# 연동 테스트 실패 내역 정리

연결 실패 시 **skip 없이 fail** 되도록 복구한 뒤 실행한 결과입니다.

---

## 1. Go 연동 테스트 (`go test ./tests/integration/ -v`)

### 실패한 테스트 (호스트에서 .env 로 실행 시)

| 테스트 | 실패 사유 |
|--------|------------|
| **TestConnectivityOverNeunexus** | 호스트에서 실행 시 내부 호스트명(typedb, qdrant, postgres, phloem-flow) 미해석. TypeDB(typedb:1729) 도달 불가, Qdrant `name resolver error: produced zero addresses`, Postgres `documents` 테이블 없음, Phloem(phloem-flow:50051) 도달 불가, EnsureQdrantCollection 실패. |
| **TestTypeDBReachable** | TypeDB에 연결 실패. `typedb:1729` 도달 불가 (호스트에서 내부 이름 미해석). |
| **TestQdrantConnect** | Qdrant `CollectionExists()` 실패. `rpc error: code = Unavailable desc = name resolver error: produced zero addresses` (호스트에서 `qdrant` 호스트명 미해석). |
| **TestPostgresConnect** | Postgres 연결 실패. `hostname resolving error: lookup postgres_db on 100.100.100.100:53: no such host` (호스트에서 `postgres_db` 미해석). |
| **TestPhloemGRPCReachable** | Phloem gRPC 서버 미기동. `localhost:50051` 연결 실패. |
| **TestPhloemIngestMarkdown** | 동일. Phloem 미도달. |
| **TestQdrantEnsureCollection** | Qdrant 연결 실패 (위와 동일, zero addresses). |

### 통과한 테스트

| 테스트 | 비고 |
|--------|------|
| **TestDockerNetworkExternalValid** | `DOCKER_NETWORK_EXTERNAL`이 neunexus 또는 traefik-net 이므로 PASS. |

### 스킵된 테스트

| 테스트 | 사유 |
|--------|------|
| **TestConnectivityOverTraefikNet** | `DOCKER_NETWORK_EXTERNAL=neunexus` 로 실행되어, traefik-net 전용 테스트는 skip. |

---

## 2. Python 연동 테스트 (`pytest tests/test_service_integration.py -v -m integration`)

### 실패한 테스트

| 테스트 | 실패 사유 |
|--------|------------|
| **test_phloem_grpc_reachable** | Phloem gRPC 서버 미기동. `localhost:50051` 연결 불가. |
| **test_phloem_ingest_markdown** | 동일. Phloem 미도달로 RPC 호출 불가. |

### 스킵된 테스트

| 테스트 | 사유 (추정) |
|--------|-------------|
| **test_typedb_connect** | `TYPEDB_HOST` 미설정 또는 `typedb-driver` 미설치. |
| **test_qdrant_http_connect** | `QDRANT_HOST` 미설정 또는 `qdrant-client` 미설치. |
| **test_qdrant_collection_optional** | 위와 동일. |
| **test_postgres_connect** | `POSTGRES_HOST`/`POSTGRES_USER` 미설정 또는 `psycopg` 미설치. |

---

## 3. 원인 요약 및 조치

| 원인 | 조치 |
|------|------|
| **호스트에서 Docker 내부 이름 사용** | `.env`에 `TYPEDB_HOST=typedb`, `QDRANT_HOST=qdrant`, `POSTGRES_HOST=postgres_db` 등으로 설정된 상태에서 **호스트**에서 테스트 시 해당 이름이 DNS로 해석되지 않아 연결 실패 → **Docker 네트워크(neunexus/traefik-net)에 붙은 컨테이너 안에서** 테스트 실행하거나, 호스트에서 쓸 때는 `localhost` 또는 실제 IP/호스트명으로 설정. |
| **Phloem 미기동** | `go run ./cmd/phloem` 또는 `docker compose up -d`(phloem-flow)로 Phloem 기동 후 재실행. |
| **Postgres `documents` 테이블 없음** | `core/ontology-so/postgres_ddl.sql` 한 번 실행. |
| **Postgres 호스트명 `postgres_db` 미해석** | 호스트에서 실행 시 `postgres_db`가 같은 네트워크/ DNS에 없으면 실패. Docker 내부에서 실행하거나, 호스트용으로 Postgres 접근 가능한 호스트명으로 `.env` 수정. |

---

## 4. Docker 네트워크 테스트로 검증하려면

내부 이름(typedb, qdrant, postgres, phloem-flow)으로 연결 검증은 **해당 Docker 네트워크에 붙은 컨테이너 안**에서 실행해야 합니다.

```bash
# neunexus / traefik-net 에서 연동 테스트 실행 (스크립트 사용 시)
./scripts/run_integration_tests_docker_networks.sh
```

위 스크립트는 각 네트워크로 컨테이너를 띄우고, 내부 호스트명으로 Go 연동 테스트를 실행합니다. TypeDB·Qdrant·Postgres가 해당 네트워크에 있고, 필요 시 Phloem(phloem-flow)도 같은 네트워크에 띄워 두면 연결 실패 없이 통과할 수 있습니다.
