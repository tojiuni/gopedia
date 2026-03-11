# 테스트 초기화 및 연동 테스트 실행 가이드

AI/자동화가 그대로 따라 실행할 수 있도록, **DB 초기화**와 **연동 테스트 실행** 절차를 정리한 문서입니다.

---

## 1. 사전 조건

- 레포 루트: `gopedia` (이 문서의 경로는 모두 레포 루트 기준)
- `.env` 파일이 있고, 다음 변수가 설정되어 있음:
  - `POSTGRES_HOST`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
  - `TYPEDB_HOST`, `TYPEDB_PORT`, `TYPEDB_DATABASE`
  - `QDRANT_HOST`, `QDRANT_PORT`, `QDRANT_COLLECTION`
- Docker 네트워크 `neunexus`, `traefik-net` 존재 (연동 테스트용)
- (선택) Phloem 서비스 기동: `docker compose up -d`

---

## 2. DB 초기화 (테이블/DB/컬렉션 없으면 생성)

연동 테스트 전에 **최소 1회** 실행합니다.

### 2.1 Python: DBInitializer (권장)

환경 변수(`.env`)를 읽어 Postgres / TypeDB / Qdrant 를 한 번에 초기화합니다.

```bash
cd /morphogen/neunexus/gopedia
# .env 로드 후 Python에서 실행
python -c "
from dotenv import load_dotenv
load_dotenv('.env')
from tests.initialize import DBInitializer
r = DBInitializer().init_all(skip_missing=True)
print('init_all result:', r)
"
```

- **의존성**: `psycopg`, `typedb-driver`, `qdrant-client`, `python-dotenv` (레포 `requirements.txt` 참고)
- **동작**: `documents` 테이블(Postgres), TypeDB DB+스키마, Qdrant 컬렉션을 없을 때만 생성

### 2.2 Go: InitAllFromEnv

Postgres DDL + Qdrant 컬렉션만 초기화합니다 (TypeDB는 Python 또는 `typedb_init.py` 사용).

```bash
cd /morphogen/neunexus/gopedia
# .env 로드 후 Go 테스트로 초기화 (레포 루트에서 실행)
set -a && [ -f .env ] && . ./.env; set +a
go test ./tests/integration/ -v -run TestInitAllFromEnv -count=1
```

- **동작**: `core/ontology-so/postgres_ddl.sql` 적용, Qdrant 컬렉션 생성. 레포 루트는 자동 탐색.

### 2.3 Postgres만: Docker로 DDL 적용

다른 DB는 이미 준비되어 있고 Postgres만 초기화할 때 사용합니다.  
`POSTGRES_HOST`가 Docker 내부 호스트명(예: `postgres_db`)이면, 해당 네트워크에서 실행합니다.

```bash
cd /morphogen/neunexus/gopedia
# neunexus 네트워크에서 postgres_db 로 DDL 적용
docker run --rm --network neunexus --env-file .env \
  -v "$(pwd)/core/ontology-so/postgres_ddl.sql:/sql/postgres_ddl.sql" \
  postgres:16-alpine sh -c 'export PGPASSWORD="$POSTGRES_PASSWORD"; psql -h "$POSTGRES_HOST" -p "${POSTGRES_PORT:-5432}" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f /sql/postgres_ddl.sql'

# traefik-net 에서도 동일하게 적용할 때
docker run --rm --network traefik-net --env-file .env \
  -v "$(pwd)/core/ontology-so/postgres_ddl.sql:/sql/postgres_ddl.sql" \
  postgres:16-alpine sh -c 'export PGPASSWORD="$POSTGRES_PASSWORD"; psql -h "$POSTGRES_HOST" -p "${POSTGRES_PORT:-5432}" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f /sql/postgres_ddl.sql'
```

---

## 3. 연동 테스트 실행

### 3.1 Docker 네트워크 연동 테스트 (권장)

`neunexus`, `traefik-net` 에서 내부 호스트명(typedb, qdrant, postgres_db, phloem-flow)으로 연결을 검증합니다.  
**DB 초기화(2절)를 먼저 수행한 뒤** 실행합니다.

```bash
cd /morphogen/neunexus/gopedia
./scripts/run_integration_tests_docker_networks.sh
```

- **필요 조건**: `.env` 존재, Docker 네트워크 `neunexus`, `traefik-net` 존재, phloem-flow 등 서비스 기동 여부는 테스트 대상에 따름
- **실행 내용**: 각 네트워크별 컨테이너에서 `go test ./tests/integration/ -run 'DockerNetwork|ConnectivityOver'` 실행

### 3.2 로컬 Go 연동 테스트

호스트에서 `.env`를 로드한 뒤 Go 연동 테스트를 돌립니다.  
`.env`에 Docker 내부 호스트명만 있으면 대부분 실패할 수 있으므로, 로컬에서 접근 가능한 호스트로 설정했을 때 사용합니다.

```bash
cd /morphogen/neunexus/gopedia
set -a && [ -f .env ] && . ./.env; set +a
go test ./tests/integration/ -v -count=1
```

### 3.3 로컬 Python 연동 테스트

```bash
cd /morphogen/neunexus/gopedia
python -m pytest tests/ -v -m integration --tb=short
```

- conftest에서 `.env`를 로드합니다.

---

## 4. 한 번에 실행 (초기화 → Docker 연동 테스트)

아래 순서로 실행하면, 초기화 후 Docker 네트워크 연동 테스트까지 한 번에 진행할 수 있습니다.

```bash
cd /morphogen/neunexus/gopedia

# 1) Postgres DDL 적용 (neunexus, traefik-net 각각)
for net in neunexus traefik-net; do
  docker run --rm --network "$net" --env-file .env \
    -v "$(pwd)/core/ontology-so/postgres_ddl.sql:/sql/postgres_ddl.sql" \
    postgres:16-alpine sh -c 'export PGPASSWORD="$POSTGRES_PASSWORD"; psql -h "$POSTGRES_HOST" -p "${POSTGRES_PORT:-5432}" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f /sql/postgres_ddl.sql' || true
done

# 2) Docker 네트워크 연동 테스트
./scripts/run_integration_tests_docker_networks.sh
```

- `|| true`는 해당 네트워크에서 `postgres_db` 등이 없을 때 스킵하기 위함입니다.

---

## 5. 참고 파일

| 파일 | 용도 |
|------|------|
| `tests/initialize.py` | Python `DBInitializer` (Postgres / TypeDB / Qdrant 초기화) |
| `tests/integration/init.go` | Go `InitAllFromEnv`, `InitPostgresFromEnv`, `InitQdrantFromEnv` |
| `tests/integration/init_test.go` | `TestInitAllFromEnv` (Go로 초기화 실행) |
| `core/ontology-so/postgres_ddl.sql` | Postgres `documents` 테이블 DDL |
| `core/ontology-so/typedb_schema.typeql` | TypeDB 스키마 (Python 또는 `typedb_init.py` 사용) |
| `scripts/run_integration_tests_docker_networks.sh` | neunexus / traefik-net 연동 테스트 실행 |
