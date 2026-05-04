# TODO

---

## GraphDB (TypeDB) RAG 강화 — 4단계 구현

> **선행 조건**: gardener_gopedia로 현재 파이프라인 품질 베이스라인 측정 및 기록 후 작업 시작.
> 측정: Telegram `"gopedia 품질 테스트 해줘"` → run_id 기록 → `doc/rag-test-reports/` 저장

### Phase 1 — TypeDB 스키마 확장 + K8s 활성화 ✅ 완료 (PR #38)

- [x] `core/ontology_so/typedb_schema.typeql` 수정
- [x] `deploy/k8s/typedb.yaml` 신규 (TypeDB StatefulSet + PVC 20Gi + ClusterIP/Headless Service)
- [x] `deploy/k8s/gopedia-svc.yaml` TypeDB env 주석 해제

### Phase 2 — Ingest-time TypeDB 동기화 확장 ✅ 완료 (PR #38)

- [x] `core/ontology_so/typedb_sync.py` 수정
- [x] `property/root_props/run.py`: ingest 완료 후 `sync_directory_tree_to_typedb()` 호출
- [x] `core/ontology_so/postgres_ddl.sql`: `knowledge_l1.typedb_synced_at TIMESTAMP` 추가

### Phase 3 — graph_context.py 신규 모듈

- [ ] `flows/xylem_flow/graph_context.py` 신규 생성
  - `get_related_l1_ids(hit_l1_ids, project_id, depth=1) -> list[str]`
  - TypeDB `contains` 탐색: hit file → parent directory → sibling files l1_id 반환
  - TypeDB 미연결 시 빈 리스트 반환 (graceful degradation 필수)

### Phase 4 — retriever.py 통합

### Phase 3 — graph_context.py 신규 모듈 ✅ 완료 (PR #38)

- [x] `flows/xylem_flow/graph_context.py` 신규 생성
  - `get_related_l1_ids(hit_l1_ids, project_id, depth=1) → list[str]`
  - TypeDB `contains` 탐색: hit file → parent directory → sibling files l1_id 반환
  - TYPEDB_HOST 미설정 시 빈 리스트 반환 (graceful degradation)

### Phase 4 — retriever.py 통합 ✅ 완료 (PR #38)

- [x] `flows/xylem_flow/retriever.py` 수정
  - `retrieve_and_enrich()` 내 graph expansion 블록 삽입
  - `TYPEDB_HOST` 설정 시에만 활성화 (zero-cost skip)
  - `use_graph_context: Optional[bool] = None` 파라미터 추가
  - `source: "graph_expansion"` 필드로 graph 기원 결과 구분
- [x] gardener_gopedia로 TypeDB K8s 배포 후 재측정 → 베이스라인 대비 Recall@5 비교
  - run_id: `991f499d-dd52-45ba-8ee0-86e226acb620` (2026-05-04)
  - TypeDB 3.8.2, 167 L1 synced (file 167 / section 2,067 / chunk 6,786)
  - Recall@5: 0.883 → **0.900** (+0.017 ✅), MRR@10 유지, P@3 소폭 하락(-0.022)
  - 상세: `doc/rag-test-reports/v0.9.0_2026-05-04_graphdb-active.md`

---

## tree.py — 프로젝트 지식 트리 조회 모듈 ✅ 완료 (PR #33)

`flows/xylem_flow/tree.py` 복원 완료.
- `fetch_project_l1_nodes`, `build_project_l1_tree`, `get_project_tree_for_viewer`
- GraphDB Phase 2에서 `sync_directory_tree_to_typedb()` 입력으로 활용 예정

---

## 인덱스 초기화 API ✅ 완료 (PR #34)

`POST /api/index/reset` 구현 완료.
- `flows/xylem_flow/index_reset.py`: FK 순서 준수 삭제 + Qdrant points 삭제
- dry-run, project_id 단위 부분 삭제 지원

---

## P@3 개선 — Qrel 확장

> **배경**: v0.8.0 베이스라인 측정 결과 P@3 = 0.333 (run_id: `5c0ddfd0-612f-4118-a993-294a1154e47e`).
> 이 값은 osteon 데이터셋이 **쿼리당 qrel 1개**이기 때문에 발생하는 구조적 상한값이며,
> 파이프라인 수정으로는 개선 불가. qrel 자체를 확장해야 함.

### 진행 현황 ✅ 완료

- [x] `dataset/sample_osteon_guide_30_v3.json` 생성 — secondary qrel 20개 추가 (총 50 qrels)
- [x] gardener_gopedia 재평가 실행 — run_id: `8bf4d5d2-1352-41ce-8f1e-1aac8f6f843a`
  - **P@3: 0.333 → 0.389** (+0.056), MRR@10=0.950 유지
  - secondary 20개 중 13개 top-5 내 검색, 5개 top-3 내 검색
