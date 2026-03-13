# 테스트 초기화 및 E2E 실행 가이드 (간단 버전)

현재 Ver1 기준으로 **DB 초기화**와 **Transpiration E2E 테스트**를 돌리기 위한 최소 플로우만 정리한 문서입니다.

---

## 1. 사전 조건

- **레포 루트**: `/morphogen/neunexus/gopedia`
- **`.env` 필수 항목**:
  - PostgreSQL: `POSTGRES_HOST`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
  - TypeDB: `TYPEDB_HOST`, `TYPEDB_PORT`, `TYPEDB_DATABASE`
  - Qdrant (문서 컬렉션, 1536차원):
    - `QDRANT_HOST`, `QDRANT_PORT`
    - `QDRANT_DOC_COLLECTION=gopedia_document`
    - `QDRANT_DOC_VECTOR_NAME=wiki`
    - `QDRANT_DOC_VECTOR_SIZE=1536`
  - Embedding:
    - `OPENAI_API_KEY`
    - `OPENAI_EMBEDDING_MODEL=text-embedding-3-small` (또는 1536차원 모델)
- **Docker 네트워크**: `neunexus` (DB 컨테이너와 통신용)
- (권장) `docker compose up -d` 로 Phloem, DB, Qdrant 등 서비스 기동

---

## 2. DB 초기화 (Python DBInitializer, 단일 플로우)

`tests/initialize.py`의 `DBInitializer` 를 사용해 **Postgres / TypeDB / Qdrant** 를 한 번에 초기화합니다.

```bash
cd /morphogen/neunexus/gopedia

python -c "
from dotenv import load_dotenv
load_dotenv('.env')
from tests.initialize import DBInitializer
r = DBInitializer().init_all(skip_missing=True)
print('init_all result:', r)
"
```

- **동작 요약**:
  - Postgres: `core/ontology-so/postgres_ddl.sql` 적용(`documents` 테이블 등)
  - TypeDB: DB 존재 여부 확인 후 스키마(`core/ontology-so/typedb_schema.typeql`) 적용
  - Qdrant:
    - 기존 마크다운 컬렉션(`QDRANT_COLLECTION`) 보장
    - 문서 컬렉션 `QDRANT_DOC_COLLECTION` (기본 `gopedia_document`)을
      - named vector `QDRANT_DOC_VECTOR_NAME` (기본 `wiki`)
      - `QDRANT_DOC_VECTOR_SIZE=1536`
      로 생성하고 seed 벡터 1건 upsert

> `scripts/run_ingestion_docker.sh` 내부에서도 동일한 `DBInitializer` 를 사용하므로, E2E 스크립트만 사용하는 경우 별도 수동 초기화는 필요 없습니다.

---

## 3. Docker 기반 End-to-End: Ingestion + Transpiration (권장)

하나의 스크립트로 **DB 초기화 → 샘플 마크다운 인게스트 → Transpiration 검증**까지 한 번에 실행합니다.

```bash
cd /morphogen/neunexus/gopedia
./scripts/run_ingestion_docker.sh tests/fixtures/sample.md Introduction
```

- **스크립트 동작**:
  - `Dockerfile.ingestion` 기반으로 `gopedia-ingestion:test` 이미지를 빌드
  - `neunexus` 네트워크에서 컨테이너 실행:
    - 컨테이너 안에서 `.env` 를 사용해 `DBInitializer().init_all(skip_missing=True)` 실행
    - `./scripts/run_transpiration_e2e.sh tests/fixtures/sample.md Introduction` 실행
- **내부 Transpiration 플로우** (`scripts/run_transpiration_e2e.sh`):
  1. `python -m property.root_props.run tests/fixtures/sample.md`
     - Phloem을 통해 Postgres/Qdrant 인게스트
     - `TYPEDB_HOST` 가 설정되어 있으면 같은 프로세스에서 TypeDB sync 수행
  2. `python scripts/verify_transpiration.py "Introduction"`
     - OpenAI 임베딩(1536차원)
     - Qdrant `QDRANT_DOC_COLLECTION` / `QDRANT_DOC_VECTOR_NAME` 로 검색
     - TypeDB에서 document/section/composition fetch 쿼리 실행

**성공 기준**:

- 스크립트가 `=== Transpiration E2E done (OK) ===` 로 종료 (exit code 0)
- 로그에
  - `OK ... -> doc_id=... machine_id=...`
  - `Qdrant hits (score, doc_id, section_id, toc_path):`
  - `Transpiration check done.`
  가 출력됨

---

## 4. 이미 초기화된 환경에서 Transpiration만 실행할 때

DB 초기화가 끝난 상태라면, Docker 없이 또는 ingestion Docker 컨테이너 안에서 직접 E2E 스크립트를 실행할 수 있습니다.

- **샘플 인게스트 + Transpiration 한 번에**:

```bash
cd /morphogen/neunexus/gopedia
./scripts/run_transpiration_e2e.sh [path-to-sample.md] [keyword]
```

- 인자를 생략하면 기본값:
  - `path-to-sample.md` → `tests/fixtures/sample.md`
  - `keyword` → `"Introduction"`

- **Transpiration만 수동 실행** (이미 인게스트가 되어 있을 때):

```bash
cd /morphogen/neunexus/gopedia
python scripts/verify_transpiration.py "Introduction"
```

---

## 5. 참고 파일

| 파일 | 용도 |
|------|------|
| `tests/initialize.py` | Python `DBInitializer` (Postgres / TypeDB / Qdrant 초기화) |
| `core/ontology-so/typedb_schema.typeql` | TypeDB document/section/composition 스키마 |
| `core/ontology_so/typedb_sync.py` | 인게스트된 문서를 TypeDB document/section/composition 으로 동기화 |
| `scripts/run_ingestion_docker.sh` | Docker 이미지 빌드 + 컨테이너에서 DB 초기화 및 Transpiration E2E 실행 |
| `scripts/run_transpiration_e2e.sh` | 샘플 마크다운 인게스트 + `verify_transpiration.py` 실행 |
| `scripts/verify_transpiration.py` | OpenAI 임베딩 → Qdrant 검색 → TypeDB fetch 로 Transpiration 검증 |
| `doc/transpiration-usage.md` | Transpiration 흐름 및 Efficiency/Speed 성공 기준 |

