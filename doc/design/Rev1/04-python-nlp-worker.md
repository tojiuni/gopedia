# Python NLP Worker (gRPC, Unary)

L3 문장 분리와 NER(영어 우선)을 Python에서 수행하고, Go와는 **gRPC(Unary RPC)**로 연동한다.

## 1. 역할

- **L3 문장 분리**: L2 텍스트를 문장 단위로 분할. (영어만 우선: 단순 문장 분리.)
- **NER (Entity Extraction)**: 사람, 조직, 기술 스택 등 개체 추출 → TypeDB용 관계형 데이터 생성.
- **전처리**: 영어만 우선 시 — Contractions 확장, Lemmatization 등은 단계적으로 추가. 초기에는 **문장 분리 + NER**만 구현한다.

## 2. 통신

- **프로토콜**: gRPC + Protobuf. 기존 Go Protobuf와 호환되는 메시지 정의 사용.
- **RPC 방식**: **Unary RPC**로 시작. (데이터 양이 많아지면 나중에 Streaming RPC 검토.)
- **방향**: Go(Phloem) → 요청 전송 → Python Worker → 전처리 + NER 수행 → 응답(문장 목록, 엔티티 목록 등) 반환 → Go에서 Qdrant/TypeDB 저장.

## 3. 메시지 예시(개념)

- **Request**: `version_id`, `l1_id`, `l2_id`, `text`(L2 원문 또는 이미 문장 리스트).
- **Response**: `sentences[]`, `entities[]`(type, text, span 등). 필요 시 `keyword_ids[]`(Tuber와 연동된 Machine ID).

## 4. 구현 위치

- **Python**: 별도 서비스/패키지(예: `python/nlp_worker`). gRPC 서버 구현, 문장 분리(영어), NER(영어) 로직.
- **Go**: gRPC 클라이언트로 해당 서비스 호출 후, 반환된 L3 문장·엔티티를 PostgreSQL L3 행, Qdrant payload, TypeDB 관계 생성에 사용한다.

## 5. 언어

- **1단계**: 영어만. 단순 문장 분리 + NER.
- 한국어(형태소 분석, Kiwi/Kagome 등)는 이후 단계에서 추가한다.
