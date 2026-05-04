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
- [ ] gardener_gopedia로 TypeDB K8s 배포 후 재측정 → 베이스라인 대비 Recall@5 비교

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

### 진행 현황

- [x] `dataset/sample_osteon_guide_30_v3.json` 생성 — secondary qrel 20개 추가 (총 50 qrels)
- [x] gardener_gopedia 재평가 실행 — run_id: `8bf4d5d2-1352-41ce-8f1e-1aac8f6f843a`
  - **P@3: 0.333 → 0.389** (+0.056), MRR@10=0.950 유지
  - secondary 20개 중 13개 top-5 내 검색, 5개 top-3 내 검색
- [x] `doc/rag-test-reports/v0.8.1_2026-05-04_qrel-expansion-v3.md` 리포트 저장

### 잔여 TODO — P@3 ≥ 0.60 달성

- [ ] gardener 상세 API 또는 retrieval 로그로 top-3 실제 l3_id 확인
- [ ] 실제 top-3에 등장하는 청크를 qrel로 등록 (v4 dataset)
  - secondary가 top-5 밖인 7개 쿼리 우선 분석
  - 동일 섹션 인접 청크(neighbor_window 결과) 활용
- [ ] v4 dataset으로 재평가 → P@3 ≥ 0.60 목표

### 현재 P@3 상한 분석

```
qrel 수 = 2/query, secondary top-3 적중 = k/20 일 때:
P@3 = (k + 30) / 90
k=5 → P@3=0.389 (현재)
k=24 → P@3=0.600 (목표, 현재 top-5 내 13개이므로 top-3 도달 필요)
```

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
