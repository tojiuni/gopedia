# 04. Hierarchical Embedding Strategy — Rev4

## 목적

RAG 응답 시 **토큰 사용량을 최소화**하면서도 관련 콘텐츠를 정확하게 찾는 것이 목표다.

현재 구조는 L3 flat search 후 모든 결과를 LLM context로 전달하는 방식이어서,
쿼리와 관련 없는 문장들까지 포함되어 토큰 낭비가 발생한다.

이를 해결하기 위해 **L1 → L2 → L3 계층 검색(Hierarchical Search)** 을 도입하고,
각 레벨의 임베딩을 하위 레벨의 의미를 포함하도록 강화한다.

---

## 현재 임베딩 구조

```
L1  →  Qdrant  (title + content[:500])
L2  →  PostgreSQL 만  (임베딩 없음)
L3  →  Qdrant + PostgreSQL  (문장 단위 원문)
```

**검색 흐름 (현재)**

```
Query
  └─ Qdrant L3 flat search
       └─ top-K L3 점수순 선택
            └─ PostgreSQL JOIN (l2_id, l1_id)
                 └─ L2 summary + L1 title → LLM context
```

문제: 관련도 낮은 L3 다수가 context에 포함 → 토큰 낭비 + 노이즈.

---

## 제안 아키텍처

### 임베딩 텍스트 구성

| 레벨 | 임베딩 텍스트 | 현재 → 변경 |
|------|-------------|------------|
| **L3** | 문장 원문 (유지) | 변경 없음 |
| **L2** | `L2 summary + "\nKeywords: " + top-N keywords(L3 children)` | 신규 |
| **L1** | `title + "\nSections: " + concat(L2 summaries, maxLen=1000)` | 변경 |

### 검색 흐름 (제안)

```
Query
  │
  ├─ [Step 1] L1 Qdrant search  →  관련 문서 후보 선별
  │           (filter: project_id)
  │
  ├─ [Step 2] L2 Qdrant search  →  관련 섹션 후보 선별
  │           (filter: l1_id IN matched_l1s)
  │
  ├─ [Step 3] L3 Qdrant search  →  정확한 문장 선별
  │           (filter: l2_id IN matched_l2s)
  │
  └─ [Step 4] 응답 단위 결정
              ├─ L3 hit score 높음  →  L3 + 주변 문장 (narrow context)
              ├─ L2 hit score 높음  →  L2 summary 전달 (mid context)
              └─ L1 hit score만 높음 →  L1 TOC/overview 전달 (broad context)
```

토큰 효율: 쿼리가 특정 문장에 집중될수록 L3 몇 개만 전달. 섹션 수준이면 L2 summary로 충분.

---

## keyword vs summary 방식 비교

L2·L1 임베딩 텍스트 구성에 두 가지 방식이 있다.

### 방식 A — keyword 집계

```
L2 embed = L2 summary + "Keywords: " + union(L3 keyword_so.canonical_name)
L1 embed = title       + "Keywords: " + union(L2 keyword aggregation)
```

**장점**: 세부 용어가 상위 레벨 벡터에 반영됨
**단점**:
- 현재 keyword 추출이 단순 regex (`[A-Za-z0-9][A-Za-z0-9_-]{2,}`) → 노이즈 큼
- 한국어 토큰 미지원
- `keyword_ids`(int64) → `keyword_so.canonical_name` JOIN 필요
- L3 변경 시 L2·L1 re-embed cascade 발생

### 방식 B — summary 집계 (권장)

```
L2 embed = knowledge_l2.summary  (AI 생성 요약, 이미 존재)
L1 embed = title + concat(knowledge_l2.summary list, truncated to 1000 chars)
```

**장점**:
- AI 요약이 keyword보다 의미 표현이 풍부함
- `knowledge_l2.summary` 컬럼 이미 존재 → 추가 추출 불필요
- 한국어·영어 모두 자연스럽게 처리
- 구현 단순

**단점**: L2 summary가 비어있는 경우 fallback 필요 (heading 텍스트로 대체)

### 방식 C — 하이브리드 (향후 개선)

```
L2 embed = L2 summary + top-K TF-IDF keywords from L3 children
```

summary 품질 + 핵심 용어 보완. keyword 추출 품질 개선 후 적용.

---

## 구현 전략

### Phase 1 — L2 임베딩 추가 (summary 방식)

**변경 대상**: `internal/phloem/sink/writer.go`

```go
// L2 embed text 구성 (summary 우선, 없으면 heading fallback)
func l2EmbedText(summary, heading string) string {
    if strings.TrimSpace(summary) != "" {
        return summary
    }
    return heading
}
```

