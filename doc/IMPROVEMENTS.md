# Gopedia 개선 항목 백로그

v0.1.0 RAG 테스트 결과 및 운영 경험에서 도출된 개선 항목.  
각 항목은 우선순위(P1–P3)와 카테고리로 분류한다.

> **연관 문서**
> - 테스트 리포트: [`doc/rag-test-reports/v0.1.0_2026-04-01_neunexus-gopedia.md`](rag-test-reports/v0.1.0_2026-04-01_neunexus-gopedia.md)
> - 버전 관리 가이드: [`doc/rag-test-reports/README.md`](rag-test-reports/README.md)

---

## P1 — 즉시 수정 필요

### IMP-01: 중복 인제스트 방지 ✅ v0.2.0
- **카테고리**: Phloem / Sink
- **현상**: 동일 파일이 다른 project_id로 두 번 인제스트되면 `knowledge_l1`, `knowledge_l3`, Qdrant 벡터가 중복 생성됨.  
  예: `Gopedia Feature Guide`가 project_id 5 / 14 양쪽에 존재 → 검색 결과에 동일 hit 2개 반복 노출.
- **해결**: `Sink.Write()`에서 `(title, source_type, l2_child_hash)` 기준으로 기존 L1을 조회.
  content hash가 일치하면 기존 document_id를 즉시 반환하여 중복 L1/L2/L3/Qdrant 생성을 방지.
- **관련 파일**: `internal/phloem/sink/writer.go`

---

### IMP-02: 검색 결과에 source_path / doc_name 노출 ✅ v0.2.0
- **카테고리**: Xylem / API
- **현상**: `Readme`, `Skill`, `Index` 등 generic title 문서가 검색 결과에 나타날 때 어느 프로젝트/파일에서 왔는지 알 수 없음.
- **해결**:
  - `CONTEXT_FOR_L3_SQL`에 `k1.source_type` 추가.
  - code hits: `source_path = l1_title` (절대 파일 경로).
  - markdown hits: `doc_name = l2_source_metadata["name"] or project_id`.
  - `fetch_rich_context` ctx dict에 `source_path`, `doc_name` 포함.
- **관련 파일**: `flows/xylem_flow/retriever.py`

---

## P2 — 다음 릴리즈 (v0.2.0)

### IMP-03: 한/영 혼용 쿼리 임베딩 품질 개선 ✅ v0.2.0
- **카테고리**: Phloem / Embedding
- **현상**: 한국어 전용 기술 용어(예: `Smart Sink`, `파티션`)가 포함된 문서가 영어 쿼리에서 score 0.474로 낮게 검색됨.
- **해결**: `annotateKoreanTerms()` 함수를 `summarizeL2()`에 적용.
  주요 한국어 기술 용어(파티션, 워터마킹, 싱크, 인제스트, 임베딩 등)를 감지하여
  `[en: partition watermarking ...]` 어노테이션을 L2 summary에 추가 후 임베딩.
  20개 용어 룩업테이블 방식; multilingual-e5-large 모델 도입 시 제거 가능.
- **관련 파일**: `internal/phloem/sink/writer.go`

---

### IMP-04: `run` entrypoint 코드 파일 자동 라우팅 ✅ v0.2.0
- **카테고리**: Phloem / Ingest
- **현상**: `property.root_props.run`으로 디렉토리를 인제스트할 때 `.py`, `.go` 등 코드 파일을 만나도 markdown pipeline으로 잘못 라우팅됨.
- **해결**: `run.py` 리라이트 — `_collect_all_paths()`가 markdown+code 확장자를 모두 수집;
  메인 루프에서 확장자별로 `ingest_code_file()` 또는 markdown 경로로 자동 분기.
- **관련 파일**: `property/root_props/run.py`, `property/root_props/run_code.py`

---

### IMP-05: Gardener 코드 도메인 smoke 데이터셋 등록 ✅ v0.2.0
- **카테고리**: 품질 테스트
- **현상**: 코드 도메인용 평가 데이터셋이 없어 코드 검색 품질을 정량적으로 추적하지 못함.
- **해결**: `dataset/code_domain_smoke.json` 추가 (6 queries, bronze tier).
  대상: `verify_xylem_flow.py`, `chunker/code.go`.
  쿼리: pg_connect_func, code_chunker_struct, l3_lines_field, build_code_chunks, restore_code_l2, xylem_verify_schema.
- **관련 파일**: `dataset/code_domain_smoke.json`, `doc/guide/code-domain.md`

---

## P3 — 중장기

### IMP-06: Gopedia 버전 태그 관리 자동화 ✅ v0.2.0
- **카테고리**: 릴리즈 / DevOps
- **해결**:
  - `CHANGELOG.md` 도입 (Keep a Changelog 형식).
  - `scripts/tag-release.sh`: pre-flight checks → annotated tag → push → `gh release create`.
  - CHANGELOG 섹션을 자동으로 GitHub Release body에 사용.
- **관련 파일**: `CHANGELOG.md`, `scripts/tag-release.sh`, `doc/rag-test-reports/README.md`

---

### IMP-07: 인제스트 이력 추적 (Audit log) ✅ v0.2.0
- **카테고리**: Phloem / 운영
- **현상**: 어떤 파일이 언제, 어떤 버전으로 인제스트되었는지 DB에서 쉽게 조회할 수 없음.
- **해결**: `documents` 테이블에 `ingest_version TEXT` 컬럼 추가.
  `GOPEDIA_VERSION` 환경변수(기본값 `dev`)를 `Sink.Write()` 시 INSERT/UPDATE에 기록.
