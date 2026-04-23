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

## Version별 IR 지표 현황 (요약)

리포트 파일에서 인용한 **aggregate** IR 지표(소수 **둘째 자리** 반올림; v0.1.0은 IR 집계 없음). **데이터셋·쿼리 수·qrel이 다르면(특히 osteon 30q vs universitas 44q) 셀 값을 동일 “개선/악화” 축으로 직접 맞대지 말 것.**

### 전 리포트 통합 표 (Gardener `aggregate` 기준, 가능한 항목)

| 리포트 / 라벨 | 날짜 | 데이터셋 (N) | Recall@5 | MRR@10 | nDCG@10 | P@3 | 링크 |
|---------------|------|--------------|----------|--------|---------|-----|------|
| v0.1.0 | 2026-04-01 | — (IR 집계 없음) | — | — | — | — | [v0.1.0](v0.1.0_2026-04-01_neunexus-gopedia.md) (수동 쿼리·score) |
| v0.2.0 | 2026-04-01 | Gardener IR (리포트 §4) | 0.79 | 0.39 | 0.49 | 0.21 | [v0.2.0](v0.2.0_2026-04-01_neunexus-gopedia.md) |
| v0.3.0 | 2026-04-01 | Gardener IR (리포트 §4) | 0.64 | 0.56 | 0.58 | 0.21 | [v0.3.0](v0.3.0_2026-04-01_neunexus-gopedia.md) |
| v0.4.0-dev | 2026-04-01 | Gardener IR (리포트 §4) | 0.50 | 0.32 | 0.37 | 0.14 | [v0.4.0](v0.4.0_2026-04-01_neunexus-gopedia.md) |
| v0.5.0 (final) | 2026-04-02 | `universitas_factual_v1` (44q) | 0.50 | 0.28 | 0.34 | 0.14 | [v0.5.0](v0.5.0_2026-04-02_universitas-factual.md) |
| v0.5.1 | 2026-04-02 | `universitas_factual_v1` (44q) | 0.61 | 0.37 | 0.45 | 0.17 | [v0.5.1](v0.5.1_2026-04-02_universitas-factual.md) |
| v0.6.0 reingest | 2026-04-02 | `universitas_factual_v1` (44q) | 0.50 | 0.36 | 0.40 | 0.17 | [v0.6.0 04-02](v0.6.0-reingest_2026-04-02_universitas-factual.md) |
| v0.6.0 reingest | 2026-04-03 | `universitas_factual_v1` (44q) | 0.48 | 0.37 | 0.40 | 0.16 | [v0.6.0 04-03](v0.6.0-reingest_2026-04-03_universitas-factual.md) |
| **mcp-2.1.0 (stack)** | 2026-04-08 | **Gardener `osteon` preset (30q)** | **1.00** | **0.95** | **0.96** | **0.33** | [mcp-2.1.0](mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md) |

- **mcp-2.1.0** 행: `summary/quality_score` = **1.0** (리포트 §2-2), `ragas/context_relevance` = **0.9** (§2-1). **44q 행과 점수·추세를 직접 맞대지 말 것** (30q, 큐레이션·코퍼스가 다름 — 아래 주석 §).
- v0.2~v0.4는 리포트마다 `dataset_id`·인덱스가 다를 수 있으나, **수치**는 기존 README 차트·각 파일 aggregate와 맞췄다.

### 44q 추세 차트 (universitas_factual_v1 + 이전 Gardener run 일치 구간) — mcp **미포함**

`neunexus-gopedia` v0.2.x ~ `universitas-factual` v0.6.x에서 **R@5 / MRR / nDCG** 3 `line` (v0.1.0·**mcp-2.1.0은 데이터 축이 달라 미포함**; 전체는 위 **통합 표**). v0.1.0은 IR aggregate 없이 [수동 쿼리·score](v0.1.0_2026-04-01_neunexus-gopedia.md)만 기록.

```mermaid
xychart-beta
    title "universitas_factual_v1 (44q) — IR aggregate"
    x-axis [v0.2, v0.3, v0.4, "v0.5.0", "v0.5.1", "v0.6 (04-02)", "v0.6 (04-03)"]
    y-axis "score" 0.0 --> 1.0
    line [0.79, 0.64, 0.50, 0.50, 0.61, 0.50, 0.48]
    line [0.39, 0.56, 0.32, 0.28, 0.37, 0.36, 0.37]
    line [0.49, 0.58, 0.37, 0.34, 0.45, 0.40, 0.40]
```

| 시리즈 | 값 (v0.2 → v0.6 04-03) | 출처 |
|--------|------------------------|------|
| **Recall@5** | 0.79, 0.64, 0.50, 0.50, 0.61, 0.50, 0.48 | [v0.2.0](v0.2.0_2026-04-01_neunexus-gopedia.md) … [v0.6.0 04-03](v0.6.0-reingest_2026-04-03_universitas-factual.md) |
| **MRR@10** | 0.39, 0.56, 0.32, 0.28, 0.37, 0.36, 0.37 | 동일 |
| **nDCG@10** | 0.49, 0.58, 0.37, 0.34, 0.45, 0.40, 0.40 | 동일 |

> **P@3** (동일 44q): 0.21, 0.21, 0.14, 0.14, 0.17, 0.17, 0.16 — **차트 생략**(척도는 같으나 `xychart`에 네 번째 `line`을 쓰면 Mermaid 뷰어에 따라 겹쳐 보일 수 있음).  
> **v0.5.0 x축**: [v0.5.0 리포트](v0.5.0_2026-04-02_universitas-factual.md) run 열이 여럿이므로, 위 `v0.5.0` 열은 **v0.4.0 대비 final** 집계(Recall 0.50, MRR 0.278, …)다. Mermaid `xychart`는 `line` 색 구분이 환경마다 달라 **수치는 위 통합 표**를 기준으로 한다.

### 데이터 축 구분 (44q vs osteon 30q)

아래 **flowchart**는 `mcp-2.1.0`([§2-1~2-2](mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md) 수치)이 **universitas 44q**와 **같은 y축·개선/악화로 읽으면 안 된다**는 힌트다. **지표 수치**는 맨 위 **통합 표**에 한 줄로 포함돼 있다.

```mermaid
flowchart TB
  U["universitas_factual_v1 (44q)<br/>R@5 ≈ 0.48 ~ 0.79"]
  N[("같은 y축·추세로<br/>읽지 말 것")]
  O["Gardener osteon (30q)<br/>mcp-2.1.0: R@5=1, MRR≈0.95"]
  U --> N
  O --> N
```

> **보충**: y축·쿼리 수·qrel 정의가 다르므로, 위 두 축의 숫자를 **한 직선 추세(개선/악화)로 읽지 말 것.**

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
| mcp 2.1.0 + stack | 2026-04-08 | Gardener + Gopedia + gopedia_mcp (스모크 + **osteon 30q**, IR 수치는 위 §과 별 축) | [mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md](mcp-2.1.0_2026-04-08_gardener-gopedia-stack.md) |
