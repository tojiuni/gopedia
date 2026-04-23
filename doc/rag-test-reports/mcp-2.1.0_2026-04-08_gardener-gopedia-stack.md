# RAG 품질·연동 테스트 리포트 — Gardener + Gopedia + gopedia_mcp (스택)

| 항목 | 값 |
|------|----|
| **Gopedia (API)** | branch `docs/design-phloem-xylem-pipelines`, commit `6fd6a7bccf2ce36827b6f37c69fe250cb0db9656` (short: `6fd6a7bc`) |
| **gardener_gopedia** | branch `feature/eval-osteon-preset-dotenv-datasets`, commit `22f71eb64b523dc9863265fcd8302a4f8c106e22` (short: `22f71eb`), PyPI name `gardener-gopedia` **0.1.0** |
| **gopedia_mcp** | branch `feature/restore-first-v2`, commit `b2ccbd3d94d99036630758c5f5758c9c8ec6e35b` (short: `b2ccbd3`); npm `gopedia-mcp-server` **1.0.0**; MCP 서버 `version` **2.1.0** |
| **테스트 정리일** | 2026-04-08 (개별 측정 시각은 아래 절별 참고) |
| **Gopedia API** | `http://127.0.0.1:18787` |
| **Gardener API** | `http://127.0.0.1:18880` |

**범위**: 동일 로컬 스택에서 (1) MCP `npm run test:mcp` 스모크, (2) `gardener_quality_run`의 `quality_preset: "osteon"` 품질 러(번들 `sample_osteon_guide_30` 계열) 결과를 한데 묶어 기록한다. 인제스트/Universitas IR 리포트(`v0.6.0` 등)와 **데이터셋·질의 구성이 다르므로** 집계 점수를 직접 비교하는 용도는 아니다.

**원시 로그 (MCP 스모크)**: `gopedia_mcp` 워크스페이스 기준 `data/logs/mcp-smoke-test-2026-04-07T22-11-30-380Z.json` (아래 1절과 대응).

---

## 인제스트 데이터 규모 (Gopedia Postgres, Rhizome)

테스트 당시 **동일 DB** 기준으로 `documents` / `knowledge_l1`–`l3` / `projects` 행 수를 집계했다. 연결은 Gopedia 저장소 [`.env`](../../.env)의 `POSTGRES_*`를 따르되, 호스트에서 직접 조회할 때는 `POSTGRES_HOST=127.0.0.1`, `POSTGRES_PORT=5432`, `POSTGRES_DB=gopedia` (스키마: `core/ontology_so/postgres_ddl.sql`).

| 항목 | 건수 | 비고 |
|------|------|------|
| **projects** | 1 | 프로젝트 |
| **documents** | **9** | 인제스트된 **논리 문서(루트)** 행 수 — “문서 개수”로 쓰기 가장 직접적인 지표 |
| **knowledge_l1** | 9 | L1 (문서 트리 루트) |
| **knowledge_l2** | 677 | L2 (섹션 등) |
| **knowledge_l3** | **1,757** | L3 (검색·벡터에 가까운 **청크** 단위) |
| **조회일** | 2026-04-08 | 위 스냅샷이 이후 인제스트로 변할 수 있음 |

Qdrant 벡터 점 수는 본 리포트에 미포함; 컬렉션명은 `.env`의 `QDRANT_COLLECTION` / `QDRANT_DOC_COLLECTION`을 참고해 별도 조회할 것.

---

## 1. gopedia_mcp 스모크 (`test:mcp`)

| 항목 | 값 |
|------|----|
| **실행 시각 (UTC)** | 2026-04-07T22:11:22.532Z — 2026-04-07T22:11:30.380Z |
| **최상위 `success`** | `true` |
| **대상 `targetApiBase`** | `http://127.0.0.1:18787` |

### 1-1. 노출된 MCP 도구 (`tools/list`)

`gopedia_health`, `gopedia_search`, `gopedia_restore`, `gopedia_ingest`, `gardener_health`, `gardener_quality_run`, `gardener_run_report`

### 1-2. 기본 점검

