# Agent Reports

Answer Agent(계층형 RAG 에이전트) 동작 품질을 버전별로 기록합니다.

> **파이프라인 품질(Phloem+Xylem IR 지표)** 은 [`doc/rag-test-reports/`](../rag-test-reports/README.md) 참조.

## 측정 대상

| 레이어 | 변경 요소 | 지표 |
|--------|---------|------|
| **Agent** | LLM 모델, 시스템 프롬프트, tool 정의, max_iterations | 성공률, iterations/query, 입력 토큰/query, 답변 품질(수동) |
| **Agent ↔ Pipeline 경계** | snippet 크기, dedup 기준, surrounding_context 전달 | escalation 비율(restore_l2/l1 호출 빈도) |

## 파일 명명 규칙

```
agent-<version>_<date>_<target>.md
```

예: `agent-v1.0_2026-05-01_neunexus.md`

**버전 정책:**
- Agent 버전은 Pipeline 버전과 독립적으로 관리
- `answer_agent.py`, `SYSTEM_PROMPT`, tool 정의, LLM 모델 변경 시 증가
- Pipeline(Phloem/Xylem) 변경만 있는 경우 Pipeline 버전만 올림

## 주요 지표 정의

| 지표 | 의미 |
|------|------|
| **성공률** | `found=true` 응답 비율 (not_found / max_iterations 초과 제외) |
| **avg iterations** | 성공 케이스의 평균 LLM tool-calling 횟수 |
| **escalation rate** | restore_l2 또는 restore_l1이 호출된 비율 |
| **search-only rate** | search → answer (restore 없이 종료)된 비율 — **핵심 효율 지표** |
| **avg input tokens** | 추정 LLM 입력 토큰 (성공 케이스 평균) |

---

## 버전별 지표 현황

| 버전 | 날짜 | LLM | 성공률 | avg iter | escalation rate | search-only rate | 파일 |
|------|------|-----|-------|---------|----------------|-----------------|------|
| v1.0 | 2026-05-01 | gemma4:26b | 80% (4/5) | 4.2 | 100% | **0%** | [agent-v1.0_2026-05-01_neunexus.md](agent-v1.0_2026-05-01_neunexus.md) |

> **목표 (v1.1 이후):** search-only rate ≥ 50%, avg iter ≤ 2.5, avg input tokens ≤ 3,000

---

## 리포트 목록

| 버전 | 날짜 | 대상 | 파일 |
|------|------|------|------|
| agent-v1.0 | 2026-05-01 | neunexus 내부 지식 베이스, 수동 쿼리 5건 | [agent-v1.0_2026-05-01_neunexus.md](agent-v1.0_2026-05-01_neunexus.md) |
