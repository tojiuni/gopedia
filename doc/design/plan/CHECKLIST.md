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
| GC-3 | gardener_gopedia 재측정 — GC-1/2 적용 후 v0.9.0 대비 P@3·Recall@5 비교 | — | 🔲 재측정 필요 |

---

## 진행 예정 — 검색 품질 개선 후보

> **진단 조건**: gardener eval Recall@5 < 0.5 또는 MRR@10 < 0.9 시 순서대로 적용.
> 한 번에 하나만 수정 후 재측정.

| # | 항목 | 현황 → 후보 | 관련 파일 | 상태 |
|---|------|------------|----------|------|
| IMP-16 | 임베딩 모델 업그레이드 | `text-embedding-3-small` → `text-embedding-3-large` | `internal/phloem/embedder/openai.go`, `retriever.py` | 🔲 대기 |
| IMP-17 | 청킹 전략 개선 | heading-based → semantic / heading+fixed-size 하이브리드 | `internal/phloem/chunker/` | 🔲 대기 |
| IMP-18 | 하이브리드 검색 | 벡터 only → 벡터 + 키워드 (Qdrant sparse) | `retriever.py::retrieve_and_enrich()` | 🔲 대기 |

---

## 보류 항목

| # | 항목 | 이유 |
|---|------|------|
| IMP-12 | Cross-Encoder → Python 상주 gRPC 서비스 | proto 설계 + 대규모 리팩터 필요 |
| IMP-14 | L2 Qdrant 인덱싱 | 전체 re-ingest 필요 |
| P@3 구조적 개선 | gardener top-k 상세 API 추가 전까지 rank-2/3 실측 불가 (k=24 필요, 현재 k=3) |
