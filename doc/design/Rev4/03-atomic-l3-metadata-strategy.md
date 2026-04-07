# 03. Atomic L3 + Metadata-Aware Retrieval Strategy — Rev4

## 목적

기존 chunking의 장점을 유지하면서, 다음 두 가지를 동시에 달성한다.

1. **L3 원자성(atomicity) 유지**: 사실 단위를 잘게, 명확하게 보존
2. **검색/복원 품질 개선**: 벡터화와 랭킹에서 메타데이터를 적극 활용

핵심 원칙은 **"L3는 합치지 않고, 검색 시 똑똑하게 묶는다"**이다.

---

## 문제 정의

평가에서 miss로 분류된 일부 쿼리는 top hit snippet이 정답 의미와 유사했지만 `target_id`가 달랐다.
이는 과도한 병합보다는 **qrel drift + 구조 정보 미활용** 문제에 가깝다.

따라서 L3를 크게 병합하기보다, L3를 원자적으로 유지하고 메타 기반 검색/복원을 강화한다.

---

## 설계 원칙

### 1) L3는 원자적으로 유지

- ordered list item 1개 = L3 1개
- bullet item 1개 = L3 1개
- code block은 의미 단위로 분할
- table은 row/의미 단위로 분할
- paragraph는 사실 1~2개 단위로 분할

권장 길이:
- 목표: 200~500 tokens
- 상한: 700 tokens

> 주의: 짧은 L3를 임의 병합하는 대신, retrieval 단계에서 block/group 기반 결합을 수행한다.

### 2) 병합은 저장이 아니라 조회 시 수행

- 저장 시: atomic L3 유지
- 조회 시: block group / sibling / neighbor metadata를 이용해 context 확장

---

## L3 메타데이터 스키마 (권장)

복원과 랭킹을 위해 다음 필드를 L3 metadata로 유지한다.

- 식별/위치:
  - `doc_id`, `l1_id`, `l2_id`, `l3_id`
  - `chunk_index`, `char_start`, `char_end`
- 구조:
  - `block_type` (`paragraph`, `ol_item`, `ul_item`, `code`, `table_row`)
  - `block_group_id` (같은 리스트/코드/표 묶음)
  - `list_level`, `list_item_no` (해당 시)
- 연결:
  - `prev_l3_id`, `next_l3_id`
- 의미 힌트:
  - `source_path`, `title`, `section_heading`, `breadcrumb`
  - `fact_tags` (`port`, `url`, `image`, `auth`, `step` 등)
  - `domain_tags` (`registry`, `traefik`, `pgadmin`, `morphso` 등)

---

## 벡터화 전략 (본문 + 경량 메타)

L3 임베딩 시 본문만 넣지 않고, 경량 메타를 prefix로 결합한다.

예시 입력 포맷:

`[domain=registry][type=ol_item][tag=step] docker push registry.toji.homes/<image>:<tag>`

기대 효과:
- 짧은 L3에서도 의도 중심 임베딩 유지
- 유사 문장 간 도메인 혼선 감소
- 절차형/설정형 질의에서 top-k 안정성 향상

---

## Retrieval / Rerank 전략

### 단계 1. Dense retrieval (넓게)

- 기존과 동일하게 top-k 후보를 넓게 수집

### 단계 2. Metadata-aware rerank (정밀하게)

가중치 예시:

- `source_path_hint` 일치: +++
- `title/section` 일치: ++
- `fact_tags` 일치: ++
- 질문 유형과 `block_type` 일치(절차형↔`ol_item`): ++
- 동일 `block_group_id` 내 연속성: +

### 단계 3. Context assembly (복원)

최종 답변 컨텍스트 구성 시:

1. hit L3 선택
2. 같은 `block_group_id` 우선 확장
3. 필요 시 `prev/next` 1~2개만 확장
4. `char_start/end` 기준 원문 순서 재조합

---

## Restore 전략

restore는 단순 "hit 하나 반환"이 아니라 "구조적으로 읽히는 단위 복원"을 목표로 한다.

- 리스트 질의: 같은 `block_group_id`의 item을 우선 결합
- 코드 질의: 같은 code block 범위 우선 결합
- 표 질의: row 인접성 기준 결합
- 과확장 방지: 토큰 budget 초과 시 중단

이렇게 하면 원자적 L3를 유지하면서도 사람이 읽기 좋은 복원이 가능하다.

---

## 운영 가드레일

1. **차원 정합성 체크 (필수)**
   - embedder vector size == Qdrant collection size 불일치 시 fail-fast

2. **관측 지표**
   - aggregate: Recall@5, MRR@10, nDCG@10, P@3
   - 분석: "miss인데 snippet 의미 유사" 비율, 도메인별 hit 분포
   - 복원: 평균 context 길이, group expansion 비율

3. **롤아웃 방식**
   - A/B 비교: 기존 전략 vs atomic+metadata 전략
   - 한 번에 한 축만 변경 (chunking, metadata rerank, restore 순)

---

## 비목표 (이번 범위 아님)

- 대규모 스키마 마이그레이션 자동화
- full hybrid(BM25 + dense) 검색 전면 도입
- cross-encoder 파이프라인 전면 교체

---

## 기대 효과 요약

- L3 원자성 유지로 fact traceability 강화
- metadata-aware rerank로 도메인 오매칭 감소
- restore 시 구조 보존으로 답변 가독성 향상
- qrel drift 영향 완화 및 평가 안정성 개선