- **`gopedia_health`**: `ok` — `phloem` / `postgres` / `qdrant` / `typedb` 모두 `ok` (로그에 기록된 응답 기준).
- **단발 `gopedia_search` (probe)**: `ok` — `results` 2건 이상 반환 (Kubernetes 관련 L3, score ~0.86).

### 1-3. 시나리오 검색 6건 (`gopedia_search` S1–S6)

| ID | 난이도 | `detail` | 검색 API 관측 (본 실행) |
|----|--------|----------|-------------------------|
| S1 | simple | summary | `PYTHON_SEARCH_FAILED` — `No Qdrant hits or empty context` |
| S2 | simple | summary | **히트** (neunexus 관련 스니펫) |
| S3 | intermediate | standard | `PYTHON_SEARCH_FAILED` — 동일 |
| S4 | intermediate | standard | `PYTHON_SEARCH_FAILED` — 동일 |
| S5 | advanced | full | **히트** (full 상세 필드 포함) |
| S6 | advanced | full | **히트** |

> **요약**: `summary` / `standard` 경로에서 일부 질의는 “빈 컨텍스트”로 실패했고, `full` 및 일부 `summary` 질의는 정상 히트했다. 품질 리포트의 **osteon 집행 평가**(2절)는 Gardener·Gopedia eval 경로이며, 본 시나리오는 MCP→검색 파이프라인 스모크 성격이 강하다.

### 1-4. Gardener 쪽 MCP 직접 호출

본 JSON에는 `gardener_health` / `gardener_quality_run` **실행 스텝이 없고**, 도구 **등록 여부**만 기록되어 있다. 품질 수치는 2절을 참고한다.

---

## 2. Gardener `quality_preset: "osteon"` 품질 러

Gardener API에 `quality_preset: "osteon"`만(파일 `dataset_json_path` 없이) 보내 eval를 수행한 결과다. (MCP: `gardener_quality_run` 프리셋 모드, `gopedia_mcp` `npm run test:gardener-e2e`와 동일 유형.)

| 항목 | 값 |
|------|----|
| **측정** | 2026-04-07 경 (로컬 스택) |
| **run_id** | `e2825756-262a-4d02-9749-17e59c7456f8` |
| **dataset_id** (응답에 포함된 경우) | `120971ed-dcd7-4087-a25e-e74b4aa3f25e` |
| **쿼리 수** | 30 |
| **top_k** | 10 |
| **search_detail** | summary (런 기본) |

### 2-1. 집계 지표 (aggregate)

`GET /runs/{id}/metrics`에서 `scope: "aggregate"`로 집계된 IR·요약 지표.

| 지표 | 값 |
|------|-----|
| **Recall@5** | 1 |
| **MRR@10** | 0.95 |
| **nDCG@10** | 0.9631 |
| **P@3** | 0.3333 |
| **ragas/context_relevance** | 0.9 |
| **summary/total_tokens** | 5484 |
| **summary/cost_total_usd** | 0.00157 |
| **summary/quality_score** | 1 |

### 2-2. KPI 요약 (`GET /runs/{id}/kpi-summary`)

| 키 | 값 |
|----|-----|
| `mean_recall_at_5` | 1.0 |
| `aggregate_recall_at_5` | 1.0 |
| `total_tokens` | 5484.0 |
| `cost_total_usd` | 0.00157 (부동소수) |

### 2-3. 실패 샘플 (Recall@5 = 0)

- **건수**: 0 (30쿼리 전부 hit)
- **per-query 테이블**: “실패 샘플” 항목 없음

### 2-4. 이 리포트와 v0.6.0 universitas-factual

| 구분 | 본 절 (osteon 번들) | v0.6.0 `universitas_factual_v1` 등 |
|------|---------------------|-------------------------------------|
| 데이터셋 | Gardener 내장 **osteon** 샘플 | 44q 사실·도메인 라벨 데이터셋 |
| 목적 | osteon 골드셋·파이프라인 스모크 | universitas 운영 인제스트·IR |
| **결론** | **동일 지표 숫자를 v0.6.0과 직접 대등 비교하지 말 것** | |

