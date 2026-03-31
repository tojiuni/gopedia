# Gopedia 개선 항목 백로그

v0.1.0 RAG 테스트 결과 및 운영 경험에서 도출된 개선 항목.  
각 항목은 우선순위(P1–P3)와 카테고리로 분류한다.

> **연관 문서**
> - 테스트 리포트: [`doc/rag-test-reports/v0.1.0_2026-04-01_neunexus-gopedia.md`](rag-test-reports/v0.1.0_2026-04-01_neunexus-gopedia.md)
> - 버전 관리 가이드: [`doc/rag-test-reports/README.md`](rag-test-reports/README.md)

---

## P1 — 즉시 수정 필요

### IMP-01: 중복 인제스트 방지
- **카테고리**: Phloem / Sink
- **현상**: 동일 파일이 다른 project_id로 두 번 인제스트되면 `knowledge_l1`, `knowledge_l3`, Qdrant 벡터가 중복 생성됨.  
  예: `Gopedia Feature Guide`가 project_id 5 / 14 양쪽에 존재 → 검색 결과에 동일 hit 2개 반복 노출.
- **개선 방향**:
  - `Sink.Write()`에서 `(title, content_hash)` 기준으로 기존 L1을 조회.
  - 이미 존재하면 새 L1을 만들��� 않고 **project 연결(`documents.project_id`)만 업데이트**.
  - 새로운 content_hash일 때만 L2/L3/Qdrant 전체 재생성 (버전업).
- **관련 파일**: `internal/phloem/sink/writer.go`

---

### IMP-02: 검색 결과에 source_path / project_name 노출
- **카테고리**: Xylem / API
- **현상**: `Readme`, `Skill`, `Index` 등 generic title 문서가 검색 결과에 나타날 때 어느 프로젝트/파일에서 왔는지 알 수 없음.
- **개선 방향**:
  - `SearchHit`에 `source_path` 필드 추가 (현재 payload에 없거나 빈 값으로 반환).
  - `documents.source_metadata->>'name'` 또는 프로젝트 정보(`projects` 테이블)에서 `project_name`을 `SearchHit`에 포함.
  - `detail=summary` 응답에도 `source_path`가 포함되도록 보장 (agent-interop.md에 명시되어 있으나 실제 값이 비어 있는 케이스 있음).
- **관련 파일**: `flows/xylem_flow/retriever.py`, `internal/api/search_shape.go`

---

## P2 — 다음 릴리즈 (v0.2.0)

### IMP-03: 한/영 혼용 쿼리 임베딩 품질 개선
- **카테고리**: Xylem / Embedding
- **현상**: 한국어 전용 기술 용어(예: `Smart Sink`, `파티션`)가 포함된 문서가 영어 쿼리에서 score 0.474로 낮게 검색됨.
- **개선 방향**:
  - L2 요약(`knowledge_l2.summary`) 생성 시 주요 한국어 기술 용어의 영어 동의어를 병기 (예: `파티션 (partition)`, `워터��킹 (watermarking)`).
  - 또는 L3 임베딩 시 한/영 hybrid 방식 적용 (multilingual embedding model 검토: `multilingual-e5-large` ��).
- **관련 파일**: `internal/phloem/embedder/`, `flows/xylem_flow/retriever.py`

---

### IMP-04: `run` entrypoint 코드 파일 자동 라우팅
- **카테고리**: Phloem / Ingest
- **현상**: `property.root_props.run`으로 디렉토리를 인제스트할 때 `.py`, `.go` ��� 코드 파일을 만나도 markdown pipeline으로 잘못 라우팅됨.
- **개선 방향**:
  - `run.py`에서 파일 확장��를 검사하여 코드 파일이면 `run_code.ingest_code_file()`로 자동 ���임.
  - 단일 entrypoint(`python -m property.root_props.run <path>`)로 markdown + code 혼합 디렉토리를 한 번에 인제스트 가능하도록.
- **관련 파일**: `property/root_props/run.py`, `property/root_props/run_code.py`

---

