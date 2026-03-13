# Ingestion Docker 테스트 실패 지점 정리

`scripts/run_ingestion_docker.sh` 로 Docker 내부에서 인게스트·Transpiration E2E를 돌릴 때, 어디에서 막히는지 간단 정리.

---

## 1. 실패 가능 지점 요약

| 단계 | 실패 시 메시지/원인 | 대응 |
|------|---------------------|------|
| **Phloem gRPC 연결** | `ConnectionRefused`, `name resolver error` | `phloem-flow` 컨테이너가 `neunexus` 네트워크에 있고 기동 중인지 확인. `docker compose up -d` 또는 동일 네트워크의 Phloem 서비스 확인. |
| **Root → Phloem 인게스트** | `Ingest failed`, gRPC 오류 | 위와 동일. `.env`의 `GOPEDIA_PHLOEM_GRPC_ADDR`는 스크립트가 `phloem-flow:50051`로 덮어씀. |
| **TypeDB sync** | `typedb-driver not installed` / `cannot import name 'SessionType'` | 이미지에 `typedb-driver` 2.x 설치됨. 3.x가 올라가면 API 불일치로 실패 → `requirements.txt`에서 `typedb-driver>=2.28,<3` 유지. |
| **TypeDB sync** | TypeDB 서버 연결 실패 | `TYPEDB_HOST=typedb` 등으로 `neunexus` 내 TypeDB 컨테이너에 도달하는지 확인. |
| **TypeDB sync** | `Request generated error` | TypeDB 서버 3.x와 typedb-driver 2.x 간 프로토콜 불일치 가능. 서버 2.x 사용 또는 드라이버 3.x + 코드 수정 검토. |
| **verify_transpiration (Qdrant)** | `Qdrant search failed`, `name resolver error` | `QDRANT_HOST=qdrant`, `neunexus` 네트워크에 qdrant 컨테이너 있는지 확인. |
| **verify_transpiration (OpenAI)** | `The api_key client option must be set` / `OPENAI_API_KEY` | `.env`에 `OPENAI_API_KEY` 설정 후 `run_ingestion_docker.sh` 실행 (스크립트가 `--env-file .env`로 전달). |
| **verify_transpiration (TypeDB)** | `TypeDB: no document/section composition results` | 인게스트 + TypeDB sync가 선행되어 TypeDB에 document/section이 있어야 함. sync 실패 시 이 단계에서 결과 없음. |

---

## 2. 정상 흐름

1. **Docker 이미지**: `Dockerfile.ingestion` — `pip install --upgrade 'protobuf>=4.25'` 및 `requirements.txt` (python-frontmatter, typedb-driver 2.x 등).
2. **네트워크**: `--network neunexus` 로 phloem-flow, typedb, qdrant, postgres_db 호스트명 해석.
3. **인게스트**: Root가 `doc/sample.md` → Phloem gRPC → PG·Qdrant 기록. `TYPEDB_HOST` 있으면 동일 프로세스에서 TypeDB sync 시도.
4. **Transpiration**: `verify_transpiration.py "서론"` — OpenAI 임베딩 → Qdrant 검색 → TypeDB document/section 조회.

---

## 3. 실행 예시

```bash
cd /morphogen/neunexus/gopedia
# .env 에 OPENAI_API_KEY, POSTGRES_*, TYPEDB_*, QDRANT_* 등 설정 후
./scripts/run_ingestion_docker.sh doc/sample.md "서론"
```

실패 시 위 표의 메시지/원인으로 단계별로 확인하면 됨.
