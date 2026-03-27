# Phloem · Root 사용 가이드 (요약)

## 흐름

**Root**(`property.root_props`)가 디렉터리/파일을 읽고 → **gRPC**로 **Phloem**(`cmd/phloem`)에 넘기면 → Phloem이 **Postgres / Qdrant** 등 Rhizome에 기록합니다. 인제스트 시작 시 **`RegisterProject`**로 `projects` 행을 맞춘 뒤, 각 파일마다 **`IngestMarkdown`**이 호출됩니다.

## 로컬에서 빠르게

```bash
# Phloem (별 터미널, .env에 POSTGRES_*, QDRANT_*, OPENAI_* 등)
go run ./cmd/phloem

# Root (.env에 GOPEDIA_PHLOEM_GRPC_ADDR=localhost:50051)
python -m property.root_props.run /path/to/project_or_file.md
```

## Docker로 프로젝트만 인제스트 (DB 리셋 없음)

```bash
export GOPEDIA_ENV_FILE=/path/to/.env   # 스택용 .env
./scripts/run_project_ingestion_docker.sh --build-ingestion-image --start-phloem \
  /path/to/project_root
```

- 레포 밖 경로는 컨테이너에 읽기 전용 마운트됩니다.
- 스크립트가 실행 시 **`pip install -r requirements.txt`** 로 Root 의존성을 맞춥니다.
- 상세·옵션: [docker-ingestion.md](./docker-ingestion.md)

## 반드시 맞출 것

| 항목 | 설명 |
|------|------|
| **Phloem 이미지** | 코드·proto를 바꾼 뒤에는 Phloem **이미지를 다시 빌드**하세요. 오래된 바이너리는 `folder` 도메인 미등록, `RegisterProject` 불일치 등으로 깨집니다. `--start-phloem`이 현재 레포로 빌드합니다. |
| **`projects` 테이블** | Phloem의 `RegisterProject`는 배포 DB 스키마(`machine_id`, `name`, `root_path` 등)에 맞춰져 있습니다. 신규 DB는 `core/ontology_so/postgres_ddl.sql` 기준과 일치합니다. |
| **네트워크** | Docker에서는 `DOCKER_NETWORK_EXTERNAL`(기본 `neunexus`)에 Postgres·Qdrant·Phloem이 같이 붙어 있어야 합니다. |

## 오류를 빠르게 볼 때

- 로그에 **`FAIL … err=`** 가 붙으면 Phloem이 준 **원인 문자열**입니다 (`sink: postgres …`, `sink: qdrant …` 등).
- **`documents_project_id_fkey`**: `RegisterProject` 실패·오래된 Phloem·깨진 `project_id` 메타가 흔한 원인입니다 → Phloem 재빌드·재기동을 먼저 하세요.
- **`no pipeline registered for domain: folder`**: Phloem을 최신 소스로 다시 빌드하세요.

## 문서 더 보기

- 아키텍처·패키지 구조: [README.md](./README.md)
- Docker 인제스트 전제·트러블슈팅: [docker-ingestion.md](./docker-ingestion.md)