### IMP-05: Gardener 코드 도메인 smoke 데이터셋 등록
- **카테고리**: 품질 테스트
- **현상**: `gardener_gopedia`에 코드 도메인용 평가 데이터셋이 없어 코드 검색 품질을 정량적으로 추적하지 못함.
- **개선 방향**:
  - `doc/guide/code-domain.md §Quality testing` 의 샘플 JSON을 ���제 Gardener에 등록.
  - 최소 쿼리 4개(함수명, 구조체명, 기능 설명, 코드 복원) 포함.
  - `gardener-smoke`에 코드 쿼리 포함되도록 smoke 데이터셋 업데이트.
- **관련 파일**: `gardener_gopedia/dataset/`, `doc/guide/code-domain.md`

---

## P3 — 중장기

### IMP-06: Gopedia 버전 태그 관리 자동화
- **카테고리**: 릴리즈 / DevOps
- **현상**: 현재 태그(`v0.1.0`)를 수동으로 생성·push하고 있으며, RAG 테스트 리포트와 태그 간 연결이 문서에만 존재함.
- **개선 방향**:

  **① 태그 명명 ���칙 표준화**
  ```
  v<major>.<minor>.<patch>
  
  major: 파이프라인 아키텍처 변경 (새 도메인, 스키마 브레이킹 변경)
  minor: 기능 추가 (새 entrypoint, 새 restore API 등)
  patch: 버그픽스, 문서, 테스트
  ```

  **② CHANGELOG.md 도입**
  - 프로젝트 루트�� `CHANGELOG.md` 추가.
  - `git tag` 생성 시 해당 버전 섹션 작성 (Keep a Changelog 형식).

  **③ RAG 테스트 리포��와 태그 연동**
  - 리포트 파일명에 태그 버전 포함 (현재 방��� 유지: `<version>_<date>_<target>.md`).
  - `doc/rag-test-reports/README.md`의 리포트 목록 테이블을 태그 릴리즈 시 함께 업데이트.

  **④ GitHub Release 활용 (선택)**
  - `git tag` push 후 `gh release create v<x>.<y>.<z>` 로 GitHub Release 페이지 생성.
  - Release body에 해당 RAG 테스트 리포트 링크 포함.

  ```bash
  # 태그 + Release 생성 예시
  git tag v0.2.0 -m "v0.2.0: 중복 인제스트 방지, source_path 노출"
  git push origin v0.2.0
  gh release create v0.2.0 \
    --title "v0.2.0 — IMP-01 중복 방지, IMP-02 source_path" \
    --notes "See doc/rag-test-reports/v0.2.0_<date>_<target>.md"
  ```

- **관련 파일**: `CHANGELOG.md` (신규), `doc/rag-test-reports/README.md`

---

### IMP-07: 인제스트 이력 추적 (Audit log)
- **카테고리**: Phloem / 운영
- **현상**: 어떤 파일이 언제, 어떤 버전으로 인제스트되었는지 DB에서 쉽게 조회할 수 없음.
- **개선 방향**:
  - `documents` 테이블에 `ingested_at`, `ingest_version`(gopedia semver) 컬럼 추가.
  - 또는 별도 `ingest_log` 테이블로 인제스트 이력 관리.
- **관련 파일**: `core/ontology_so/postgres_ddl.sql`, `internal/phloem/sink/writer.go`

---

## 항목 요약

| ID | 우선순위 | 카테고리 | 제목 |
|----|----------|----------|------|
| IMP-01 | **P1** | Phloem/Sink | 중복 인제스트 방지 |
| IMP-02 | **P1** | Xylem/API | 검색 결과 source_path / project_name 노출 |
| IMP-03 | P2 | Xylem/Embedding | 한/영 혼용 임베딩 품질 개선 |
| IMP-04 | P2 | Phloem/Ingest | `run` entrypoint 코드 파일 자동 라우팅 |
| IMP-05 | P2 | 품질 테스트 | Gardener 코드 도메인 smoke 데이터셋 등록 |
| IMP-06 | P3 | 릴리즈/DevOps | 버전 태그 관리 자동화 + CHANGELOG |
| IMP-07 | P3 | Phloem/운영 | 인제스트 이력 추적 (Audit log) |