- **관련 파일**: `core/ontology_so/postgres_ddl.sql`, `internal/phloem/sink/writer.go`

---

---

## P1 — v0.3.0 신규

### IMP-08: multilingual-e5-large 임베딩 모델 도입 ✅ v0.3.0
- **카테고리**: Phloem+Xylem / Embedding
- **현상**: OpenAI `text-embedding-3-small` (1536 dims)은 G6 `smart sink routing strategy` 0.474로 한국어-영어 혼용 쿼리에 취약; API 비용 발생.
- **해결**:
  - `python/embedding_service/` — FastAPI + `sentence-transformers` 로컬 서비스 (포트 18789)
  - `Dockerfile.embedding` — `intfloat/multilingual-e5-large` 모델 사전 다운로드 포함
  - `internal/phloem/embedder/local.go` — Go HTTP embedder, `passage:` prefix로 L3 임베딩
  - `embedder.Embedder` 인터페이스에 `VectorSize() int` 추가 → Qdrant 컬렉션 크기 자동 설정
  - `GOPEDIA_EMBEDDING_BACKEND=local|openai` 환경변수로 백엔드 선택
  - Python retriever: `embed_query_local()` 추가, `query:` prefix 사용
  - Qdrant 벡터 크기: 1536 → 1024
- **측정 효과**: neunexus 평균 0.678 → 0.899 (+0.221), gopedia 평균 0.594 → 0.869 (+0.275)
  G6 smart sink: 0.474 → 0.841 (+0.367). MRR@10: 0.389 → 0.560 (+44%).
- **관련 파일**: `python/embedding_service/main.py`, `Dockerfile.embedding`, `internal/phloem/embedder/local.go`, `internal/phloem/embedder/embedder.go`, `internal/api/api.go`, `flows/xylem_flow/retriever.py`, `flows/xylem_flow/project_config.py`, `docker-compose.dev.yml`

---

## P1 — v0.4.0 신규

### IMP-09: L3 청크 컨텍스트 보강 (Contextual Embedding) ✅ v0.4.0
- **카테고리**: Phloem / Sink / Embedding
- **현상**: 짧은 영어 bullet-point L3 청크(`"- **Distro**: Rocky Linux 9.6"`)가 한국어 쿼리와 semantic gap.
  v0.3.0에서 q_server_os_specs, q_docker_registry_auth, q_gopedia_envelope_strategy가 top-30 밖으로 회귀.
- **해결**: L3 임베딩 시 섹션 제목 행을 청크 앞에 prepend.
  `embedText = headingLine + "\n" + l3.text` (예: `"## Server OS\n- **Distro**: Rocky Linux 9.6..."`)
  - `l3ToEmbed` 구조체에 `headingText string` 필드 추가.
  - 마크다운 L3 빌드 루프에서 `headingText: headingLine` 설정.
  - 코드 도메인 청크는 `headingText` 비워둠 (함수 시그니처가 이미 컨텍스트 역할).
  - 저장된 L3 content는 변경 없음 — 임베딩 텍스트만 달라짐.
- **관련 파일**: `internal/phloem/sink/writer.go`

---

### IMP-10: Universitas Architecture Blueprint 인제스트 ✅ v0.4.0
- **카테고리**: Phloem / Ingest / 데이터
- **현상**: `q_universitas_bio_groups` — "neunexus, taxon, osteon은 인체의 어떤 체계를 모사하나" 쿼리가 2 버전 연속 MISS.
  원인: `Universitas_System_Architecture_Blueprint.md`가 인제스트되지 않아 관련 L3 청크가 존재하지 않음.
- **해결**: `universitas_architecture` 프로젝트 신규 생성 후 다음 파일 인제스트:
  - `/neunexus/geneso/universitas/Universitas_System_Architecture_Blueprint.md`
  - `/neunexus/geneso/universitas/docs/arch/bio_inspired_blueprint.md`
- **관련 파일**: 인제스트 런타임 운영 (코드 변경 없음)

---

## 항목 요약

| ID | 우선순위 | 카테고리 | 제목 | 상태 |
|----|----------|----------|------|------|
| IMP-01 | **P1** | Phloem/Sink | 중복 인제스트 방지 | ✅ v0.2.0 |
| IMP-02 | **P1** | Xylem/API | 검색 결과 source_path / doc_name 노출 | ✅ v0.2.0 |
| IMP-03 | P2 | Phloem/Embedding | 한/영 혼용 임베딩 품질 개선 | ✅ v0.2.0 |
| IMP-04 | P2 | Phloem/Ingest | `run` entrypoint 코드 파일 자동 라우팅 | ✅ v0.2.0 |
| IMP-05 | P2 | 품질 테스트 | Gardener 코드 도메인 smoke 데이터셋 등록 | ✅ v0.2.0 |
| IMP-06 | P3 | 릴리즈/DevOps | 버전 태그 관리 자동화 + CHANGELOG | ✅ v0.2.0 |
| IMP-07 | P3 | Phloem/운영 | 인제스트 이력 추적 (Audit log) | ✅ v0.2.0 |
| IMP-08 | **P1** | Phloem+Xylem/Embedding | multilingual-e5-large 도입 + 로컬 임베딩 서비스 | ✅ v0.3.0 |
| IMP-09 | **P1** | Phloem/Sink/Embedding | L3 청크 컨텍스트 보강 (Contextual Embedding) | ✅ v0.4.0 |
| IMP-10 | **P1** | Phloem/Ingest/데이터 | Universitas Architecture Blueprint 인제스트 | ✅ v0.4.0 |
