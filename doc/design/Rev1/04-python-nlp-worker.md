# Python NLP Worker (gRPC, Unary)

L3 문장 분리(및 향후 NER)를 Python에서 수행하고, Go(Phloem)와는 **gRPC(Unary RPC)**로 연동한다.

## 1. 역할

- **L3 문장 분리**: L2 텍스트를 문장 단위로 분할한다.
- **마스킹 보강**: 마크다운 링크, URL, 시맨틱 버전(`v1.3`), 절 번호(`4.1.3~4.1.5`), 일반적인 `*.md` 등 **경로/버전** 구간을 먼저 플레이스홀더로 치환한 뒤 문장 분리하여, 내부의 `.`가 문장 경계로 오인되지 않게 한다. 규칙은 Go 폴백 [`internal/phloem/sink/splitmask.go`](../../../internal/phloem/sink/splitmask.go)와 맞춘다.
- **언어별 분리기**: 한글 비율이 높으면 **kss** `split_sentences`, 그렇지 않으면 **pysbd**(영어 규칙 기반). `kss` 미설치 시에는 CJK/라틴 구두점 기준의 정규식 폴백을 사용한다. 임계값은 환경변수 `GOPEDIA_NLP_LANG_HANGUL_RATIO`(기본 `0.12`)로 조정 가능하다.
- **NER (Entity Extraction)**: 이후 단계 — 사람, 조직, 기술 스택 등 → TypeDB용 관계형 데이터.

## 2. 통신

- **프로토콜**: gRPC + Protobuf. 기존 Go Protobuf와 호환되는 메시지 정의 사용.
- **RPC 방식**: **Unary RPC**로 시작. (데이터 양이 많아지면 나중에 Streaming RPC 검토.)
- **방향**: Go(Phloem) → 요청 전송 → Python Worker → 마스킹 + 문장 분리 → 응답(`sentences[]`) 반환 → Go에서 PostgreSQL L3, Qdrant 등에 반영.
- **환경**: Phloem은 `GOPEDIA_NLP_WORKER_GRPC_ADDR`가 설정되어 있고 호출에 성공할 때 워커 결과를 쓰고, 실패 시 Go 쪽 `splitSentencesEnglish`(동일 마스킹 + 구두점 분리)로 폴백한다.

## 3. 메시지 예시(개념)

- **Request**: `version_id`, `l1_id`, `l2_id`, `text`(L2 본문).
- **Response**: `sentences[]`, `entities[]`(type, text, span 등). 필요 시 `keyword_ids[]`(Tuber와 연동된 Machine ID).

## 4. 구현 위치

- **Python**: [`python/nlp_worker/`](../../../python/nlp_worker/) — `mask.py`, `split_text.py`, `server.py`. 의존성은 [`python/nlp_worker/requirements.txt`](../../../python/nlp_worker/requirements.txt)(`grpcio`, `kss`, `pysbd`).
- **Go**: gRPC 클라이언트 [`internal/phloem/nlpworker/client.go`](../../../internal/phloem/nlpworker/client.go); 폴백 문장 분리는 [`internal/phloem/sink/writer.go`](../../../internal/phloem/sink/writer.go)의 `splitSentencesEnglish` + `splitmask.go`.

## 5. 언어

- **한국어·영어 혼합 코퍼스**: 한글 비율에 따라 kss / pysbd 분기.
- **테스트**: 저장소의 [`tests/test_nlp_worker_mask.py`](../../../tests/test_nlp_worker_mask.py) 및 [`tests/fixtures/nlp_skill_sample.md`](../../../tests/fixtures/nlp_skill_sample.md); 선택적으로 `GOPEDIA_NLP_TEST_SKILL_PATH`로 외부 SKILL.md 경로 지정.

## 6. 실행

- 워커 기동: 저장소 루트에서 `python python/nlp_worker/server.py` (또는 `GOPEDIA_NLP_WORKER_ADDR`로 바인드 주소 지정, 기본 `0.0.0.0:50052`).
- Phloem: `GOPEDIA_NLP_WORKER_GRPC_ADDR=host:50052` (또는 Docker 네트워크 상의 서비스명).

## 7. 구조화 L2와의 관계

Phloem은 마크다운 전처리로 **추가 L2 청크**(`o*` ordered list, `t*` table, `c*` code, `i*` image)를 만들 수 있다. **NLP `ProcessL2` 호출**은 `heading` / `ordered` 타입 L2에 대해서만 수행하고, `table` / `code` / `image`는 로컬 분할(표는 데이터 행 단위, 코드·이미지는 단일 블록)만 사용한다. Qdrant payload의 `section_type`은 해당 L2의 블록 타입과 일치한다.
