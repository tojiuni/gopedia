본 문서는 `markdown_strategy.md`의 3단계 운영 흐름을 **실행 관점으로 풀어쓴 상세 가이드**입니다.
공통 정책(계층 분리, ID/링킹 표준, 오케스트레이션)은 중복 서술하지 않고 기준 문서를 참조합니다.

- 기준 문서: `markdown_strategy.md`
- sidecar 상세: `markdown_sidecar_data_strategy.md`
- 계층/그래프 검색 상세: `hierarchy_embedding_strategy.md`

---

### 1단계: 파일 작성 (Sidecar Data & Markdown)
마크다운은 본문 맥락, sidecar는 구조/사실 데이터를 담당합니다.

세부 규칙은 기준 문서를 따릅니다.

- 공통 정책: `markdown_strategy.md`의 "고정 원칙"
- sidecar 포맷/링킹: `markdown_sidecar_data_strategy.md`의 "저장 포맷 원칙", "Linking Strategy"

---

### 2단계: Ingest (Polyglot Persistence 전략)
데이터 성격에 따라 저장소를 분리하고, 공통 ID로 동기화합니다.

* **RDB (The Source of Truth):** 정규화된 데이터, 수치, 상태 정보 등을 저장합니다. '정확한 값'에 대한 쿼리가 필요할 때 사용합니다. (예: "가격이 100만 원 이하인 제품 목록")
* **GraphDB (The Context Map):** 문서 간의 참조 관계, 카테고리 계층, 엔티티 간의 논리적 연결을 저장합니다. (예: "A 기능을 사용하는 B 모듈의 설계서 찾기")
* **NoSQL / VectorDB (The Semantic Store):** 마크다운의 텍스트 청크를 저장합니다. 의미적 유사성 기반의 검색을 담당합니다.
* **ID 동기화 규칙:** 공통 ID 규칙은 `markdown_strategy.md`를 따릅니다.



---

### 3단계: Embedding (특성 활용 전략)
단순한 텍스트 임베딩을 넘어, 각 DB의 강점을 활용한 **'하이브리드 인덱싱'**을 수행해야 합니다.

* **Graph-Augmented Embedding:** 텍스트를 임베딩할 때, 해당 텍스트가 속한 GraphDB의 경로(Path) 정보를 텍스트 앞에 덧붙입니다(상시 적용).
    * *예:* `[Path: Project > Architecture > Database] (본문 내용)`
    * 이렇게 하면 벡터 검색 시 구조적 맥락이 함께 계산되어 검색 품질이 비약적으로 향상됩니다.
* **Metadata Filtering (RDB 연동):** VectorDB에 임베딩을 저장할 때, RDB에 있는 핵심 속성들을 **Metadata Payload**로 함께 넣습니다. 이를 통해 '유사도 검색'을 수행하기 전, 'RDB 조건 필터링'을 먼저 실행하여 검색 대상 범위를 좁힙니다 (Pre-filtering).
* **Table/Sidecar 임베딩 규칙:** 상세 chunking/전처리는 `markdown_sidecar_data_strategy.md`를 따릅니다.
* **Entity Centric Indexing:** 엔티티 중심 인덱싱 규칙은 기준 문서의 embedding 정책을 따릅니다.
* **Parent 전달 최소화:** 기본 응답에는 `l3_id`, `l2_id`, `l1_id`와 최소 스니펫만 전달하고, 추가 질문 시에만 parent 본문을 확장 조회합니다.

---

### 종합 제안: "검색-연결-생성" 파이프라인

가장 효과적인 운영 전략은 **'Query Decomposition(질문 분해)'**을 도입하는 것입니다.

1.  **질문 분석:** 사용자의 질문이 들어오면 LLM이 이를 분석하여 `RDB용 조건`, `Graph용 경로`, `Vector용 의미`로 나눕니다.
2.  **고정 오케스트레이션:**
    * GraphDB(TypeQL) prefilter (항상 실행)
    * RDB/Parquet factual prefilter
    * VectorDB 유사도 검색
    * Metadata-aware rerank
3.  **컨텍스트 통합:** 추출된 모든 정보를 하나의 프롬프트로 재구성하여 LLM에 전달합니다.

이 전략으로 진행하면, 데이터가 방대해지더라도 **"구조를 아는 검색"**이 가능해져 할루시네이션(환각 현상)을 최소화하고 매우 정교한 RAG 시스템을 구축할 수 있습니다. 설계하신 프로세스는 확장이 용이한 구조이므로, 초기부터 **데이터 간의 ID 체계**만 엄격히 관리하신다면 매우 강력한 엔진이 될 것입니다.

---

### 빠른 판정 기준 (실무용)

- 관계/의미 탐색 중심이면: **JSON-LD 비중 확대**
- 수치/범위/정렬/집계 중심이면: **Parquet/CSV 또는 RDB 비중 확대**
- 둘 다 중요하면: **이중 계층(semantic + factual) 유지**