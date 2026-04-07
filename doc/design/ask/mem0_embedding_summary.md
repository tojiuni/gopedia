# mem0 Embedding 관련 정리

## 기준 소스 (핵심)

- `/neunexus/mem0/mem0/embeddings/base.py`
- `/neunexus/mem0/mem0/configs/embeddings/base.py`
- `/neunexus/mem0/mem0/embeddings/configs.py`
- `/neunexus/mem0/mem0/utils/factory.py`
- `/neunexus/mem0/mem0/memory/main.py`
- `/neunexus/mem0/mem0/embeddings/*.py` (provider별 구현)
- `/neunexus/mem0/tests/embeddings/*.py` (동작 검증)
- `/neunexus/mem0/openmemory/api/default_config.json` (OpenMemory 기본값)

## 1) 결론: embedding 관련 내용 있음

mem0에는 embedding 추상화/설정/팩토리/실행 경로가 분리되어 있고, 여러 provider를 교체 가능하도록 설계되어 있다.

- 추상화: `EmbeddingBase`
- 공통 설정: `BaseEmbedderConfig`
- provider 선택: `EmbedderFactory`
- 실제 사용: `Memory` 클래스에서 add/search/update 시 `embed()` 호출

## 2) 아키텍처 흐름

1. `MemoryConfig.embedder`에 provider/config를 지정
2. `Memory.__init__`에서 `EmbedderFactory.create(...)`로 embedder 인스턴스 생성
3. 메모리 라이프사이클에서 action별 임베딩 호출
   - add: `embed(text, "add")`
   - search: `embed(query, "search")`
   - update: `embed(text, "update")`
4. 결과 벡터를 vector store에 insert/search/update로 전달

### Flow 다이어그램

```mermaid
flowchart TD
    A[사용자/앱 요청] --> B[MemoryConfig 로드]
    B --> C[EmbedderFactory.create(provider, config)]
    C --> D[Provider별 Embedder 인스턴스]
    D --> E{memory_action}
    E -->|add| F[embed(text, add)]
    E -->|search| G[embed(query, search)]
    E -->|update| H[embed(text, update)]
    F --> I[VectorStore.insert]
    G --> J[VectorStore.search]
    H --> K[VectorStore.update]
```

### Flow를 코드 함수 기준으로 보면

- 초기화:
  - `Memory.__init__` -> `EmbedderFactory.create(...)`
- 추가(add):
  - `_add_to_vector_store` -> `self.embedding_model.embed(msg_content, "add")`
  - `_create_memory` -> `vector_store.insert(...)`
- 검색(search):
  - `_search_vector_store` -> `self.embedding_model.embed(query, "search")`
  - `vector_store.search(query=query, vectors=embeddings, ...)`
- 수정(update):
  - `_update_memory` -> `self.embedding_model.embed(data, "update")`
  - `vector_store.update(..., vector=embeddings, ...)`

## 3) 지원 provider (Python mem0 코어)

`mem0/utils/factory.py` 기준:

- `openai`
- `ollama`
- `huggingface`
- `azure_openai`
- `gemini`
- `vertexai`
- `together`
- `lmstudio`
- `langchain`
- `aws_bedrock`
- `fastembed`

## 4) provider별 기본값/특징 요약

### OpenAI

- 기본 모델: `text-embedding-3-small`
- 기본 차원: `1536`
- `embedding_dims`를 사용자가 명시했을 때만 API에 `dimensions` 전달
- `encoding_format="float"` 고정 사용 (프록시 호환성 고려)

### Ollama

- 기본 모델: `nomic-embed-text`
- 기본 차원: `512`
- 로컬 모델 존재 확인 후 없으면 자동 pull

### HuggingFace

- 기본 모델: `multi-qa-MiniLM-L6-cos-v1`
- 로컬 `SentenceTransformer` 또는 `huggingface_base_url` 경유 OpenAI-compatible endpoint 모두 지원
- 차원은 모델에서 자동 추론 가능

### Azure OpenAI

- `AzureOpenAI` client 사용
- API key 없으면 `DefaultAzureCredential` 경로 지원

### Vertex AI

