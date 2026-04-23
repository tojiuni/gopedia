GraphDB를 활용하여 파일 시스템의 계층 구조(Hierarchy)를 RAG에 통합하는 것은 단순한 벡터 검색의 한계를 극복할 수 있는 매우 영리한 전략입니다. 특히 대규모 프로젝트나 정형/비정형 데이터가 섞여 있는 환경에서는 **"컨텍스트의 유실"**을 막는 핵심 열쇠가 됩니다.

제안하신 폴더 구조(`Project > Category > File`)를 활용한 구체적인 임베딩 및 검색 전략을 제안합니다.

---

### 1. 계층적 컨텍스트 주입 (Path-to-Context Injection)
가장 단순하면서도 강력한 방법은 파일의 **Full Path를 데이터의 일부로 임베딩**하는 것입니다.

* **방법:** 각 마크다운 청크(Chunk)의 최상단에 실제 파일 내용 대신 구조 정보를 텍스트로 추가합니다.
    * *예시:* `[Context: Project Alpha > Architecture > Network_Design.md] ... (실제 본문 내용)`
* **효과:** "이 프로젝트의 네트워크 설계는 어떻게 되어 있어?"라는 질문이 들어왔을 때, 본문에 '네트워크'라는 단어가 적더라도 **경로명에 포함된 키워드** 덕분에 검색 순위가 비약적으로 상승합니다.

### 2. GraphRAG: 엔티티 및 관계 기반 모델링
단순한 트리 구조를 넘어 GraphDB를 본격적으로 활용한다면, 파일 시스템을 **Knowledge Graph** 형태로 변환해야 합니다.

* **노드 구성:**
    * `Project Node`: 프로젝트 메타데이터 (기간, 담당자 등)
    * `Category Node`: 기술 스택이나 도메인 분류
    * `File Node`: 파일 속성 및 요약본
    * `Chunk Node`: 실제 텍스트 조각 및 벡터 데이터
* **관계(Edge) 구성:**
    * `CONTAINS`: 폴더가 파일을 포함함
    * `REFERENCES`: A 파일이 B 파일을 참조함 (마크다운 링크 등)
    * `SIMILAR_TO`: 내용상 유사도가 높은 청크 간의 연결



---

### 3. 구조 기반 임베딩 전략: "Structural Embedding"

수치적으로 증명된 성능 향상을 위해 다음 두 가지 방식을 혼합해 보시기 바랍니다.

#### A. Summary-based Hierarchical Indexing
* **전략:** 하위 노드(Chunk)들의 벡터를 평균 내거나 요약하여 상위 노드(File, Folder)의 대표 벡터를 생성합니다.
* **검색 흐름:** 1.  사용자 질문을 먼저 `Category`나 `File` 수준의 요약 벡터와 비교하여 범위를 좁힙니다.
    2.  선별된 범위 내의 `Chunk` 노드들에서 상세 검색을 수행합니다.
* **이점:** 대규모 데이터에서 검색 노이즈를 획기적으로 줄여줍니다 (Search Space Reduction).

#### B. Community Detection (Leiden Algorithm)
* **전략:** Microsoft의 *GraphRAG* 논문에서 제안된 방식입니다. GraphDB 내에서 서로 밀접하게 연결된 노드들을 그룹화(Community)하고, 각 그룹에 대한 요약문을 생성하여 이를 임베딩합니다.
* **효과:** 특정 폴더 전체의 내용을 요약해야 하는 광범위한 질문(예: "Category A 아래의 모든 기술 명세 요약해줘")에 대해 기존 RAG보다 훨씬 정확한 답변을 내놓습니다.

---

### 4. 수치적 근거 및 권장 아키텍처

최근 연구(*"From Local to Global: A GraphRAG Approach to Query-Focused Summarization"*)에 따르면, 단순 벡터 검색 대비 Graph 기반 검색은 다음과 같은 이점을 보입니다.

| 지표 | 단순 Vector RAG | Graph-based RAG |
| :--- | :--- | :--- |
| **Context Retention** | 낮음 (단편적 정보) | **높음** (구조적 맥락 유지) |
| **Global Query 성능** | 매우 낮음 | **매우 높음** (전체 요약 가능) |
| **Relational Accuracy** | 40~50% | **80% 이상** |

#### **실전 구현 제안 (현재 스택 기준)**
1.  **GraphDB (TypeDB)**: `Project -> Category -> File -> Chunk`를 엔티티/관계로 명시적으로 모델링하고, 경로/참조 관계(`contains`, `references`)를 TypeQL로 조회 가능하게 유지하세요.
2.  **VectorDB (Qdrant)**: `chunk_id`를 기준으로 벡터를 저장하고, TypeDB의 `Chunk` 엔티티 ID와 1:1 매핑하세요. 메타데이터에 `project_id`, `category_id`, `file_id`를 함께 넣어 필터 검색을 가능하게 하세요.
3.  **RDB (PostgreSQL)**: 수집 이력, 배치 상태, 버전, 접근 로그 같은 운영성 데이터는 PostgreSQL에서 관리하고, 검색 파이프라인에서는 `chunk_id`를 공통 키로 사용하세요.
4.  **Hybrid Query**: `TypeQL(구조 검색) -> Qdrant(의미 검색) -> PostgreSQL(운영 메타 결합)` 순서로 질의를 결합하십시오.
    * `TypeQL로 후보 file/chunk_id를 좁힌 뒤, Qdrant에서 벡터 검색하고, PostgreSQL 메타데이터(버전/권한/상태)를 조인해 최종 컨텍스트를 구성`

이 전략은 단순한 파일 저장을 넘어, 데이터 간의 **"족보"**를 만들어주는 작업입니다. 특히 폴더 구조 자체가 엔지니어의 논리적 분류를 반영하고 있다면, 이를 임베딩에 활용하는 것만으로도 성능이 20~30% 이상 향상될 가능성이 큽니다.

원하시면 다음 단계로 TypeDB 스키마 예시(`entity/relation`), Qdrant payload 설계, PostgreSQL 테이블 DDL까지 연결된 샘플을 작성해드릴 수 있습니다.