### 2-5. 지표가 높게 나올 수 있었던 이유 (해석)

아래는 **2절의 aggregate·KPI가 상단에 몰릴 수 있는** 구조적 이유이며, “일반 난이도 RAG에서도 항상 동일”을 뜻하지 않는다.

1. **큐레이션된 osteon 번들**  
   `quality_preset: "osteon"`는 Gardener에 동봉된 `sample_osteon_guide_30_v2` 계열 **질의·qrel**로, 스토리·용어·정답 청크가 **osteon 가이드 맥락**에 맞춰져 있다. 광의의 오픈도메인·다중 소스 혼재 데이터셋보다 **Recall@5 / MRR / nDCG가 올라가기 쉬운** 전형이다.

2. **쿼리 수 N=30**  
   표본이 작으면 **한 러에서** 지표가 **최댓값 근처**로 보이기 쉽고, 수백 쿼리 풀과 **동일 스케일의 “안정한 평균”**으로 읽기 어렵다.

3. **v0.6.0 universitas-factual과의 난이도 차**  
   44q 사실 질의, 도메인 분산, `target_id` exact-match·qrel 드리프트 이슈 등이 있었던 리포트와 **평가 정의·골드 품질이 다름**. 본 osteon 러는 **“번들 골드 + 현재 스택”에 대한 스모크/회귀**에 가깝다.

4. **지표 포화는 부분적**  
   `P@3` ≈ 0.33, `ragas/context_relevance` 0.9 등 **만점이 아닌** 항목이 있어, “모든 하위 지표가 한없이 포화”가 아님을 전제로 **해당 러·집계 정의 안의 상위 구간**으로 읽는 것이 타당하다.

5. **인덱스 규모(위 인제스트 표)**  
   **문서 9 / L3 1,757** 수준은 **상대적으로 작고**, 골드 질의가 그 코퍼스·도메인과 **정합**될 때 상위 *k* 안에 정답이 들어갈 **조건이 대규모·다도메인 인덱스보다 유리할 수 있다** (골드가 코퍼스 밖이면 지표는 오히려 떨어짐—osteon 프리셋은 전자에 맞게 설계된 측면이 있다).

---

## 3. 재현 절차 (요약)

```bash
# Gopedia / Gardener 가동 후

# (A) gopedia_mcp — 스모크
cd /path/to/gopedia_mcp
npm run test:mcp
# → data/logs/mcp-smoke-test-*.json

# (B) gopedia_mcp — Gardener 도구·헬스만 빠르게
npm run test:gardener-mcp

# (C) gopedia_mcp — 풀 파이프라인 (osteon)
unset DATASET_JSON_PATH USE_DATASET_FILE
export QUALITY_PRESET=osteon
npm run test:gardener-e2e
```

Gardener만으로 재현할 때는 [README](README.md)의 `POST /runs` (또는 `quality_preset` 필드) 절차를 따른다.

---

## 4. 결론

- **컴포넌트 시점**: Gopedia `6fd6a7bc` + gardener_gopedia `22f71eb` (`gardener-gopedia` 0.1.0) + gopedia_mcp `b2ccbd3` (MCP 서버 2.1.0) 조합으로 기록했다.
- **인제스트**: 동일 DB 기준 **문서 9**, **L3 1,757** (위 “인제스트 데이터 규모” 절).
- **MCP 스모크**는 dep·기본 검색·시나리오 6건 중 3건에서 Qdrant 빈 컨텍스트 이슈가 있었고, **osteon 30q 평가**는 Recall@5=1, 실패 쿼리 0으로 완료되었다. 높은 수치는 **2-5**의 큐레이션·N=30·코퍼스 규모·평가 정의를 함께 고려할 것. 두 측정(스모크 vs IR 러)은 경로·데이터가 달라 **상호 보정**해 해석하는 것이 좋다.
- **다음 측정** 시 동일 `run_id`·커밋·일시·**Postgres 스냅샷(문서·L3 건수)** 를 본 파일에 갱신해 추적하는 것을 권장한다.
