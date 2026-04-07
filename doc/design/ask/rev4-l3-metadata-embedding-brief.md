# Rev4 L3 Metadata + Embedding Brief

## 목적

이 문서는 아래 두 컨텍스트를 한 번에 정리한다.

- 대화에서 논의한 L3 metadata/embedding 설계 포인트
- `/home/ubuntu/.cursor/plans/chunking-gopedia-gardener-plan_39159326.plan.md` 실행 계획 핵심

대상 독자는 `gopedia`/`gardener_gopedia` Rev4 개선 작업을 이어서 구현할 개발자다.

## 기반 플랜 요약

기준 플랜: `/home/ubuntu/.cursor/plans/chunking-gopedia-gardener-plan_39159326.plan.md`

핵심 목표:

- Atomic L3 유지 + metadata-aware retrieval/restore 구현
- gopedia 인제스트/검색/복원 품질 향상 + dimension fail-fast 고도화
- gardener qrel resolve 신뢰도 개선 및 drift 진단 가시성 확보

단계 요약:

1. Gopedia chunking을 원자 L3 중심으로 정렬
2. `knowledge_l3.source_metadata` + Qdrant payload 확장
3. metadata-aware embedding/rerank/restore 도입
4. embedder dimension 검증 fail-fast 일원화
5. gardener qrel resolver 가중치/가시성 강화
6. near-miss/same-doc drift 진단 및 dataset 게이트 반영
7. universitas 재평가 및 리포트 업데이트

## L3 Metadata 권장 스키마 (실행용)

아래는 "바로 구현 가능한 최소-권장 필드"다.

### 1) 구조/연결 필드

- `block_type` (string, 필수)
  - 권장 값: `paragraph`, `ordered`, `unordered`, `code`, `table_row`, `image`, `heading`
  - 현재 코드 호환 최소값: `heading|ordered|table|code|image`
- `block_group_id` (string, 필수)
  - 예: `{l2_id}:g1`
- `chunk_index` (int, 필수)
  - 그룹 내 0-based 순서
- `prev_l3_id`, `next_l3_id` (uuid string, 선택)
- `list_level` (int, 목록일 때)
- `list_item_no` (int, ordered일 때)

### 2) 위치/문맥 힌트 필드

- `char_start`, `char_end` (int, 선택)
- `section_heading` (string, 선택)
- `breadcrumb` (string, 선택)
- `source_path` (string, 선택)

### 3) 의미 태그 필드

- `fact_tags` (string[], 권장)
  - 초기 vocab 예: `step`, `command`, `url`, `path`, `port`, `auth`, `version`, `error`, `config`, `image`
- `domain_tags` (string[], 권장)
  - 초기 vocab 예: `gopedia`, `gardener`, `qdrant`, `postgres`, `docker`, `registry`, `traefik`, `redis`, `pgadmin`, `morphso`

권장 운영 원칙:

- 자유 텍스트 태깅보다 고정 vocab 우선
- 과다 태깅 방지(필드당 최대 3개 정도)

## 왜 필요한가 (요약)

- 짧은 L3 문장/라인의 의미 보강 (도메인 혼선 감소)
- rerank 근거 강화 (`query intent` vs `block_type/tag` 정합)
- restore 품질 개선 (`block_group_id` + neighbor 기반)
- miss 원인 추적성 향상 (why miss 분석 가능)
- qrel drift 완화 (의미 유사하나 id mismatch인 케이스 감소)

## Gopedia vs mem0 임베딩 방식 비교

### Gopedia 현재 방식

- 인제스트 시점에 L3 생성 후 Qdrant upsert 전 임베딩
- L3 텍스트(`item.text`)를 그대로 `Embed(ctx, text)`에 전달
- code domain은 `anchor` L3 중심 임베딩 경로 존재
- local embedder는 문서/쿼리 prefix를 분리
  - ingest: `passage`
  - retrieval query: `query`

관련 코드:

- `internal/phloem/sink/writer.go`
- `internal/phloem/embedder/local.go`
- `flows/xylem_flow/retriever.py`

### mem0 방식

- memory CRUD 플로우에 임베딩 호출이 직접 결합
- `embed(text, memory_action)` 형태 (`add|search|update`)
- 입력 텍스트는 memory/query 본문 문자열 자체
- provider별 경량 정규화(예: newline 제거) 수행

관련 코드:

- `/neunexus/mem0/mem0/memory/main.py`
- `/neunexus/mem0/mem0/embeddings/*.py`

### 비교 핵심

- gopedia: 문서 구조 기반 L3 임베딩 파이프라인
- mem0: 액션 기반 memory 임베딩 파이프라인
- 공통점: 본문 위주 임베딩 + provider 교체 가능
- 차이점: mem0는 action-aware, gopedia는 현재 text-only 중심

## Metadata를 임베딩 입력에 함께 넣는 OSS 사례

아래는 "본문 + 메타데이터 결합 임베딩"을 지원/권장하는 대표 오픈소스다.

1. Haystack (deepset)
   - `meta_fields_to_embed=[...]`로 문서 본문 + 메타 필드 동시 임베딩
   - 튜토리얼: <https://haystack.deepset.ai/tutorials/39_embedding_metadata_for_improved_retrieval>

2. Vespa
   - indexing language에서 필드 결합 후 `embed`
   - 예: `(input title || "") . " " . (input body || "") | embed ...`
   - 문서: <https://docs.vespa.ai/en/rag/embedding.html#concatenating-input-fields>

3. OpenSearch
   - `text_embedding` processor로 임베딩 필드 생성
   - 메타+본문은 ingest pipeline에서 결합 필드 생성 후 매핑하는 방식으로 구현 가능
   - 문서: <https://opensearch.org/docs/latest/ingest-pipelines/processors/text-embedding/>

참고:

- LlamaIndex도 metadata extraction + metadata-aware indexing 패턴을 공식 문서로 제공
  - <https://developers.llamaindex.ai/python/examples/metadata_extraction/metadataextractionsec/>

## gopedia 적용 제안 (작업 우선순위)

1. `knowledge_l3.source_metadata` 필드 스키마 확정
2. Qdrant payload에 최소 필드 확장
   - `block_type`, `block_group_id`, `chunk_index`, `fact_tags`, `domain_tags`
3. 임베딩 입력 prefix 실험
   - 예: `[domain=qdrant][type=ordered][tag=step] <content>`
4. metadata-aware rerank 가중치 도입
5. restore에서 group-first + bounded neighbor 확장
6. universitas 재평가로 회귀/개선 검증

## 산출물 위치

- 본 문서: `doc/design/ask/rev4-l3-metadata-embedding-brief.md`
- 대화 기반 보조 정리:
  - `tests/mem0_chunking_summary.md`
  - `tests/mem0_embedding_summary.md`
