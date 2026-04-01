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

## 리포트 목록

| 버전 | 날짜 | 대상 | 파일 |
|------|------|------|------|
| v0.1.0 | 2026-04-01 | neunexus, gopedia universitas | [v0.1.0_2026-04-01_neunexus-gopedia.md](v0.1.0_2026-04-01_neunexus-gopedia.md) |
| v0.2.0 | 2026-04-01 | neunexus, gopedia universitas, 코드 도메인 | [v0.2.0_2026-04-01_neunexus-gopedia.md](v0.2.0_2026-04-01_neunexus-gopedia.md) |
