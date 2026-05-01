# Agent 동작 리포트 — v1.0 (초기 계층형 RAG)

| 항목 | 값 |
|------|----|
| **Agent 버전** | v1.0 (commit `d56ea19`, feat/answer-agent merged) |
| **테스트 일시** | 2026-05-01 |
| **대상** | neunexus 내부 지식 베이스 |
| **평가 방식** | 수동 쿼리 5건 — `GET /api/answer` 직접 호출 (port-forward) |
| **LLM** | `gemma4:26b` via Ollama (`OLLAMA_CHAT_URL=http://ollama-generate.ai-assistant.svc:11434`) |
| **max_iterations** | 8 |
| **snippet 크기** | 300자 (`matched_content` 기준) |
| **dedup 기준** | l1_id |
| **surrounding_context 전달** | ❌ (retrieve_and_enrich에서 fetch하나 answer_agent에서 미사용) |
| **l2_summary 전달** | ❌ (동일) |
| **Pipeline 버전** | mcp-2.1.0 (변경 없음) |
| **연관 Pipeline 리포트** | [`rag-test-reports/mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md`](../rag-test-reports/mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md) |

---

## 1. Agent 구성

### 아키텍처

```
GET /api/answer?q=<query>
  └─ Go: python subprocess spawn (매 요청 신규)
       └─ flows.xylem_flow.cli answer --query <q>
            └─ answer_agent.run(query, conn)
                 └─ LLM tool-calling loop (max 8회)
                      ├─ search(query, top_k=5)
                      │    └─ retrieve_and_enrich() → snippet[:300] 만 전달
                      ├─ restore_l2(l2_id) → 최대 3,000자
                      ├─ restore_l1(l1_id) → 최대 5,000자
                      ├─ answer(content, sources) → 종료
                      └─ not_found(reason) → 종료
```

### 시스템 프롬프트 전략 (v1.0)

- "반드시 tool을 호출해야 합니다. 직접 텍스트로 답하지 마세요."
- "결과가 불충분하면 restore_l2 → restore_l1 순으로 escalation"
- search 재시도 횟수 제한 없음 → Q2에서 search 6회 반복 후 초과

---

## 2. 쿼리별 테스트 결과

### Q0. "geneso가 무엇인지 설명해줘"

| 항목 | 값 |
|------|----|
| **결과** | ✅ found |
| **trace** | search → restore_l2 → restore_l2 → restore_l1 → answer |
| **iterations** | 5 |
| **추정 입력 토큰** | ~11,000 |

**분석:** GeneSo README 문서 존재. l3 snippet(300자)만으로 LLM이 "불충분" 판단 → l2 2회, l1 1회 restore 후 답변. 답변 품질은 우수 (역할, 관리방식, 원칙, 로드맵, 환경 포함).

---

### Q1. "쿠버네티스 클러스터 구성이 어떻게 되어 있어?"

| 항목 | 값 |
|------|----|
| **결과** | ✅ found |
| **trace** | search → restore_l2 → restore_l2 → search → restore_l1 → answer |
| **iterations** | 6 |
| **추정 입력 토큰** | ~13,000 |

**분석:** search를 2회 수행(쿼리 변경 재시도). l2 2회, l1 1회 restore. K8s v1.35.3, Master/Worker IP, Calico VXLAN, Traefik, Cinder CSI 등 상세 내용 포함 답변.

---

### Q2. "네트워크 설계는 어떻게 되어 있어? WireGuard VPN 포함해서 설명해줘"

| 항목 | 값 |
|------|----|
| **결과** | ❌ max_iterations 초과 |
| **trace** | search×6 → restore_l2 → search → (8회 초과) |
| **iterations** | 8 |
| **추정 입력 토큰** | ~16,000 (낭비) |

**분석:** `02 Network Design` 문서 존재하나 search 쿼리를 6회 반복하다 restore_l1에 도달 못하고 초과. **search 재시도 루프가 핵심 문제.** → IMP-11(프롬프트 search 횟수 제한) 적용 필요.

