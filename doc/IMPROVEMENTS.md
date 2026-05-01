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

---

## P1 — v0.8.0 신규 (Answer Agent 토큰 효율성)

> 배경: v0.7.0 테스트에서 search 결과만으로 답변 가능한 케이스가 0% — 모든 질문이 restore_l2/l1을 호출함.  
> 원인: snippet이 300자로 제한되어 LLM이 콘텐츠를 평가하지 못하고 즉각 escalation.  
> 목표: P1 3개 항목 적용 후 restore 없이 answer 가능한 케이스를 50% 이상으로.  
> 연관 리포트: [`doc/rag-test-reports/v0.7.0_2026-05-01_neunexus-answer-agent.md`](rag-test-reports/v0.7.0_2026-05-01_neunexus-answer-agent.md)

### IMP-09: search 결과에 l2_summary + surrounding_context 포함, snippet 확장 🔲 v0.8.0
- **카테고리**: Xylem / Answer Agent
- **현상**: `answer_agent._execute_search()`가 snippet을 300자로 자르고, `retrieve_and_enrich()`가 이미 fetch한 `l2_summary`와 `surrounding_context`를 버림. LLM이 평가할 정보가 부족해 즉시 restore_l2 호출.
- **해결**:
  - `snippet`: 300 → 500자 (`matched_content`)
  - `context` 필드 추가: `surrounding_context` 최대 600자 (neighbor window 내용)
  - `l2_summary` 필드 추가: 섹션 요약 전달 (LLM이 섹션 전체 내용을 미리 파악)
- **예상 효과**: factual 단순 질문(Q4류)은 search → answer로 단축. 복합 질문도 restore_l2 1회로 충분해질 가능성.
- **관련 파일**: `flows/xylem_flow/answer_agent.py` (`_execute_search()`)

---

### IMP-10: dedup 기준 l1_id → l2_id 변경 🔲 v0.8.0
- **카테고리**: Xylem / Answer Agent
- **현상**: `_execute_search()`에서 `l1_id` 기준으로 dedup → 같은 문서의 서로 다른 섹션 청크가 1개만 전달됨. LLM이 문서 내 여러 섹션의 정보를 한 번의 search로 볼 수 없어 restore_l1 호출.
- **해결**: dedup 기준을 `l2_id`로 변경. 같은 문서의 다른 섹션은 각각 전달, 완전 동일 섹션만 dedup.
  ```python
  # 변경 전
  seen_l1: set[str] = set()
  key = h.get("l1_id", "")
  # 변경 후
  seen: set[str] = set()
  key = h.get("l2_id") or h.get("l1_id", "")
  ```
- **예상 효과**: top_k=5일 때 최대 5개 섹션 전달 → 문서 내 분산 정보 질문에서 restore_l1 불필요.
- **관련 파일**: `flows/xylem_flow/answer_agent.py` (`_execute_search()`)

---

### IMP-11: 시스템 프롬프트 튜닝 — 즉시 answer 조건 명시 🔲 v0.8.0
- **카테고리**: Xylem / Answer Agent
- **현상**: 현재 프롬프트가 "결과가 불충분하면 restore_l2" → LLM이 과도하게 보수적으로 판단. Q2처럼 search를 6회 반복하다 최대 반복 초과.
- **해결**: 프롬프트에 명시적 기준 추가:
  - "snippet + context + l2_summary로 질문의 핵심에 답할 수 있으면 즉시 answer 호출"
  - "restore는 구체적인 명령어/수치/설정값이 snippet에 없을 때만 사용"
  - "search를 2회 이상 반복하지 말 것. 결과가 없으면 not_found 호출"
- **예상 효과**: search 루프 방지, 불필요한 iteration 감소.
- **관련 파일**: `flows/xylem_flow/answer_agent.py` (`SYSTEM_PROMPT`)

---

## P2 — v0.9.0 (아키텍처 개선)

### IMP-12: Python 상주 gRPC 서비스 전환 🔲 v0.9.0
- **카테고리**: Xylem / Architecture
- **현상**: Go가 매 요청마다 Python subprocess를 spawn → Python 인터프리터 startup(~100-200ms) + 모듈 import(~200-400ms) + DB 커넥션 신규 생성(~100ms) 발생. connection pool 불가.
- **현재 구조**:
  ```
  HTTP req → Go spawn Python → 처리 → 프로세스 종료 (반복)
  ```
