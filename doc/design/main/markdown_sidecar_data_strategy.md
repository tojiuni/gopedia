마크다운(Markdown)은 사람이 읽기에는 좋지만, 대규모 테이블이나 복잡한 Key-Value를 그대로 포함하면 RAG에서 의미 손실이 발생하기 쉽습니다. 특히 테이블은 행/열 관계가 텍스트 1차원으로 평탄화되면서 검색 품질이 저하됩니다.

구조화된 데이터를 별도 파일(Sidecar)로 분리하고 마크다운은 참조(Anchor)만 유지하는 전략은 대규모 환경에서 매우 유효합니다. 본 문서는 다음 원칙을 기준으로 설계합니다.

> **원칙:** JSON-LD는 의미 계층(semantic layer), Parquet/CSV는 사실 계층(factual layer)으로 분리한다.

-----

## 1. 저장 포맷 원칙 (Semantic vs Factual)

| 계층 | 목적 | 권장 포맷 | 비고 |
| :--- | :--- | :--- | :--- |
| **Semantic Layer** | 엔티티 의미, 관계, 타입, 단위 정의 | **JSON-LD** | Graph/TypeDB 연동에 최적 |
| **Factual Layer** | 대량 row, 수치, 이력, 집계 대상 값 | **Parquet / CSV / RDB** | 조회/집계/필터 성능 우수 |
| **Light Config** | 소규모 설정/사양 | **JSON / YAML** | 단순 관리용 |

핵심은 "테이블 전체를 JSON-LD로 치환"하는 것이 아니라, **테이블 본체는 factual layer**, **의미/스키마/관계는 JSON-LD**로 분리하는 것입니다.

-----

## 2. 분리 기준 (의사결정 체크리스트)

아래 질문으로 데이터셋마다 계층을 판정합니다.

1. 주 사용 목적이 관계 탐색인가, 값 조회/집계인가?
2. 데이터 규모가 큰가(예: 100k+ rows)?
3. 질문 패턴이 "누가 누구와 연결?"인가, "조건/범위/정렬?"인가?
4. 컬럼 의미/단위/온톨로지 정합성이 중요한가?
5. Graph 기반 pre-filter가 필요한가?

### 점수 규칙

- **JSON-LD +2**
  - 엔티티 간 관계가 핵심
  - `@id`, `@type`, 단위/정의의 보존이 중요
  - Graph/TypeDB 질의 비중이 높음
- **Parquet/CSV +2**
  - 대량 row 또는 시계열/로그 데이터
  - 필터/집계/정렬 중심 질의
  - 저장 효율과 스캔 성능이 중요

### 판정

- JSON-LD 점수 >= Parquet/CSV 점수 + 2: **JSON-LD 중심**
- Parquet/CSV 점수 >= JSON-LD 점수 + 2: **Factual 중심**
- 점수 차 <= 1: **이중 계층(권장)**
  - 본체: Parquet/CSV
  - 의미 메타: JSON-LD manifest

-----

## 3. Linking Strategy (Markdown Anchor)

마크다운에는 데이터 본문이 아니라 **ID 기반 참조만** 남깁니다.

```markdown
## 서버 하드웨어 사양
자세한 표 데이터는 [[data-ref:hw-spec-v3]]를 참고.
```

`data-ref`는 다음 자원으로 resolve됩니다.

- `hw-spec-v3.parquet` (factual)
- `hw-spec-v3.schema.jsonld` (semantic)
- 선택: `hw-spec-v3.profile.json` (품질/분포/신선도)

ID는 `.gopedia` config의 `project_machine_id`를 루트로 생성하여, 모든 sidecar 파일과 DB 레코드가 동일한 `dataset_id` 계열을 공유하도록 합니다.

-----

## 4. Ingest & Embedding 전략

정형 데이터는 "원본 그대로 임베딩"하지 않고, 검색 목적에 맞게 변환합니다.

### A. Textual Linearization (선택적 문장화)

- Row/Column을 질의 목적에 맞춰 요약 문장으로 변환
- 예:
  - Bad: `{"id":1,"status":"active"}`
  - Good: `Entity 1 has status active.`

### B. Entity-Centric + Column-wise Index

- 전 행 임베딩 대신 **핵심 컬럼/엔티티 단위 인덱싱** 우선
- 대용량 테이블은 summary sidecar를 따로 생성

### C. Hybrid Retrieval

1. 질문 분해(Query Decomposition)
2. Semantic 조건(관계/의미)은 JSON-LD/Graph pre-filter (항상 실행)
3. Factual 조건(수치/범위)은 SQL/Parquet pre-filter
4. 최종적으로 벡터 검색 + metadata-aware 재정렬(rerank) 결합

### D. Ontology 운영 책임

- JSON-LD `@context`, `@type` 규칙 변경 책임자는 `admin_id`입니다.
- 온톨로지 변경 시에는 버전(`ontology_version`)을 갱신하고, 하위 dataset 매핑 테이블을 함께 업데이트합니다.

-----

## 5. 운영 템플릿 (권장)

데이터셋 단위로 아래 파일 집합을 표준화합니다.

1. `{dataset_id}.md` - 사람 중심 문맥
2. `{dataset_id}.parquet` - 사실 데이터(SoT)
3. `{dataset_id}.schema.jsonld` - 의미/관계/타입
4. `{dataset_id}.profile.json` - row 수, null 비율, 갱신 시각

LLM에 컨텍스트를 전달할 때는 Parquet/CSV 원본을 그대로 넣지 않고, 필요한 row만 선별해 compact 텍스트 또는 JSONL로 변환해 전달합니다.

이 표준을 사용하면 마크다운은 가볍게 유지하면서도, RAG는 정형 데이터의 의미와 수치를 모두 안정적으로 활용할 수 있습니다.