- 기본 모델: `gemini-embedding-001`
- 기본 차원: `256`
- `memory_action`에 따라 task type 분리 가능
  - add/update: `RETRIEVAL_DOCUMENT`
  - search: `RETRIEVAL_QUERY`

### Gemini (Google GenAI)

- 기본 모델: `models/gemini-embedding-001`
- 기본 차원: `768` (또는 `output_dimensionality`)

### LM Studio

- OpenAI client를 local base URL로 사용
- 기본 base URL: `http://localhost:1234/v1`

### Together

- 기본 모델: `togethercomputer/m2-bert-80M-8k-retrieval`
- 기본 차원: `768`

### AWS Bedrock

- 기본 모델: `amazon.titan-embed-text-v1`
- provider별 요청 포맷 처리(cohere vs 기타)

### FastEmbed

- 기본 모델: `thenlper/gte-large`
- ONNX runtime 기반 임베딩

### Langchain

- `config.model`에 LangChain `Embeddings` 인스턴스를 직접 주입하는 방식

## 5) Memory(main.py)에서 실제 쓰는 방식

핵심 포인트:

- add/update/search 모두 `self.embedding_model.embed(...)` 호출
- 동일 텍스트 임베딩 재사용을 위한 캐시 dict를 일부 경로에서 사용
  - 중복 API 호출을 줄이려는 의도
- `_create_memory`, `_update_memory`, `_search_vector_store`에서 벡터 store와 결합됨

### 실행 흐름 (step-by-step)

1. **요청 수신**
   - `add`, `search`, `update` API/메서드가 호출된다.
2. **텍스트 정규화 및 임베딩 생성**
   - provider 구현체에서 줄바꿈 제거/포맷 정리 후 임베딩 API 호출.
   - 예: OpenAI/FastEmbed/LMStudio는 `text.replace("\n", " ")` 처리.
3. **action별 임베딩 타입 적용 (provider 특화)**
   - VertexAI는 `memory_action`에 따라 `RETRIEVAL_DOCUMENT`/`RETRIEVAL_QUERY` task type을 바꿔 호출.
4. **벡터 스토어 연산**
   - add -> `insert`, search -> `search`, update -> `update`.
5. **재사용 최적화**
   - 일부 경로에서 동일 텍스트 임베딩을 dict로 캐시해 중복 호출을 줄인다.

### mem0 embedding 방법의 성격

- **추상화 중심**: provider 차이를 `EmbeddingBase.embed()` 인터페이스로 통일.
- **설정 주도**: provider/model/api key/base URL/dims를 config로 교체 가능.
- **액션 인지형**: `memory_action`을 전달해 add/search/update 문맥을 provider가 활용 가능.
- **스토어 결합형**: 임베딩은 단독 기능이 아니라 vector store CRUD 흐름 안에 포함됨.

## 6) 설정 키 (BaseEmbedderConfig)

주요 키:

- 공통: `model`, `api_key`, `embedding_dims`
- OpenAI: `openai_base_url`
- Ollama: `ollama_base_url`
- HuggingFace: `model_kwargs`, `huggingface_base_url`
- Azure: `azure_kwargs`, `http_client_proxies`
- Vertex: `vertex_credentials_json`, `memory_*_embedding_type`
- Gemini: `output_dimensionality`
- LM Studio: `lmstudio_base_url`
- AWS: `aws_access_key_id`, `aws_secret_access_key`, `aws_region`

## 7) OpenMemory 기본 설정 (참고)

`/neunexus/mem0/openmemory/api/default_config.json` 기준:

- embedder provider: `openai`
- embedder model: `text-embedding-3-small`
- api_key: `env:OPENAI_API_KEY`

즉 OpenMemory 기본 배포도 OpenAI embedding을 기본값으로 사용한다.

## 8) 추가 참고: mem0-ts

TypeScript OSS 버전에도 embedder factory가 있고 provider 교체를 지원한다.

- 파일: `/neunexus/mem0/mem0-ts/src/oss/src/utils/factory.ts`
- provider 예: `openai`, `ollama`, `lmstudio`, `google/gemini`, `azure_openai`, `langchain`
- 기본 설정: `/neunexus/mem0/mem0-ts/src/oss/src/config/defaults.ts`에서 OpenAI embedder 기본값 사용
