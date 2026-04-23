# Markdown Ingest/Embedding 통합 전략 (Gopedia)

이 문서는 **통합 정책 허브**입니다. 상세 구현 가이드는 하위 문서를 참조합니다.

- 프로세스 상세: `markdown_process.md`
- 사이드카 데이터 상세: `markdown_sidecar_data_strategy.md`
- 계층/그래프 검색 상세: `hierarchy_embedding_strategy.md`

---

## 1. 고정 원칙

1. **데이터 계층 분리**
   - Semantic layer: `JSON-LD`
   - Factual layer: `Parquet/CSV/RDB`
2. **식별자(ID) 생성 기준**
   - 최상위 source 경로의 `.gopedia` 폴더 config에 저장된 `project_machine_id`를 기반으로 파생 ID를 생성
   - 공통 키: `project_id`, `dataset_id`, `entity_id`, `file_id`, `chunk_id`
3. **참조 문법 통일**
   - Markdown 본문에는 `[[data-ref:<dataset_id>]]` 형식으로만 외부 데이터 참조
4. **역할 경계**
   - Frontmatter: 운영 메타데이터(작성자, 수정일, 태그, 접근등급)
   - Sidecar: 도메인 지식(관계, 수치, 표, 스키마)
5. **Graph prefilter**
   - 검색 시 Graph prefilter는 항상 수행

---

## 2. Ingest/Chunking 전략 (목표 상태)

핵심 정책만 유지하고, 상세 규칙은 링크 문서로 위임합니다.

- 기본 모델: **L2(섹션) -> L3(검색)**
- sidecar까지 포함해 chunk 단위를 통합 관리
- Path-to-Context injection 상시 적용
- chunk 정책 변경 시 `pipeline_version` 기반 재인덱싱

상세 참조:
- `markdown_process.md` (단계별 ingest/chunk 흐름)
- `markdown_sidecar_data_strategy.md` (JSON-LD/Parquet sidecar chunk 규칙)
- `hierarchy_embedding_strategy.md` (계층/그래프 기반 검색 범위 축소)

---

## 3. Embedding 전략

요약 정책:

- chunk-aware embedding: markdown/sidecar 공통 기준 적용
- dual index: small index + parent index
- metadata-aware rerank와 연계 가능한 payload 표준 유지
- 최소 토큰 전달: 1차 ID 중심, 2차 확장

상세 참조:
- `markdown_process.md` (ingest -> embedding 운영 흐름)
- `markdown_sidecar_data_strategy.md` (sidecar 임베딩 단위/전처리)
- `hierarchy_embedding_strategy.md` (구조 기반 후보 축소와 하이브리드 쿼리)

---

## 4. Retrieval 오케스트레이션 (고정)

기본 순서는 고정합니다.

1. `TypeDB(TypeQL)` graph prefilter
2. `PostgreSQL/Parquet` factual prefilter
3. `Qdrant` vector search
4. rerank
5. context compose

### 질의 유형별 분기(가중치 조정)

- **관계 중심 질의**: graph 후보 폭 확대, factual 필터는 보조
- **수치/범위 질의**: factual 후보 폭 확대, vector는 보조
- **혼합 질의**: graph/factual 균형 후 vector + rerank 강화

---

## 5. Rerank 전략

### 5-1. Content-only vs Metadata-aware

- Content-only: 구현 단순, 일반 의미 검색에 유리
- Metadata-aware: 경로/섹션/태그/버전/권한 반영 가능, 운영 적합

### 5-2. 권장

- 기본: metadata-aware rerank
- fallback: content-only

예시 점수식:

`final = a*dense + b*cross_encoder + c*path_match + d*heading_match + e*tag_match + f*freshness`

---

## 6. 저장 포맷 선택 기준

### JSON-LD 적용 단위

- 기본: schema/manifest 단위(JSON-LD)
- 예외: 관계 밀도가 높은 소규모 테이블은 row 단위 JSON-LD 허용

### Parquet vs CSV

- 기본 저장: Parquet 우선(압축/스캔/스키마 안정성)
- CSV: 수동 점검/교환용
- LLM 입력: 원본 Parquet/CSV 직접 전달 대신, 선별 row를 compact 텍스트 또는 JSONL로 변환해 전달

---

## 7. 운영 품질 관리

품질 관리는 `doc/rag-test-reports`와 `gardener_gopedia` 지표를 기준으로 수행합니다.

- 핵심 IR 지표: `Recall@5`, `MRR@10`, `nDCG@10`, `P@3`
- 비교 원칙: 동일 dataset 기준 baseline/candidate 비교
- 주의: 서로 다른 dataset 간 절대 점수 직접 비교 금지

---

## 8. 표준 산출물

각 dataset는 아래 파일을 기본 세트로 관리합니다.

1. `{title}.md` (사람이 읽는 문서명 기준, slug 권장)
2. `{dataset_id}.parquet` (또는 CSV)
3. `{dataset_id}.schema.jsonld`
4. `{dataset_id}.profile.json`

권장 매핑:
- 문서 본문에는 `dataset_id`를 명시하고, 참조는 `[[data-ref:<dataset_id>]]`로 통일
- 파일명은 가독성(`title`)과 식별성(`dataset_id`)을 분리하여 운영

이 기준을 적용하면 markdown 중심 워크플로우를 유지하면서도, 구조/수치/관계 정보를 모두 안정적으로 검색 파이프라인에 반영할 수 있습니다.