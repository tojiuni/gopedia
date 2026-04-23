# RAG Test Reports

Ingest(Phloem) + Search(Xylem) 품질 테스트 결과를 버전별로 기록합니다.

## 파일 명명 규칙

```
<version>_<date>_<target>.md
```

예: `v0.1.0_2026-04-01_neunexus-gopedia.md`

## 버전 관리

```bash
# 태그 목록
git tag --list

# 태그 생성 (릴리즈 시)
git tag v0.2.0 -m "릴리즈 설명"
git push origin v0.2.0
```

## gardener_gopedia로 리포트 작성하기

[gardener_gopedia](../../../gardener_gopedia) — Gopedia 검색 품질을 측정하는 평가 서비스 (Gardener API: `18880`, Gopedia API: `18787`).

### 사전 준비

```bash
# 1. Gopedia 스택 실행 확인
curl -s http://127.0.0.1:18787/health

# 2. Gardener API 실행 (Gopedia의 Postgres 공유)
cd /neunexus/gardener_gopedia
export GARDENER_GOPEDIA_BASE_URL=http://127.0.0.1:18787
export POSTGRES_USER=... POSTGRES_PASSWORD=... POSTGRES_HOST=127.0.0.1 POSTGRES_DB=gopedia
uvicorn gardener_gopedia.main:app --host 0.0.0.0 --port 18880

# 또는 Docker Compose 환경에서는 GARDENER_DATABASE_URL 직접 지정
export GARDENER_DATABASE_URL=postgresql+psycopg://USER:PASS@127.0.0.1:5432/gopedia
```

### 평가 실행 절차

```bash
export GARDENER=http://127.0.0.1:18880
export GOPEDIA=http://127.0.0.1:18787

# 1. 데이터셋 등록 (dataset/ 디렉토리의 curated JSON 사용)
DS=$(curl -s -X POST "$GARDENER/datasets" \
  -H 'Content-Type: application/json' \
  -d @/neunexus/gardener_gopedia/dataset/universitas_gopedia_neunexus.json | jq -r .id)
echo "dataset_id=$DS"

# 2. (필요 시) target_data qrel → l3_id 해소
curl -s -X POST "$GARDENER/datasets/$DS/resolve-qrels" | jq .

# 3. 평가 실행 (버전 정보 태깅)
RUN=$(curl -s -X POST "$GARDENER/runs" \
  -H 'Content-Type: application/json' \
  -d "{\"dataset_id\":\"$DS\",\"top_k\":10,\"search_detail\":\"summary\",\"git_sha\":\"$(git -C /neunexus/gopedia rev-parse --short HEAD)\",\"index_version\":\"v0.x.0\"}" \
  | jq -r .id)

# 4. 완료 대기
curl -s -X POST "$GARDENER/runs/$RUN/wait" | jq '{status, id}'

# 5. 지표 조회
curl -s "$GARDENER/runs/$RUN/metrics" | jq .
```

### 주요 지표 (IR Metrics)

| 지표 | 의미 | 기준 |
|------|------|------|
| `Recall@5` | 상위 5개 결과에 정답 포함 비율 | 높을수록 좋음 |
| `MRR@10` | 상위 10개에서 정답 순위의 역수 평균 | 높을수록 좋음 |
| `nDCG@10` | 순위 가중 관련도 | 높을수록 좋음 |
| `P@3` | 상위 3개 결과의 정밀도 | 높을수록 좋음 |

### 버전 간 비교

```bash
# 베이스라인 vs 후보 비교 (같은 dataset_id 필수)
curl -s "$GARDENER/compare?baseline=$BASE&candidate=$CAND&metric=Recall@5" | jq .
```

### 리포트 작성 기준

1. **인제스트 현황** — `knowledge_l1/l2/l3` 문서 수, Qdrant 벡터 수, 컬렉션 상태
2. **벡터 품질** — 샘플 쿼리로 `GET /api/search` 결과 확인 (vector_score, combined_score)
3. **IR 지표** — `GET /runs/{id}/metrics` 결과 (Recall@5, MRR@10, nDCG@10)
4. **이전 버전 대비** — `GET /compare` 결과로 개선/퇴보 쿼리 식별
5. **파일명** — `<version>_<YYYY-MM-DD>_<target>.md` 규칙 준수 후 아래 목록에 추가

---

<a id="ir-metrics-snapshot"></a>

## Version별 IR 지표 현황 (요약)

리포트에서 인용한 **aggregate** 지표(소수 **둘째 자리** 반올림, `mcp-2.1.0`의 nDCG는 **0.9631 → 0.96**). **마지막 점(`mcp-2.1.0*`)**만 Gardener **osteon 30q** — 앞 7점은 **`universitas_factual_v1` 44q**와 동일 정의(연속 추세로 읽을 수 있음). **7↔8 구간**은 서로 **다른 데이터셋**이므로 개선/악화로 해석하지 말 것. (v0.1.0은 [수동 쿼리](v0.1.0_2026-04-01_neunexus-gopedia.md)만 있음.)

```mermaid
xychart-beta
    title "IR aggregate (앞 7: universitas 44q, 마지막: mcp-2.1.0 osteon 30q*)"
    x-axis [v0.2, v0.3, v0.4, "v0.5.0", "v0.5.1", "v0.6a", "v0.6b", "mcp*"]
    y-axis "score" 0.0 --> 1.0
    line [0.79, 0.64, 0.50, 0.50, 0.61, 0.50, 0.48, 1.00]
    line [0.39, 0.56, 0.32, 0.28, 0.37, 0.36, 0.37, 0.95]
    line [0.49, 0.58, 0.37, 0.34, 0.45, 0.40, 0.40, 0.96]
```

