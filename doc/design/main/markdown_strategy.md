대규모 마크다운(Markdown) 기반 데이터로 RAG(Retrieval-Augmented Generation) 시스템을 구축할 때, 성능을 결정짓는 핵심 요소는 **데이터의 구조화(Structure)**, **컨텍스트를 보존하는 청킹(Chunking)**, 그리고 **고해상도 검색(Retrieval)** 전략입니다.

수치적 근거와 최신 논문 기술을 바탕으로 제안하는 아키텍처 전략은 다음과 같습니다.

---

## 1. 데이터 저장 방식: 구조화된 Markdown 관리
대규모 데이터셋에서는 단순히 파일을 모으는 것이 아니라, 검색 효율을 높이기 위한 **메타데이터 중심의 저장**이 필요합니다.

* **Frontmatter 활용:** 모든 마크다운 파일 상단에 YAML Frontmatter를 포함하여 주제, 작성일, 권한, 키워드 등을 정의합니다. 이는 검색 시 **Metadata Filtering**으로 활용되어 검색 범위를 획기적으로 줄여줍니다.
* **Hierarchical Structure (계층 구조):** 파일 시스템 자체가 논리적 카테고리를 반영하도록 구성합니다.
* **Version Control:** 데이터의 변경 이력을 추적하기 위해 Git 기반 혹은 DVC(Data Version Control)를 병행하는 것이 좋습니다.

---

## 2. Ingestion 방식: 구조 인식형 청킹 (Structure-Aware Chunking)
일반적인 고정 길이 청킹(Fixed-size chunking)은 마크다운의 논리적 흐름을 깨뜨려 성능을 저하시킵니다.

### 제안 전략: MarkdownHeaderTextSplitter + Recursive Character Splitting
1.  **Header 기반 분할:** `#`, `##`, `###` 등 마크다운 헤더를 기준으로 논리적 단락을 먼저 나눕니다.
2.  **재귀적 분할:** 헤더 내 내용이 너무 길 경우, `\n\n`, `\n`, ` ` 순서로 재귀적으로 분할하여 의미적 연속성을 유지합니다.
3.  **Overlap 설정:** 인접 청크 간의 컨텍스트 단절을 막기 위해 약 **10~20%의 중첩(Overlap)**을 허용합니다.



---

## 3. Embedding 전략: 수치로 증명된 고성능 기법
단순한 벡터 변환을 넘어, 검색 정확도를 극대화하기 위한 전략입니다.

### A. Parent Document Retrieval (PDR)
* **개념:** 검색은 아주 작은 단위(Small Chunk)로 수행하여 유사도를 높이고, LLM에게 전달할 때는 해당 청크가 포함된 상위 문서(Parent/Large Chunk) 전체를 전달하는 방식입니다.
* **효과:** 검색의 정밀도(Precision)와 생성의 문맥(Context)을 동시에 잡을 수 있습니다.

### B. Embedding 모델 선택 및 차원 최적화
* **MTEB(Massive Text Embedding Benchmark)** 기준 상위 모델을 선택하십시오. 현재 **BGE-M3**나 **OpenAI의 text-embedding-3-large**가 범용적으로 우수한 성능을 보입니다.
* **Late Interaction (ColBERT):** 단순 코사인 유사도가 아닌, 토큰 레벨의 상호작용을 계산하는 ColBERT 방식을 Re-ranking 단계에 도입하면 NDCG(검색 정확도 지표)가 대폭 향상됩니다.

### C. 유사도 계산 공식
벡터 공간에서의 유사도는 일반적으로 **Cosine Similarity**를 사용합니다. 두 벡터 $A, B$에 대하여:
$$\text{similarity} = \cos(\theta) = \frac{A \cdot B}{\|A\| \|B\|}$$

---

## 4. 논문 및 자료 기반의 성능 최적화 제안

### 1) "Lost in the Middle" 현상 방지
논문 *["Lost in the Middle: How Language Models Use Long Context"]* 에 따르면, LLM은 컨텍스트의 처음과 끝 정보는 잘 파악하지만 중간 정보는 놓치는 경향이 있습니다. 이를 해결하기 위해 **Re-ranking** 단계를 반드시 추가하여 가장 관련성 높은 청크를 컨텍스트의 최상단에 배치해야 합니다.

### 2) RAPTOR (Recursive Abstractive Processing for Tree-Organized Retrieval)
최신 연구인 *RAPTOR* 방식은 대규모 데이터를 트리 구조로 요약하여 저장합니다. 
* **방법:** 하위 청크들을 클러스터링하고 요약(Summarization)하여 상위 노드를 만듭니다.
* **이점:** 질문이 광범위할 경우(예: "이 프로젝트의 전반적인 아키텍처는?") 요약된 상위 노드를 검색하여 답변 성능을 높입니다.

---

## 5. 최종 권장 파이프라인 요약

| 단계 | 권장 기술 스택 / 전략 | 비고 |
| :--- | :--- | :--- |
| **Storage** | S3 (Object Storage) + Vector DB (Pinecone, Milvus, Weaviate) | Metadata Filtering 필수 |
| **Ingestion** | MarkdownHeaderTextSplitter | 구조적 의미 보존 |
| **Embedding** | BGE-M3 or text-embedding-3-large | MTEB 벤치마크 기반 |
| **Retrieval** | Parent Document Retrieval + Hybrid Search | 키워드(BM25) + 벡터 검색 결합 |
| **Post-Process** | Cohere Re-rank or BGE-Reranker | 검색 결과 재정렬로 정확도 향상 |

이 방식은 대규모 데이터 환경에서 검색 노이즈를 최소화하고, LLM이 가장 정확한 정보에 접근할 수 있도록 설계된 검증된 구조입니다. 구현 시 **Ragas**나 **TruLens** 같은 프레임워크를 활용해 실제 Retrieval 성능(Faithfulness, Relevancy)을 수치화하며 튜닝하는 것을 권장합니다.