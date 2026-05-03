# 미완성 항목 체크리스트

> 최종 업데이트: 2026-05-03

---

## 완료 항목

| # | 항목 | PR | 상태 |
|---|------|-----|------|
| TODO-1 | `tree.py` 복원 (L1 트리 조회) | #33 | ✅ 병합 완료 |
| TODO-2 | `POST /api/index/reset` | #34 | ✅ 병합 완료 |
| IMP-13 | Query Rewriting (GOPEDIA_QUERY_REWRITE) | #35 | ✅ 병합 완료 |
| IMP-15 | Reranker env-based default | #36 | ✅ 병합 완료 |

---

## 진행 예정 — GraphDB RAG 강화

> **선행 조건**: gardener_gopedia 품질 테스트로 베이스라인 측정 후 시작
> Telegram `"gopedia 품질 테스트 해줘"` → run_id 기록 → `doc/rag-test-reports/` 저장

| Phase | 항목 | 핵심 파일 | 상태 |
|-------|------|----------|------|
| 1 | TypeDB 스키마 확장 + K8s 활성화 | `typedb_schema.typeql`, `typedb.yaml` | 🔲 대기 |
| 2 | Ingest-time 동기화 (`sync_directory_tree`) | `typedb_sync.py`, `run.py` | 🔲 대기 |
| 3 | `graph_context.py` 신규 모듈 | `graph_context.py` | 🔲 대기 |
| 4 | `retriever.py` graph expansion 통합 | `retriever.py` | 🔲 대기 |

상세 스펙 → `doc/design/plan/TODO.md` GraphDB 섹션 참고

---

## 보류 항목

| # | 항목 | 이유 |
|---|------|------|
| IMP-12 | Python 상주 gRPC 서비스 | proto 설계 + 대규모 리팩터 |
| IMP-14 | L2 Qdrant 인덱싱 | 전체 re-ingest 필요 |