| 시리즈 | v0.2 → v0.6b (44q) | mcp-2.1.0* (osteon 30q) | 출처 |
|--------|-------------------|-------------------------|------|
| **Recall@5** | 0.79, 0.64, 0.50, 0.50, 0.61, 0.50, 0.48 | **1.00** | 44q: [v0.2.0](v0.2.0_2026-04-01_neunexus-gopedia.md)…[v0.6.0 04-03](v0.6.0-reingest_2026-04-03_universitas-factual.md) · mcp: [§2-1](mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md) |
| **MRR@10** | 0.39, 0.56, 0.32, 0.28, 0.37, 0.36, 0.37 | **0.95** | 44q 출처는 Recall@5와 동일 · mcp §2-1 |
| **nDCG@10** | 0.49, 0.58, 0.37, 0.34, 0.45, 0.40, 0.40 | **0.96** | 동일 |
| **P@3** | 0.21, 0.21, 0.14, 0.14, 0.17, 0.17, 0.16 | **0.33** | 44q는 위행; mcp는 §2-1(만점 아님) |
| `summary/quality_score` (mcp만) | — | **1.0** | Gardener KPI |

> **v0.5.0** 열: [v0.5.0](v0.5.0_2026-04-02_universitas-factual.md)에서 **v0.4.0 대비 final** run 집계. `mcp*`: [mcp-2.1.0…](mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md) `GET /runs/{id}/metrics` / KPI. Mermaid `xychart` **색·범례**는 뷰어마다 달라 **수치는 위 표**를 본다.

> **주석: `mcp-2.1.0` IR이 높게 보일 수 있었던 이유** (상세: [mcp-2.1.0… §2-5](mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md)와 동일 취지)  
> 1) **큐레이션된 osteon 번들** — Gardener 내장 `sample_osteon_guide_30` 계열은 질의·qrel이 osteon 가이드 맥락에 맞춰 있어, 오픈도메인 44q보다 R@5·MRR·nDCG가 오르기 쉽다.  
> 2) **N=30** — 표본이 작으면 한 러에서 지표가 최댓값 근처로 보이기 쉽다.  
> 3) **평가 정의가 universitas 44q와 다름** — 사실·도메인 분산, qrel 정의·난이도가 같지 않다.  
> 4) **완전 만점은 아님** — P@3·RAGAS 등은 포화가 아닌 항목이 있음(리포트 표 참고).  
> 5) **인덱스 규모** — 당시 **문서 9 / L3 1,757** 수준이면 골드와의 정합이 맞을 때 top-*k*에 유리할 **조건**이 대규모·혼재 코퍼스보다 맞는 편이 될 수 있다.  
> → **회귀·스모크**로 읽고, v0.6 universitas-factual과 **점수 랭크를 직접 맞대지 말 것.**

---

## 리포트 목록

| 버전 | 날짜 | 대상 | 파일 |
|------|------|------|------|
| v0.1.0 | 2026-04-01 | neunexus, gopedia universitas (수동 쿼리·score) | [v0.1.0_2026-04-01_neunexus-gopedia.md](v0.1.0_2026-04-01_neunexus-gopedia.md) |
| v0.2.0 | 2026-04-01 | neunexus, gopedia universitas, 코드 도메인 | [v0.2.0_2026-04-01_neunexus-gopedia.md](v0.2.0_2026-04-01_neunexus-gopedia.md) |
| v0.3.0 | 2026-04-01 | neunexus, gopedia universitas + 전체 Universitas(gardener) | [v0.3.0_2026-04-01_neunexus-gopedia.md](v0.3.0_2026-04-01_neunexus-gopedia.md) |
| v0.4.0-dev | 2026-04-01 | IMP-02 API 연동 + 재기동 후 gardener 재측정 | [v0.4.0_2026-04-01_neunexus-gopedia.md](v0.4.0_2026-04-01_neunexus-gopedia.md) |
| v0.5.0 | 2026-04-02 | `universitas_factual_v1` 44q, 임베딩 e5-large (로컬) | [v0.5.0_2026-04-02_universitas-factual.md](v0.5.0_2026-04-02_universitas-factual.md) |
| v0.5.1 | 2026-04-02 | v0.5.0 + server-os 문서 보강·재인제스트 | [v0.5.1_2026-04-02_universitas-factual.md](v0.5.1_2026-04-02_universitas-factual.md) |
| v0.6.0 reingest | 2026-04-02 | `universitas_factual_v1` (L1 Merkle 등 인제스트 변경) | [v0.6.0-reingest_2026-04-02_universitas-factual.md](v0.6.0-reingest_2026-04-02_universitas-factual.md) |
| v0.6.0 reingest | 2026-04-03 | 동 데이터셋 후속 | [v0.6.0-reingest_2026-04-03_universitas-factual.md](v0.6.0-reingest_2026-04-03_universitas-factual.md) |
| mcp 2.1.0 + stack | 2026-04-08 | Gardener + Gopedia + gopedia_mcp (스모크 + **osteon 30q**; IR는 위 차트 `mcp*` 절) | [mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md](mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md) |
