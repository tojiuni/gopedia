# Gopedia Data Pipeline — Overview

전체 파이프라인은 **Ingestion → Processing → Storage** 3단계로 구성한다.

## Ingestion 앵커 — `documents`

- **논리 문서 한 개**는 항상 **`documents`** 테이블 한 행으로 고정한다.
- **`documents.id`**: PostgreSQL·API 응답의 **`doc_id`** 등과 맞추는 **캐논 UUID**. Qdrant 벡터 payload에는 넣지 않으며, 검색·필터 스코프는 **`l1_id`** 를 쓴다.
- **`documents.machine_id`**: `identity_so`에서 오는 **BIGINT**, UNIQUE — 인제스트·외부 시스템과의 **전역 대표 키**(같은 슬롯의 기준 문서).
- **`knowledge_l1`**: 같은 논리 문서에 대해 **여러 리비전**(스냅샷)이 있을 수 있으며, 각 행은 `document_id → documents.id` 로 앵커에 묶인다. 리비전 구분·열거 규칙은 [05-storage-and-payloads.md](05-storage-and-payloads.md) § 1.3 참고.

## `machine_id` (온톨로지)

- **`machine_id`** 는 **문서(`documents`)만**의 전용 개념이 아니다. **`keyword_so`**, 향후 **user** 등, 온톨로지 상 여러 엔티티에서 **동일한 “안정 정수 ID” 패턴**으로 쓸 수 있다.
- 테이블마다 독립된 이름공간으로 할당하거나, 글로벌 풀을 쓰든 **설계 시 충돌만 막으면** 된다. 문서와 키워드의 `machine_id`는 **서로 다른 엔티티**이다.

## 단계 요약

| 단계 | 담당 | 핵심 역할 |
|------|------|-----------|
| **Ingestion** | Go (Phloem) | `documents` 앵커 확보 → `knowledge_l1` 리비전·L2/L3, `l2_child_hash`/`l3_child_hash` 선별, Tuber |
| **Processing** | Go + Python | L2 요약(Map-Reduce), L3 문장 분리 + NER, 전처리(영어 우선) |
| **Storage** | PostgreSQL, Qdrant, TypeDB, Redis | 원본(PG UUID·BYTEA), 벡터·필터(Qdrant), 관계·**`l1_id` 문서 노드**(TypeDB), Redis(Tuber) |

## 계층 정의

- **`documents`**: 논리 문서 앵커 — `machine_id`, `title`, `source_type` 등(본문 해시 컬럼 없음; 멱등은 `knowledge_l1.l2_child_hash` 등으로 판별).
- **L1 (`knowledge_l1`)**: 해당 앵커에 종속된 **루트 스냅샷**(리비전). Explorer/폴더 트리는 `parent_id` 등 단일 테이블 모델 + `source_type` 유지.
- **L2**: 섹션/헤더 단위.
- **L3**: 문장 등 원자 단위.

## RAG / 그래프에서의 “문서 노드”

- **벡터 검색·그래프 질의(RQG)** 에서 문서 단위로 묶어 찾을 때는 **`l1_id`** 를 사용한다. 모든 적재된 `knowledge_l1` 행이 각각 후보가 되어, **코퍼스 전체 L1**을 대상으로 필터·조인할 수 있다.
- **`documents`** 는 **인제스트·대표 메타(글로벌 기준)** 에 가깝고, 반드시 “그래프상 유일 문서 노드”와 1:1로 대응할 필요는 없다(리비전이 여러 L1이면 L1이 여러 개).

## 상세 설계 문서

- [02-idempotency-and-tuber.md](02-idempotency-and-tuber.md) — L2/L3 계층형 해시 멱등성, Tuber vs 버전 관리 분리
- [03-summary-pipeline.md](03-summary-pipeline.md) — Bottom-up Summary (Map-Reduce, L2 context)
- [04-python-nlp-worker.md](04-python-nlp-worker.md) — Python NLP Worker(gRPC, unary), 문장 분리 + NER
- [05-storage-and-payloads.md](05-storage-and-payloads.md) — PG/Qdrant/TypeDB/Redis 역할, Payload·속성, 리비전 열거 제안
- [06-data-flow.md](06-data-flow.md) — End-to-end 데이터 흐름