---

### Q3. "트러블슈팅 사례 알려줘"

| 항목 | 값 |
|------|----|
| **결과** | ✅ found |
| **trace** | search → restore_l2 → search → restore_l1 → answer |
| **iterations** | 5 |
| **추정 입력 토큰** | ~10,000 |

**분석:** restore_l1까지 도달 후 답변. NET-01, K8S-02, K8S-05, K8S-06 등 트러블슈팅 사례 포함.

---

### Q4. "브랜드 가이드라인이 뭐야? 로고나 색상 규정 있어?"

| 항목 | 값 |
|------|----|
| **결과** | ✅ found |
| **trace** | search → restore_l2 → answer |
| **iterations** | 3 |
| **추정 입력 토큰** | ~3,500 |

**분석:** 가장 효율적인 케이스. restore_l2 1회로 충분한 내용 확보. factual 단순 질문에서 l2만으로 답변 가능함을 확인.

---

### Q5. "AWS EC2 인스턴스 설정 방법 알려줘" (없는 정보)

| 항목 | 값 |
|------|----|
| **결과** | ❌ not_found (정상) |
| **trace** | search → search → not_found |
| **iterations** | 2 |
| **추정 입력 토큰** | ~800 |

**분석:** 없는 정보를 2회 search 후 정확히 포기. not_found 이유도 명시. 의도대로 동작.

---

## 3. 집계 지표

| 지표 | v1.0 값 | 목표 (v1.1) |
|------|---------|------------|
| **성공률** | 80% (4/5) | ≥ 90% |
| **avg iterations (성공 케이스)** | 4.25 | ≤ 2.5 |
| **escalation rate** | **100%** (4/4) | ≤ 50% |
| **search-only rate** | **0%** | ≥ 50% |
| **avg 입력 토큰 (성공)** | ~9,375 | ≤ 3,000 |
| **not_found 정확도** | 100% (1/1) | 100% 유지 |

### escalation 패턴 분류 (성공 케이스)

| 패턴 | 건수 | 비율 |
|------|------|------|
| search → answer | 0 | 0% |
| search → restore_l2 → answer | 1 (Q4) | 25% |
| search → restore_l2 × n → restore_l1 → answer | 3 (Q0,Q1,Q3) | 75% |

---

## 4. 문제 원인 분석

### 원인 A — snippet 300자 제한 (가장 큰 영향)

`_execute_search()`에서 `matched_content[:300]`만 전달. `retrieve_and_enrich()`가 이미 fetch한 `surrounding_context`(neighbor window), `l2_summary`를 버림 → LLM이 평가할 정보 부족 → 즉각 escalation.

### 원인 B — l1_id 기준 dedup

같은 문서의 서로 다른 섹션이 1개로 축소됨. 문서 내 분산 정보를 한 번의 search로 확보 불가 → restore_l1 강제.

### 원인 C — search 재시도 제한 없음

프롬프트에 "결과가 부족하면 다른 쿼리로 재검색" 허용 → Q2에서 search 6회 반복. iteration 낭비 후 max_iterations 초과.

---

## 5. 개선 항목 (v1.1 대상)

| ID | 내용 | 예상 효과 | 코드 변경 |
|----|------|---------|---------|
| **IMP-09** | search 결과에 l2_summary + surrounding_context 포함, snippet 300→500자 | escalation rate 100%→50% | `answer_agent._execute_search()` |
| **IMP-10** | dedup 기준 l1_id → l2_id | restore_l1 호출 감소 | `answer_agent._execute_search()` |
| **IMP-11** | 프롬프트: search 최대 2회 제한, 즉시 answer 조건 명시 | Q2류 max_iterations 방지 | `answer_agent.SYSTEM_PROMPT` |

상세: [`doc/IMPROVEMENTS.md`](../IMPROVEMENTS.md) IMP-09~11 참조.
