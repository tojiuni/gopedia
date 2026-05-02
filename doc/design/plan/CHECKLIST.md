# 미완성 항목 체크리스트

> 생성일: 2026-05-02  
> 기준: `doc/IMPROVEMENTS.md` + `doc/design/plan/TODO.md`

---

## P2 — 다음 릴리즈 대상

| # | 항목 | 브랜치 | 상태 |
|---|------|--------|------|
| IMP-13 | Query Rewriting — 한국어 구어체 → 기술 용어 변환 | `wt/imp-13` | 🔲 진행 중 |
| IMP-12 | Python 상주 gRPC 서비스 전환 (subprocess 제거) | — | ⏸ 보류 (대규모 아키텍처) |

## P3 — 중장기

| # | 항목 | 브랜치 | 상태 |
|---|------|--------|------|
| IMP-15 | Cross-Encoder Reranker 기본 활성화 | `wt/imp-15` | 🔲 진행 중 |
| IMP-14 | L2 summary Qdrant 인덱싱 (hybrid 검색) | — | ⏸ 보류 (재인덱싱 필요) |

## TODO — 기능 복원 / 신규

| # | 항목 | 브랜치 | 상태 |
|---|------|--------|------|
| TODO-1 | `flows/xylem_flow/tree.py` 복원 (프로젝트 L1 트리 조회) | `wt/feat-tree` | 🔲 진행 중 |
| TODO-2 | 인덱스 초기화 API `POST /api/index/reset` | `wt/feat-index-reset` | 🔲 진행 중 |

## 보류 항목 (별도 스프린트)

| # | 항목 | 이유 |
|---|------|------|
| IMP-12 | Python 상주 gRPC 서비스 | proto 설계 + 대규모 리팩터 필요 |
| IMP-14 | L2 Qdrant 인덱싱 | 전체 re-ingest 필요 |

---

## 병합 순서

1. `wt/feat-tree` (독립)
2. `wt/feat-index-reset` (독립)
3. `wt/imp-13` (retriever.py)
4. `wt/imp-15` (retriever.py 충돌 → IMP-13 머지 후 rebase)
