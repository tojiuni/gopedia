# mem0 Chunking 방식 정리

## 기준 소스

- `/neunexus/mem0/embedchain/embedchain/chunkers/base_chunker.py`
- `/neunexus/mem0/embedchain/embedchain/chunkers/common_chunker.py`
- `/neunexus/mem0/embedchain/configs/chunker.yaml`
- `/neunexus/mem0/embedchain/tests/chunkers/test_base_chunker.py`
- `/neunexus/mem0/embedchain/tests/chunkers/test_chunkers.py`

## 1) 전체 구조

mem0(embedchain)의 chunking은 크게 2단으로 동작한다.

1. **Text Splitter로 텍스트 분할**
   - 기본적으로 `RecursiveCharacterTextSplitter`를 사용한다.
   - `chunk_size`, `chunk_overlap`, `length_function`으로 분할 규칙을 제어한다.
2. **BaseChunker에서 공통 후처리**
   - loader가 반환한 데이터(record)를 순회하며 chunk를 생성한다.
   - `chunk_id` 생성, 중복 제거, 최소 길이 필터, metadata 보강을 수행한다.

즉, "어떻게 자를지"는 splitter가 담당하고, "저장 가능한 청크로 정리"는 BaseChunker가 담당한다.

## 2) BaseChunker 동작 상세

`BaseChunker.create_chunks(loader, src, app_id, config, **kwargs)` 흐름:

1. `loader.load_data(src, **kwargs)` 실행
   - 반환 형식:
     - `data`: `[{ "content": ..., "meta_data": ... }, ...]`
     - `doc_id`: 원본 문서 ID
2. `doc_id`에 `app_id` prefix 적용
   - 예: `test_app--DocID`
3. 각 record에 대해 `get_chunks(content)` 호출
   - 기본 구현은 `text_splitter.split_text(content)`
4. 각 chunk마다 `chunk_id = sha256(chunk + url)` 생성
   - `url`이 metadata에 없으면 `src`를 fallback으로 사용
   - `app_id`가 있으면 chunk_id에도 prefix 적용
5. 중복/길이 필터
   - 같은 `chunk_id`는 1개만 유지
   - `len(chunk) >= min_chunk_size`인 청크만 유지
6. metadata 보강
   - `data_type`, `doc_id`를 metadata에 추가
7. 최종 반환
   - `documents`, `ids`, `metadatas`, `doc_id`

## 3) 기본 Chunker(CommonChunker) 규칙

`CommonChunker`는 `BaseChunker`를 상속하고 splitter를 다음과 같이 초기화한다.

- 기본값(설정 미전달):
  - `chunk_size=2000`
  - `chunk_overlap=0`
  - `length_function=len`
- 설정 전달 시:
  - 전달한 `ChunkerConfig` 값 그대로 적용

## 4) 설정 포인트

### YAML 예시

`/neunexus/mem0/embedchain/configs/chunker.yaml`

- `chunk_size: 100`
- `chunk_overlap: 20`
- `length_function: 'len'`

### 테스트에서 확인되는 클래스별 기본값 예시

`test_chunkers.py` 기준:

- `CommonChunker`: `chunk_size=2000`, `overlap=0`
- `TextChunker`: `chunk_size=300`, `overlap=0`
- `PdfFileChunker`: `chunk_size=1000`, `overlap=0`
- `WebPageChunker`: `chunk_size=2000`, `overlap=0`

즉, 데이터 소스 타입에 따라 기본 chunk 크기를 다르게 가져간다.

## 5) mem0 방식의 핵심 특징

1. **단순/일관된 분할 파이프라인**
   - splitter + BaseChunker 공통 후처리 구조로 확장성이 좋다.
2. **중복 제거 기준이 명확**
   - `sha256(chunk + url)` 기반 dedup.
3. **최소 길이 필터 지원**
   - 너무 짧은 chunk를 저장 단계에서 제외 가능.
4. **메타데이터 추적성 확보**
   - 모든 청크에 `data_type`, `doc_id`를 부여한다.
5. **소스별 튜닝 가능**
   - chunker 클래스별로 `chunk_size/overlap` 기본값을 분리한다.

## 6) 간단 예시

입력:

- `content`: 긴 문서 본문
- `meta_data.url`: `https://example.com/a`
- `app_id`: `test_app`
- `min_chunk_size`: `10`

처리:

1. splitter가 문서를 여러 청크로 분할
2. 각 청크별 `sha256(chunk + "https://example.com/a")` ID 생성
3. 길이 10 미만 청크 제거
4. metadata에 `data_type`, `doc_id=test_app--<원본doc_id>` 추가

결과:

- `documents`: 필터 후 청크 목록
- `ids`: 안정적인 chunk ID 목록
- `metadatas`: 보강된 metadata 목록
- `doc_id`: app prefix가 반영된 문서 ID
