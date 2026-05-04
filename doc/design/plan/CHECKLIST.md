# 미완성 항목 체크리스트

> 최종 업데이트: 2026-05-04

---

## 완료 항목

| # | 항목 | PR | 상태 |
|---|------|-----|------|
| TODO-1 | `tree.py` 복원 (L1 트리 조회) | #33 | ✅ 병합 완료 |
| TODO-2 | `POST /api/index/reset` | #34 | ✅ 병합 완료 |
| IMP-13 | Query Rewriting (GOPEDIA_QUERY_REWRITE) | #35 | ✅ 병합 완료 |
| IMP-15 | Reranker env-based default | #36 | ✅ 병합 완료 |
| GraphDB Phase 1-4 | TypeDB 스키마·K8s·sync·graph_context·retriever 통합 | #38 | ✅ 병합 완료 |
| P@3 Qrel 확장 | sample_osteon_guide_30_v3 (45 qrels), P@3 0.333→0.389 | #37 | ✅ 병합 완료 |
| docs(rag): IR 지표 README 업데이트 | v0.8.0·v0.8.1 차트 반영 | #39 | ✅ 병합 완료 |
| docs(rag): v0.9.0 측정 결과 기록 | TypeDB graph_context 활성, Recall@5 +0.017 | #40 | ✅ 병합 완료 |

---

## 진행 예정 — TypeDB graph_context 튜닝

> **배경**: v0.9.0 측정 결과 Recall@5 +0.017(✅) / P@3 -0.022(⚠️).
> graph expansion 결과가 top-3 슬롯을 경쟁해 secondary qrel 탈락.
> 아래 두 항목으로 P@3 하락 완화 가능성 검토.

| # | 항목 | 핵심 파일 | 상태 |
|---|------|----------|------|
| GC-1 | `graph_context.py` — 반환 l1_id 수 상한 추가 (`max_siblings` / `GRAPH_MAX_SIBLINGS`) | `flows/xylem_flow/graph_context.py` | ✅ 완료 |
| GC-2 | `retriever.py` — graph expansion 결과 수 상한 (`max_graph_results` / `GRAPH_MAX_RESULTS`) | `flows/xylem_flow/retriever.py` | ✅ 완료 |
| GC-3 | gardener_gopedia 재측정 — GC-1/2 적용 후 v0.9.0 대비 P@3·Recall@5 비교 | — | ✅ 완료 (v0.10.0, run_id: 86c9d663) |

---

## GC 튜닝 결론

> `GRAPH_MAX_RESULTS=3` / `GRAPH_MAX_SIBLINGS=3` 운영 환경 고정 권장.
> P@3 = 0.367은 구조적 한계 — graph 결과 수가 아닌 삽입 위치 문제. gardener top-k 상세 API 없이는 개선 불가.
> Recall@5 0.900 / MRR@10 0.950 / nDCG@10 0.890 안정 유지.

---

## 진행 예정 — P@3 개선 로드맵

> **현황 (v0.10.0)**: P@3 = 0.367 (k=3, secondary 15개 중 3개만 top-3 내 검색)
> **구조적 원인**: cross-encoder 재랭킹 후 secondary가 rank 4-5로 밀림. graph 결과 수 제한(GC-1/2)으로 해결 불가.
> **상세 분석**: `doc/design/plan/TODO.md` — P@3 개선 로드맵 섹션 참고.

| # | 항목 | 접근 | 관련 저장소/파일 | re-ingest | 상태 |
|---|------|------|----------------|-----------|------|
| P3-A | gardener top-k 상세 API — `GET /runs/{id}/queries` 엔드포인트 추가 | 근본 진단 수단 | `gardener_gopedia` 저장소 | 불필요 | ✅ 구현 완료 (feat/p3a-topk-detail-api, PR 예정) |
| P3-B | Cross-Encoder reranker 활성화 — `BAAI/bge-reranker-v2-m3` | 직접 원인 해결 시도 | `gopedia-svc.yaml` | 불필요 | ✅ 완료 (v0.11.0) — P@3 불변, 레이턴시 -512ms. secondary top-k 부재 확인 |
| P3-C | 임베딩 모델 검토 — 현재 BGE-M3 이미 최상급 | 간접 효과 | `embedder/openai.go` | **필요** | ⚠️ 낮은 우선순위 |
| P3-D | 하이브리드 검색 — Qdrant sparse(BM25) + dense | 간접 효과 | `retriever.py`, ingest 파이프라인 | **필요** | 🔲 P3-A 결과 확인 후 판단 |

> **P3-A·B 결론**: secondary qrel이 top-k 후보 자체에 없음 확인. 재랭킹으로는 해결 불가.
> **다음**: P3-A로 수집한 hit rank 데이터로 IMP-17(청킹) vs P3-D(하이브리드) 방향 결정.

---

## 진행 예정 — 검색 품질 개선 후보 (메트릭 회귀 시)

> **진단 조건**: gardener eval Recall@5 < 0.85 또는 MRR@10 < 0.90 시 순서대로 적용.
> 한 번에 하나만 수정 후 재측정.

| # | 항목 | 현황 → 후보 | 관련 파일 | 상태 |
|---|------|------------|----------|------|
| IMP-16 | 임베딩 모델 비교·검토 | 현재 **BGE-M3** (로컬 Ollama) — 이미 최상급. 교체 효과 낮음. 필요 시 재평가. | `embedder/openai.go`, `gopedia-svc.yaml` | ⚠️ 낮은 우선순위 |
| IMP-17 | 청킹 전략 개선 | heading-based → semantic / heading+fixed-size 하이브리드 | `internal/phloem/chunker/` | 🔲 대기 |
| IMP-18 | 하이브리드 검색 | 벡터 only → 벡터 + 키워드 (Qdrant sparse) | `retriever.py::retrieve_and_enrich()` | 🔲 대기 (P3-D와 동일) |

---

## 보류 항목

| # | 항목 | 이유 |
|---|------|------|
| IMP-12 | Cross-Encoder → Python 상주 gRPC 서비스 | proto 설계 + 대규모 리팩터 필요 |
| IMP-14 | L2 Qdrant 인덱싱 | 전체 re-ingest 필요 |
