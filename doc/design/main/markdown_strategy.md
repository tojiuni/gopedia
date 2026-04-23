# Markdown Ingest/Embedding 통합 전략 (Gopedia)

이 문서는 `markdown_process.md`, `markdown_sidecar_data_strategy.md`, `hierarchy_embedding_strategy.md`를 통합한 **운영 기준서**입니다.

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

## 2. Ingest 전략

### 2-1. Markdown 분할

- 1차: 헤더 기준 분할 (`#`, `##`, `###`)
- 2차: 재귀 분할 (`\n\n` -> `\n` -> ` `)
- overlap: 10-20%

### 2-2. Path-to-Context Injection (상시 적용)

각 chunk 입력 앞에 canonical path를 주입합니다.

- 예: `[Path: <project>/<category>/<file>#<h1>/<h2>]`
- 장점: 계층 맥락 보존, 관계형 질의 recall 개선
- 단점: 토큰 증가 가능
- 운영 규칙: path는 축약된 canonical 형태만 사용하여 토큰 증가를 제한

### 2-3. Table ingest 규칙

- 대형 테이블 raw 전체 임베딩 금지
- 허용 방식:
  - 핵심 컬럼/엔티티 중심 인덱싱
  - Row/Column 요약 문장화(linearization)
  - summary sidecar 생성 후 임베딩

---

## 3. Embedding 전략

### 3-1. Dual index

- Small chunk 검색 인덱스 + Parent context 전달 인덱스
- 검색 정밀도는 small chunk로 확보하고, 응답 생성은 최소 parent 단위로 보강

### 3-2. 최소 토큰 전달 정책

기본 응답에서는 텍스트 본문을 최소화하고 식별자 중심으로 전달합니다.

- 1차 전달: 핵심 텍스트 스니펫 + `l3_id`, `l2_id`, `l1_id`
- 2차 확장: 필요 시(추가 질문/근거 요청) parent 본문 추가 조회
- 목적: 근거 추적성 유지 + 토큰 사용량 절감

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

1. `{dataset_id}.md`
2. `{dataset_id}.parquet` (또는 CSV)
3. `{dataset_id}.schema.jsonld`
4. `{dataset_id}.profile.json`

이 기준을 적용하면 markdown 중심 워크플로우를 유지하면서도, 구조/수치/관계 정보를 모두 안정적으로 검색 파이프라인에 반영할 수 있습니다.