- `Write()` 내 L2 insert 직후 `s.embed.Embed(ctx, l2EmbedText(...))` 호출
- Qdrant `upsertL2Point()` 신규 함수 (payload: `l1_id`, `l2_id`, `section_id`, `version_id`, `project_id`)
- L2 Qdrant 포인트 ID 형식: `{docUUID}_l2_{l2UUID}`

### Phase 2 — L1 임베딩 강화 (L2 summaries 집계)

**현재**: `title + content[:500]` (원문 앞부분)
**변경**: `title + "\n" + concat(l2_summaries, sep="\n", max=1000)`

```go
// L2 summary 목록 조회 후 L1 embed text 재구성
l2Summaries, _ := fetchL2Summaries(ctx, s.pg, l1UUID)
l1EmbedText := buildL1EmbedText(msg.Title, l2Summaries)
```

L2 insert 완료 후 L1 임베딩 생성 → 순서 중요.

### Phase 3 — 검색 계층화 (Xylem)

**변경 대상**: `flows/xylem_flow/retriever.py`

```python
def hierarchical_search(query, project_id, settings):
    # Step 1: L1 search
    l1_hits = qdrant_search(query_vec, filter={"project_id": project_id},
                            collection=settings.collection, source_type="l1", top_k=5)

    # Step 2: L2 search within matched L1s
    l1_ids = [h.payload["l1_id"] for h in l1_hits]
    l2_hits = qdrant_search(query_vec, filter={"l1_id": {"any": l1_ids}},
                            source_type="l2", top_k=10)

    # Step 3: L3 search within matched L2s
    l2_ids = [h.payload["l2_id"] for h in l2_hits]
    l3_hits = qdrant_search(query_vec, filter={"l2_id": {"any": l2_ids}},
                            source_type="l3", top_k=20)

    return decide_response_granularity(l1_hits, l2_hits, l3_hits)
```

### 응답 단위 결정 로직

```
L3 top score >= threshold_l3  →  L3 context (문장 + 주변 N개)
L2 top score >= threshold_l2  →  L2 summary
L1 top score만 유효           →  L1 title + TOC
```

임계값은 평가 지표(Recall@5, MRR@10) 기반으로 튜닝.

---

## Qdrant 포인트 구조 변경

### 기존 (L3 포인트)

```json
{
  "l1_id": "...", "l2_id": "...", "l3_id": "...",
  "section_id": "...", "version_id": "...",
  "keyword_ids": [...], "source_type": "heading", "project_id": 1
}
```

### 추가 (L2 포인트)

```json
{
  "l1_id": "...", "l2_id": "...",
  "section_id": "...", "version_id": "...",
  "source_type": "l2", "project_id": 1
}
```

### 변경 (L1 포인트)

```json
{
  "l1_id": "...",
  "version": 1, "source_type": "l1", "project_id": 1
}
```

`source_type` 필드를 `"l1"` / `"l2"` / `"heading"` / `"code"` 로 구분하여
Qdrant filter에서 레벨별 분리 검색 가능.

---

## 업데이트 cascade 정책

| 변경 사항 | L3 re-embed | L2 re-embed | L1 re-embed |
|----------|------------|------------|------------|
| L3 내용 변경 | O | O (summary 재생성 후) | O (L2 list 갱신 후) |
| L2 heading 변경 | - | O | O |
| L1 title 변경 | - | - | O |

현재 content hash 기반 deduplication(`summary_hash`)을 L2 임베딩 갱신 트리거로 활용 가능.
L3 변경 → L2 summary 재생성 → `summary_hash` 변경 감지 → L2 re-embed → L1 re-embed.

---

## 구현 우선순위

1. **L2 임베딩 추가** (Phase 1) — 가장 즉각적인 효과, 구현 단순
2. **L1 임베딩 강화** (Phase 2) — L2 완료 후 자연스럽게 연결
3. **Xylem 계층 검색** (Phase 3) — 검색 품질 평가 후 적용
4. **keyword 품질 개선** — 추후 방식 C 전환 시 전제 조건

---

## 관련 문서

- [`01-chunking-architecture.md`](./01-chunking-architecture.md) — L2 청킹 전략
- [`02-chunking-improvements.md`](./02-chunking-improvements.md) — 청킹 개선 이력
- [`03-atomic-l3-metadata-strategy.md`](./03-atomic-l3-metadata-strategy.md) — L3 원자성 + 메타데이터 검색
