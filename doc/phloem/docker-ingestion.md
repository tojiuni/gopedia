# Docker로 프로젝트 인제스트만 실행 (Phloem)

전체 사용 요약: [USAGE.md](./USAGE.md)

`scripts/run_project_ingestion_docker.sh`는 **DB 리셋·스키마 초기화 없이**, 이미 떠 있는 스택(Postgres, Qdrant, TypeDB, **Phloem**)에 대해 Root 워커를 Docker로 돌려 **디렉터리(또는 단일 파일) 인제스트**만 수행합니다.

`run_fresh_ingestion_docker.sh`와 달리 `reset_rhizome_docker.py` / `DBInitializer.init_all()` 을 호출하지 않습니다.

## 전제

- `.env` (또는 `GOPEDIA_ENV_FILE`)에 스택 접속 정보가 있어야 합니다.
- Docker 네트워크 `DOCKER_NETWORK_EXTERNAL`(기본 `neunexus`)에 DB·Qdrant·TypeDB·Phloem이 붙어 있어야 합니다.
- Phloem gRPC 주소: 기본 `GOPEDIA_PHLOEM_GRPC_ADDR=phloem-e2e:50051` (컨테이너 이름이 다르면 환경 변수로 지정).

## 사용법

```bash
cd /path/to/gopedia/repo

# 인제스트 이미지가 없으면 한 번 빌드
docker build -f Dockerfile.ingestion -t gopedia-ingestion:test .

# 호스트의 프로젝트 루트(또는 단일 .md) 경로를 인자로 전달
./scripts/run_project_ingestion_docker.sh /morphogen/neunexus/project_skills/wiki/universitas/gopedia
```

Phloem을 같이 띄우려면(이미지 빌드 + 백그라운드 컨테이너):

```bash
./scripts/run_project_ingestion_docker.sh --build-ingestion-image --start-phloem \
  /morphogen/neunexus/project_skills/wiki/universitas/gopedia
```

## 동작 요약

| 항목 | 내용 |
|------|------|
| 컨테이너 | `Dockerfile.ingestion` 이미지에서 `python -m property.root_props.run <경로>` 실행 |
| 레포 밖 경로 | 읽기 전용으로 `/mnt/gopedia_project_ingest` 에 마운트 후 그 경로를 인자로 전달 |
| 레포 안 경로 | `/app/...` 로 매핑 (레포 루트를 `/app` 에 마운트) |

## 문제 해결

- **`RegisterProject` / `documents_project_id_fkey`**: 배포 DB의 `projects` 테이블은 `machine_id`, `name`, `root_path` 등 **실제 스키마**를 씁니다. 예전 Phloem이 존재하지 않는 `display_name` 컬럼만 넣으면 `RegisterProject`가 실패하고, 잘못된 바이너리·proto 조합에서는 `project_id`가 깨져 FK 오류가 납니다. **항상 레포의 최신 `cmd/phloem`으로 이미지를 다시 빌드**하세요 (`--start-phloem`이 그렇게 합니다).
- **`no pipeline registered for domain: folder`**: Phloem 바이너리가 오래되어 `folder` 도메인이 등록되지 않은 경우입니다. 이미지 재빌드로 해결됩니다.
- **`FAIL … err=…`**: 최신 Root는 Phloem `error_message`를 stderr에 붙입니다. `sink: postgres …`, `sink: qdrant …` 등으로 원인을 좁힙니다.
- **`project_id` 0**: Sink는 `project_id=0`을 NULL로 넣어 `projects(id)` FK를 피합니다. Root는 유효한 ID가 있을 때만 메타에 `project_id`를 넣습니다.
- **ingestion 이미지에 `pathspec` 없음**: `Dockerfile.ingestion`은 기본적으로 `requirements.txt`만 COPY합니다. 레포를 `/app`에 마운트할 때는 호스트 `requirements.txt`와 일치하는지 확인하거나, 컨테이너 안에서 `pip install -r requirements.txt`를 한 번 실행하세요.

## 관련 스크립트

- 전체 초기화 + E2E: `scripts/run_fresh_ingestion_docker.sh`
- Phloem 아키텍처: [README.md](./README.md)
