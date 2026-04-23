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

## 2. Ingest/Chunking 전략 (목표 상태)

### 2-1. Chunking 계층 모델

Gopedia의 chunking은 **L2(섹션 단위) -> L3(검색 단위)** 2계층으로 고정합니다.

1. **L2 section chunk**
   - Markdown TOC 기준으로 섹션을 비중첩(non-overlap) 분할
   - 문서 첫 헤더 이전 텍스트는 `root` 섹션으로 보존
2. **L3 retrieval chunk**
   - L2 내부를 섹션 타입별 규칙으로 세분화
   - 최종 검색/재정렬은 L3 기준으로 수행

### 2-2. 섹션 타입별 L3 분할 규칙

- **Heading/Ordered text**
  - 문장 분할 후 clause 단위(쉼표/세미콜론/파이프)로 추가 분해
  - 긴 문장/복합 문장 질의 대응을 위해 semantic fragment 확장 적용
- **Table**
  - 헤더/구분선 이후 데이터 row를 1행 1청크로 분할
  - 대형 테이블은 raw 전체 임베딩 금지, 핵심 row/컬럼 중심 인덱싱 적용
- **Code**
  - 코드 라인 기반 L3를 생성하고 부모-자식(parent_id) 구조를 유지
  - anchor line 우선 임베딩으로 검색 노이즈를 줄임
- **Image/기타**
  - 설명 텍스트를 단일 또는 소수 L3로 유지

### 2-3. Chunk 연결성과 순서

- L3는 `sort_order`와 `parent_id`를 유지해 복원 가능성을 보장
- 각 L2는 `title_id`(섹션 제목 L3) + 본문 L3 체인을 갖도록 정규화
- 목표: 검색은 원자 단위로, 복원은 계층 단위로 수행

### 2-4. Path-to-Context Injection (상시 적용)

모든 임베딩 입력에 canonical path를 prepend합니다.

- 예: `[Path: <project>/<category>/<file>#<h1>/<h2>]`
- 경로 주입은 recall을 높이되 토큰 증가를 막기 위해 축약 경로만 사용
- `Frontmatter 운영 메타 + 섹션 path + section_type`를 최소 공통 컨텍스트로 사용

### 2-5. 구현/운영 가이드

- 기본 정책: **L2 비중첩 + L3 미세 분할 + 검색 시 확장**
- overlap은 ingest 단계에서 고정 주입하지 않고, retrieval의 neighbor/context 조립에서 동적으로 보완
- chunk 단위 변경 시 `pipeline_version`과 함께 재인덱싱하여 품질 비교를 가능하게 유지

### 2-6. Sidecar 데이터 Chunking 규칙

Sidecar(`.schema.jsonld`, `.parquet`, `.csv`)도 markdown과 동일하게 **검색 가능한 chunk**로 변환합니다.

- **JSON-LD (semantic chunk)**
  - 기본 단위: `entity(@id)` 또는 `relation(triple)` 1개를 1청크로 생성
  - 필수 메타: `dataset_id`, `entity_id`, `@type`, `path`, `ontology_version`, `admin_id`
  - 대형 그래프는 커뮤니티/서브그래프 단위로 parent chunk를 추가 생성
- **Parquet/CSV (factual chunk)**
  - 기본 단위: 1 row 1청크, 대형 데이터셋은 핵심 컬럼 projection 후 row 요약 청크 생성
  - 숫자 질의 대응을 위해 원본 값은 factual store에 유지하고, 임베딩에는 요약 텍스트를 사용
  - 동일 스키마 데이터는 column-profile chunk(컬럼 의미/분포/단위)도 함께 생성
- **Markdown-Sidecar 결합 키**
  - `[[data-ref:<dataset_id>]]`를 기준으로 markdown chunk와 sidecar chunk를 연결
  - 응답 1차 단계에는 sidecar의 `l3_id/l2_id/l1_id` 또는 `dataset_id/entity_id`만 우선 전달
  - 추가 근거 요청 시에만 원본 row/엔티티 본문을 확장 조회

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