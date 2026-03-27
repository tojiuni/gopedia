# Gopedia Design Docs

파이프라인·저장소·역할 분리 설계 문서 모음.

**PostgreSQL 스키마의 정본(canonical)** 은 [`core/ontology_so/postgres_ddl.sql`](../../core/ontology_so/postgres_ddl.sql) 이다. 컬럼·주석은 DDL과 아래 문서를 함께 본다(불일치 시 DDL 우선).

| 문서 | 내용 |
|------|------|
| [01-overview.md](01-overview.md) | Ingestion 앵커(`documents`)·`machine_id` 온톨로지·L1–L3·리비전 관계 |
| [02-idempotency-and-tuber.md](02-idempotency-and-tuber.md) | `l2_child_hash` / `l3_child_hash`, 멱등성, Tuber(`keyword_so`) vs 버전 레이어 |
| [03-summary-pipeline.md](03-summary-pipeline.md) | Bottom-up Summary(Map-Reduce), L2 요약 시 부모 헤더/L1 context |
| [04-python-nlp-worker.md](04-python-nlp-worker.md) | Python NLP Worker(gRPC Unary), 문장 분리 + NER(영어 우선) |
| [05-storage-and-payloads.md](05-storage-and-payloads.md) | PG(`documents`·L1–L3)·리비전 열거 제안·Qdrant/TypeDB 키 계약 |
| [06-data-flow.md](06-data-flow.md) | L2 헤더 계층·L3 시퀀스·TOC·Mermaid, End-to-end 흐름 |

## 식별자 한 줄 요약

- **`documents`**: 논리 문서 **한 개** = 인제스트 앵커. `id`(UUID)·`machine_id`(BIGINT, UNIQUE)가 전역 기준이다.
- **`machine_id`**: 온톨로지 전역에서 재사용되는 **안정적 정수 ID** 패턴. `documents`·`keyword_so`(Tuber)·향후 `user` 등 엔티티에 붙일 수 있다(같은 컬럼명이어도 **서로 다른 테이블·도메인**이다).
- **`knowledge_l1`**: 동일 `document_id`에 매달리는 **콘텐츠 리비전**(루트 스냅샷). 파이프라인 스냅샷은 `version_id` 등으로 구분한다. 상세·리비전 열거 제안은 [05-storage-and-payloads.md](05-storage-and-payloads.md).
- **RAG / 그래프 “문서 노드”**: 검색·TypeDB 질의에서는 **`l1_id`(UUID)** 를 문서 단위 노드로 쓴다 → 모든 적재된 L1을 대상으로 탐색 가능. `documents`는 **글로벌 대표·인제스트 기준선** 역할.

원본은 PostgreSQL(`BYTEA`/`TEXT`/`UUID`), 시맨틱 검색은 Qdrant, 관계·식별자는 TypeDB에 둔다.