- **개선 구조**:
  ```
  HTTP req → Go gRPC call → 상주 Python xylem service (DB pool 유지)
  ```
  - `python/xylem_service/server.py` 신규: PostgreSQL connection pool, Qdrant client, embedding model 캐시 상주
  - `core/proto/xylem.proto` 신규: `XylemService { rpc Answer, Search, Restore }`
  - Go `runner.RunModule()` → `xylem_client.Answer()` 대체
  - Phloem gRPC 패턴 재사용 (이미 `python/nlp_worker/` 에 동일 구조 존재)
- **예상 효과**: 요청당 오버헤드 ~500ms → ~5ms. DB connection pool 재사용.
- **관련 파일**: `python/xylem_service/` (신규), `core/proto/xylem.proto` (신규), `internal/api/api.go`

---

### IMP-13: Query Rewriting — 한국어 구어체 → 기술 용어 변환 🔲 v0.9.0
- **카테고리**: Xylem / Retrieval
- **현상**: 한국어 구어체 쿼리("클러스터 구성이 어떻게 되어 있어?")와 영어/기술어 중심 문서 청크 간 임베딩 유사도 저하 → 관련 청크가 낮은 순위에 위치.
- **해결**: 임베딩 검색 전에 경량 LLM으로 쿼리를 기술 용어로 변환:
  ```
  "쿠버네티스 클러스터 구성이 어떻게 되어 있어?"
  → "Kubernetes cluster node configuration Master Worker CNI network"
  ```
  - `retriever.py`에 `_rewrite_query(query)` 추가 (ollama gemma4:2b 또는 동일 모델 소형화)
  - `GOPEDIA_QUERY_REWRITE=true|false` 환경변수로 on/off
- **예상 효과**: 첫 search 결과 품질 향상 → escalation 빈도 감소.
- **관련 파일**: `flows/xylem_flow/retriever.py`

---

## P3 — 중장기 (재인덱싱 필요)

### IMP-14: L2 summary Qdrant 인덱싱 (L2+L3 hybrid 검색) 🔲 미정
- **카테고리**: Phloem+Xylem / Embedding
- **현상**: 현재 Qdrant에 L3 청크 벡터만 존재. 섹션 레벨(L2) 질문에서 L3 청크가 부분 정보만 제공 → restore_l2 강제 호출.
- **해결**:
  - 인제스트 시 L2 summary도 Qdrant에 임베딩 (별도 named vector 또는 컬렉션)
  - `retrieve_and_enrich()`에서 L3 hit 외 L2 summary hit도 병행 검색
  - LLM에게 "이 섹션 요약이 충분하면 restore_l2 없이 answer 가능" 신호 전달
- **관련 파일**: `internal/phloem/sink/writer.go`, `flows/xylem_flow/retriever.py`

---

### IMP-15: Cross-Encoder Reranker 기본 활성화 🔲 미정
- **카테고리**: Xylem / Retrieval
- **현상**: Qdrant vector search로 상위 30개 후보 중 top-5 선택 시 의미적으로 부적절한 청크(디렉토리 목록, 헤더만 있는 청크)가 상위에 올 수 있음.
- **해결**: `--reranker` 플래그는 이미 CLI/retriever에 구현됨 (`BAAI/bge-reranker-v2-m3`). K8s 환경에서 기본 활성화:
  - `deploy/k8s/gopedia-svc.yaml`에 `GOPEDIA_RERANKER_ENABLED=true` 추가
  - `retrieve_and_enrich(use_reranker=True)` 기본값 변경
  - 초기 로딩 비용(모델 ~400MB)은 IMP-12(상주 서비스) 적용 시 1회만 발생
- **관련 파일**: `flows/xylem_flow/retriever.py`, `deploy/k8s/gopedia-svc.yaml`

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
| IMP-09 | **P1** | Xylem/Answer Agent | search 결과에 l2_summary + surrounding_context 포함, snippet 확장 | 🔲 v0.8.0 |
| IMP-10 | **P1** | Xylem/Answer Agent | dedup 기준 l1_id → l2_id 변경 | 🔲 v0.8.0 |
| IMP-11 | **P1** | Xylem/Answer Agent | 시스템 프롬프트 튜닝 — 즉시 answer 조건 명시 | 🔲 v0.8.0 |
| IMP-12 | P2 | Xylem/Architecture | Python 상주 gRPC 서비스 전환 (subprocess 제거) | 🔲 v0.9.0 |
| IMP-13 | P2 | Xylem/Retrieval | Query Rewriting — 한국어 구어체 → 기술 용어 변환 | 🔲 v0.9.0 |
| IMP-14 | P3 | Phloem+Xylem/Embedding | L2 summary Qdrant 인덱싱 (L2+L3 hybrid 검색) | 🔲 미정 |
| IMP-15 | P3 | Xylem/Retrieval | Cross-Encoder Reranker 기본 활성화 | 🔲 미정 |