- [x] Gemini code review 반영 — 부정확한 secondary 5개 제거, 45 qrels로 확정 (PR #25)
- [x] `doc/rag-test-reports/v0.8.1_2026-05-04_qrel-expansion-v3.md` 리포트 저장

### P@3 목표 조정 및 근거

**P@3 목표: 0.333 → 0.389 달성 ✅** (v0.8.0 대비 +0.056)

P@3 ≥ 0.60 목표는 qrel 벡터 검색 기반 확장으로는 달성 불가 — 구조적 한계 확인:

```
현재 파이프라인 특성:
  - Primary qrel은 항상 rank-1 검색됨 (MRR@10=0.950)
  - Secondary는 cross-encoder 재랭킹 후 rank 4-5로 밀림
  - 벡터 유사도 기반 rank-1/2 예측 → 실제 gardener 재랭킹 결과와 불일치

P@3 달성 상한:
  P@3 = (30 + k) / 90  (k = secondary top-3 적중 수)
  현재 k=3 → P@3=0.367 (코드리뷰 후 45 qrels 기준)
  현재 k=5 → P@3=0.389 (원본 v3 50 qrels 기준, 최고치)
  k=24 필요 → P@3=0.600 (달성 불가 — gardener top-3 실측 없이)
```

**P@3 개선 다음 조건 (향후 작업 시):**
- gardener에 top-k 상세 API 추가 → 실제 rank-2/3 l3_id 확보
- 또는 임베딩 모델 업그레이드 / 하이브리드 검색 도입 후 재측정

---

## P@3 개선 로드맵

> **현황 (v0.10.0 기준)**
>
> ```
> P@3 = 0.367 = 33/90
>   - primary 30개 × 1/3 기여 = 30/90  (primary는 항상 top-3 내 검색)
>   - secondary 15개 × k/15 기여 = 3/90  (현재 k=3, top-3 내 secondary 3개)
>
> 목표별 필요 k:
>   P@3 = 0.389 → k=5   (qrel 코드리뷰 전 50 qrels 기준 최고치)
>   P@3 = 0.450 → k=10
>   P@3 = 0.500 → k=15  (secondary 전부 top-3 — 이론 상한)
> ```
>
> **v0.10.0 확인 사항**: `GRAPH_MAX_RESULTS=3`으로 graph 결과 수 제한해도 P@3 불변.
> → P@3 하락은 결과 수 문제가 아니라 **cross-encoder 재랭킹 시 secondary가 rank 4-5로 밀리는 구조** 문제.

### 접근 A — gardener top-k 상세 API 추가 ⭐ (근본 해결, 외부 저장소 변경)

**효과**: 실제 rank-1/2/3 l3_id를 확보 → secondary qrel 정확도 재검증 가능 → k를 실질적으로 올릴 수 있음.

| 항목 | 내용 |
|------|------|
| 변경 대상 | `gardener_gopedia` 저장소 — `/runs/{id}/queries` 또는 `/runs/{id}/details` 엔드포인트에 `top_k_hits: [l3_id, ...]` 추가 |
| gopedia 측 변경 | gardener_run_report MCP 툴 or gardener API 호출 스크립트에서 top-3 l3_id 수집 → qrel과 대조 |
| 기대 효과 | 어떤 secondary qrel이 실제로 rank 4-5에 있는지 파악 → 해당 l3_id를 qrel에 추가하거나 청킹·모델 전략 근거 자료로 활용 |
| 선행 조건 | gardener_gopedia 저장소 접근 및 API 확장 권한 |

### 접근 B — Cross-Encoder 모델 교체·튜닝 (직접 원인 해결)

**효과**: 재랭킹 단계에서 secondary qrel이 rank 1-3으로 올라올 가능성 직접 상승.

| 항목 | 내용 |
|------|------|
| 현황 | `ms-marco-MiniLM-L-6-v2` (또는 동급) — osteon 의학 도메인에 최적화 안 됨 |
| 후보 1 | `cross-encoder/ms-marco-MiniLM-L-12-v2` — 동일 계열 대형 버전, 재랭킹 정확도 향상 |
| 후보 2 | 도메인 특화 fine-tuning — osteon 데이터로 cross-encoder 직접 학습 (리소스 소요 큼) |
| 관련 파일 | `flows/xylem_flow/retriever.py` (reranker 호출부), `deploy/k8s/gopedia-svc.yaml` (모델 env) |
| 측정 방법 | gardener 재측정 → P@3·nDCG@10 비교 |
| 주의 | re-ingest 불필요, 모델 교체만으로 즉시 효과 확인 가능 |

### 접근 C — 임베딩 모델 교체·비교 (간접 효과)

**효과**: 초기 Qdrant 벡터 검색 단계에서 secondary qrel score 상승 → cross-encoder에 더 좋은 후보 제공.

> **현황 정정 (2026-05-04 확인)**:
> 코드 기본값은 `text-embedding-3-small`이나, 운영 환경은 `OPENAI_BASE_URL=http://ollama-embed.ai-assistant.svc:11434/v1` + `OPENAI_EMBEDDING_MODEL=bge-m3`으로 **BGE-M3 로컬 모델**을 사용 중.

#### 현재 모델: BGE-M3

| 항목 | 값 |
|------|-----|
| 모델 | `BAAI/bge-m3` |
| 서빙 | Ollama (`ollama-embed.ai-assistant.svc:11434`, OpenAI 호환 API) |
| 차원 | 1024-dim |
| 컨텍스트 | 최대 8,192 토큰 |
| 특성 | 다국어 dense + sparse + ColBERT 통합. 오픈소스 최상급 다국어 모델. 한국어 우수. |
| 비용 | 무료 (로컬 서빙) |

#### 후보 모델 비교

| 모델 | 차원 | 비용 | 특성 | P@3 기대 효과 |
|------|------|------|------|--------------|
| **BGE-M3** (현재) | 1024 | 무료 | 다국어·다목적·dense+sparse+colbert | 기준 |
| `text-embedding-3-small` | 1536 | 유료($0.02/1M) | OpenAI 영어 강세, 한국어 보통 | 현재보다 낮을 가능성 |
| `text-embedding-3-large` | 3072 | 유료($0.13/1M) | OpenAI 고정밀, 영어 강세 | 한국어 쿼리엔 BGE-M3 대비 불확실 |
| `bge-large-zh-v1.5` | 1024 | 무료 | 중국어 특화 | 한국어 쿼리엔 부적합 |
| `multilingual-e5-large` | 1024 | 무료 | 다국어, BGE-M3보다 구형 | BGE-M3 대비 약세 예상 |

> **결론**: BGE-M3는 이미 최상급 오픈소스 다국어 모델. 임베딩 모델 교체로 P@3 개선 여지 낮음.
> 대신 **Cross-Encoder 교체(접근 B)**가 더 직접적인 효과 기대.

| 관련 파일 | `internal/phloem/embedder/openai.go`, `flows/xylem_flow/retriever.py`, `deploy/k8s/gopedia-svc.yaml` |
|-----------|------|
| 주의 | **전체 re-ingest 필요** (벡터 공간 변경). `POST /api/index/reset` 후 재인제스트. |

### 접근 D — 하이브리드 검색 도입 (간접 효과)

**효과**: 벡터 유사도 + 키워드 매칭 결합 → secondary qrel이 벡터로 놓쳐도 키워드로 캐치.

| 항목 | 내용 |
|------|------|
| 현황 | Qdrant 벡터 검색 only |
| 후보 | Qdrant sparse vector(BM25) + dense vector 결합 (Qdrant 네이티브 지원) |
| 관련 파일 | `flows/xylem_flow/retriever.py::retrieve_and_enrich()`, ingest 시 sparse vector 추가 |
| 주의 | ingest 파이프라인 변경 필요 (sparse 임베딩 추가), 전체 re-ingest 필요 |

### 접근 우선순위 및 의존성

```
[지금 바로 가능]
  B (Cross-Encoder 교체) → re-ingest 불필요, 즉시 gardener 재측정 가능
  A (gardener API 확장) → 외부 저장소 작업이지만 근본 진단 수단

[re-ingest 필요, 나중에]
  C (임베딩 모델 업그레이드) → B 효과 확인 후 병행
  D (하이브리드 검색) → C와 동시 적용 가능하나 ingest 변경 범위 큼
```

> **권장 순서**: A(gardener API) → B(cross-encoder) → C(임베딩) → D(하이브리드)
> A 없이 B부터 해도 P@3 효과 확인 가능.

---

## 검색 품질 개선 후보 (평가 메트릭 기반)

**배경**

gardener_gopedia 평가 파이프라인으로 universitas/ 문서 인제스트 후 Recall@5, MRR@10,
nDCG@10, P@3 메트릭 측정. 현재 메트릭은 기준치 충족 (T14 SKIPPED)이나,
향후 문서 규모 확대 또는 도메인 변경 시 재진단 필요. 아래 항목은 병목 발생 시 개선 후보.

**개선 후보 목록**

### 1. 청킹 전략
- 현재: heading-based chunking (`internal/phloem/chunker/`)
- 후보: semantic chunking 또는 heading + fixed-size 하이브리드
- 증상: 특정 쿼리에서 관련 내용이 청크 경계에서 잘려 hit 실패

### 2. 임베딩 모델
- 현재: `text-embedding-3-small` (OpenAI)
- 후보: `text-embedding-3-large` 또는 multilingual-e5-large (로컬)
- 관련 파일: `internal/phloem/embedder/openai.go`, `flows/xylem_flow/retriever.py`

### 3. 검색 파라미터 및 랭킹
- 현재: Qdrant 벡터 검색 + neighbor_window 기반 컨텍스트 확장
- 후보: 하이브리드 검색 (벡터 + 키워드), 리랭킹 레이어 추가
- 관련 파일: `flows/xylem_flow/retriever.py::retrieve_and_enrich()`

### 4. Cross-Encoder 로딩/실행 구조 최적화 (IMP-12 선행 필요)
- 현재: 요청마다 subprocess spawn → Cross-Encoder 모델 반복 로드
- 후보: Python 상주 gRPC 서비스로 분리 (IMP-12)
- 기대 효과: warm process 재사용으로 리랭킹 레이턴시 감소

**진단 방법**

- gardener_gopedia eval로 Recall@5 < 0.5 이면 개선 필요
- per-query 분석으로 실패 패턴 파악 후 위 후보 중 택일
- 한 번에 하나만 수정하고 재평가
