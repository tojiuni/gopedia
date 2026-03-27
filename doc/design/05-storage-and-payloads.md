# Storage & Payload 정의

데이터 저장소별 역할과, PostgreSQL 스키마(`documents`·L1–L3·`keyword_so` 등), Qdrant Payload·TypeDB 속성·Redis 사용 방식을 정리한다.

**정본**: 테이블·컬럼 정의는 [`core/ontology_so/postgres_ddl.sql`](../../core/ontology_so/postgres_ddl.sql) 이 우선한다. 아래 표는 의미·역할 요약이다.

## 0. 식별자 — `documents` 앵커와 `machine_id`

| 개념 | 역할 |
|------|------|
| **`documents`** | **인제스트 앵커**이자 **논리 문서 한 개**의 기준. `id`(UUID)·`machine_id`(BIGINT UNIQUE)·`title`·`version`/`version_id`·`source_type` 등. 본문 전체 해시 컬럼은 두지 않는다(멱등은 `knowledge_l1.l2_child_hash` 등). |
| **`machine_id`** | 온톨로지에서 재사용 가능한 **안정 정수 ID** 패턴. `documents`·`keyword_so`·향후 `user` 등 **서로 다른 엔티티**에 붙일 수 있다(의미는 엔티티별로 다름). |
| **`knowledge_l1`** | `document_id → documents.id` 로 묶인 **루트 스냅샷(리비전)**. 동일 앵커에 여러 L1 행이 있으면 리비전 후보. |
| **RAG / 그래프 “문서 노드”** | TypeDB·필터링에서는 **`l1_id`(UUID)** 를 문서 단위 노드로 사용한다 → **모든 L1**을 코퍼스 대상으로 질의할 수 있다. `documents`는 **글로벌 대표·인제스트 기준**에 가깝다. |

---

## 1. PostgreSQL (원본·메타)

- **역할**: **원본 데이터** 및 계층 메타 저장. 식별자는 **`UUID`** 기반으로 통일한다.
- **데이터 타입** (정책):
  - 요약·본문 등은 설계에 따라 **`BYTEA`** 또는 **`TEXT`** — DDL(`core/ontology_so/postgres_ddl.sql`)을 기준으로 한다.
  - **통합 해시**(`l2_child_hash`, `l3_child_hash` 등): SHA256 바이너리(**`BYTEA`**, 32 bytes 권장).

### 1.1 `documents` (Ingestion 앵커)

| 개념 | 설명 |
|------|------|
| `id` | `uuid` PK — API·클라이언트 `doc_id` 등과 맞추는 캐논 문서 ID(Qdrant payload에는 미포함) |
| `machine_id` | BIGINT UNIQUE — identity_so·전역 대표 키 |
| `title` | 문서 제목 |
| `version` / `version_id` | 문서 행 단위 버전·파이프라인 스냅샷 |
| `source_type` | `md` 등 |
| `source_metadata` | `jsonb` — 소스별 확장 메타 |
| **`current_l1_id`** | **(구현됨, 방식 C)** nullable FK → `knowledge_l1(id)` — 서비스·동기화가 사용하는 **현재 리비전 헤드**. NULL이면 `knowledge_l1`에서 `document_id` 기준 최신 `created_at` 행으로 해석할 수 있다. (`ALTER TABLE` 로 `documents` 테이블에 추가됨, DDL 하단 참고.) |

### 1.2 `knowledge_l1` (리비전 루트 — 단일 테이블)

**폴더·문서·Git·티켓 등** 논리 루트를 **한 테이블**로 관리한다. **`document_id`** 로 **`documents`** 에 종속된다.

| 개념 | 설명 |
|------|------|
| `id` | `uuid` PK — **RAG/TypeDB에서의 “문서 노드” ID (`l1_id`)** |
| `document_id` | `uuid` NOT NULL FK → **`documents(id)`** — 논리 문서 앵커 |
| (no `machine_id` on L1) | 문서 식별은 **`documents.machine_id`** 만 사용 |
| `parent_id` | `uuid` FK → `knowledge_l1(id)`, nullable — Explorer 트리 |
| `source_type` | 소스 구분 (`md`, `folder`, …) |
| `version_id` | `pipeline_version` 등 — **리비전·파이프라인 스냅샷** 구분에 사용 |
| `toc` | `jsonb` — 경량 목차 |
| `l2_child_hash` | 하위 L2 스켈레톤 통합 해시 |
| `summary` / `summary_hash` | 루트 요약(바이트)·보조 해시 |
| `created_at` / `modified_at` | 리비전 시각 판단에 유용 |

### 1.3 리비전 열거 (채택: **C** + 폴백)

- **활성 리비전(헤드)**: `documents.current_l1_id` 가 가리키는 `knowledge_l1` 행. Phloem 인제스트는 새 L1 삽입 후 **`UPDATE documents SET current_l1_id = …`** 로 갱신한다.
- **폴백**: `current_l1_id IS NULL` 이면 `WHERE document_id = … ORDER BY created_at DESC LIMIT 1` 로 최신 L1을 헤드로 본다(TypeDB 동기화·멱등 스킵 조회 등에서 동일 규칙).
- **히스토리 전체**: `SELECT * FROM knowledge_l1 WHERE document_id = $doc_id ORDER BY created_at DESC` (또는 `version_id` 포함 정렬)로 모든 스냅샷 나열.
- **D(연쇄)** 등은 필요 시 이후 확장.

### 1.4 `knowledge_l2` (Skeleton)

| 개념 | 설명 |
|------|------|
| `id` | `uuid` PK |
| `l1_id` | `uuid` NOT NULL FK → `knowledge_l1` |
| `parent_id` | 상위 L2 (헤더 트리) |
| `section_id` | TOC/청크 식별자 |
| **`title_id`** | (구현) nullable FK → `knowledge_l3(id)` — 해당 섹션의 **원문 마크다운 헤더 한 줄**(`#` …)을 담은 L3 행(`sort_order = 0`). 마크다운 전체 복원·RAG 헤더 조회에 사용. |
| `summary` | 섹션 요약 텍스트 |
| `summary_bin` | 요약 등 보조 바이너리(구현 정책에 따름) |
| `summary_hash` | 섹션 본문(`Chunk.Text`) SHA-256 **바이너리** — L2 행 단위 멱등 비교에 사용 |
| `version` / `version_id` | 섹션 행·파이프라인 스냅샷 |
| `sort_order` | 동일 부모 아래 순서 |
| `l3_child_hash` | 해당 섹션 L3 통합 해시 |
| `created_at` / `modified_at` | 생성·갱신 시각 |

### 1.5 `knowledge_l3` (Content)

| 개념 | 설명 |
|------|------|
| `id` | `uuid` PK |
| `l2_id` | `uuid` NOT NULL FK → `knowledge_l2` |
| `parent_id` | 그룹/문맥 (L3 체인 등) |
| `content` | 원문(`TEXT`) |
| `content_hash` | `content`의 SHA-256 **hex** 문자열 |
| `version` / `version_id` | 행·파이프라인 스냅샷 |
| `sort_order` | 동일 L2 내 순서 |
| `qdrant_point_id` | 벡터 포인트 연동 키 |
| `created_at` / `modified_at` | 생성·갱신 시각 |

### 1.6 `sort_order` (L2 / L3 공통 권장)

- **목적**: 동일 부모 아래 **순서**를 정수로 유지.
- **방식**: **중간값 + 재정렬(Integer)** — 초기 1,000 단위, 인접 삽입 시 `round((A+B)/2)`, 필요 시 리밸런싱. (상세는 기존 설계와 동일.)

### 1.7 `pipeline_version`

| 개념 | 설명 |
|------|------|
| `id` | `bigserial` PK — 인제스트/전처리 실행 스냅샷 식별 |
| `name`, `bytea_metadata`, `preprocessing_metadata` | 파이프라인 이름·메타 JSON |

`documents.version_id` / `knowledge_l*`.version_id 등이 참조할 수 있다.

### 1.8 `keyword_so` (Tuber — 키워드 엔티티)

문서 `machine_id` 와 **다른 이름공간**이다. 키워드 텍스트 → 안정 `machine_id` 매핑.

| 컬럼 | 설명 |
|------|------|
| `machine_id` | PK — 구현상 키워드 정규화 키(예. `kw:` + 소문자)에 대한 SHA-256 **앞 8바이트**로부터 파생된 BIGINT |
| `canonical_name` | 정규화된 표기 |
| `wikidata_id`, `aliases`, `lang` | 외부 연동·별칭 |
| **`content_hash_bin`** | 위 정규화 키에 대한 SHA-256 **전체 32바이트**(`BYTEA`). PK는 이 해시의 일부만 쓰므로, **저장된 전체 다이제스트**는 (1) `machine_id` 충돌이 의심될 때 재계산 해시와 비교, (2) **파생·정규화 규칙이 바뀐 뒤** 기존 행을 재검증·감사, (3) 행 단위 무결성 확인에 사용한다. DDL에는 `COMMENT ON COLUMN` 및 인라인 주석으로 동일 목적이 명시되어 있다. |
| `created_at` / `modified_at` | 생성·갱신 시각 |

### 1.9 기타

- Phloem 환경 변수: `GOPEDIA_SOURCE_TYPE`, `GOPEDIA_PROJECT_ID` 등.

---

## 2. Qdrant (전처리 데이터·벡터·필터)

- **역할**: L1 요약 벡터·L3(또는 정책에 따른 청크) **임베딩** 및 **시맨틱 검색**.
- **Payload (Phloem DefaultSink)** — `documents.machine_id`·`doc_id`·`level`·`toc_path` 는 넣지 않는다(필터·운영·스코프는 **`l1_id`** 중심).
  - **`l1_id`**: `knowledge_l1.id` (UUID 문자열) — **주 필터/스코프 키**, RAG·TypeDB와 동일한 “문서 스냅샷” 단위.
  - **`l2_id`**, **`l3_id`**: 문자열(UUID), PG와 1:1.
  - L1 전용 포인트: `l1_id`, `version`, `source_type`, `project_id`.
  - L3 포인트: 위에 더해 `section_id`, `version_id`, `keyword_ids`(Tuber), `source_type`, `project_id`.

**RAG**: 검색 결과에서 **`l1_id`** 로 묶으면 “문서(스냅샷) 단위”로 모든 섹션·문장을 따라갈 수 있다. 앵커 문서 UUID(`documents.id`)가 필요하면 PG에서 `knowledge_l1.document_id` 로 조회한다.

---

## 3. TypeDB (관계·식별자)

- **역할**: 그래프·관계. 시맨틱 유사도는 Qdrant가 담당.
- **문서 노드**: 엔티티는 **`l1_id`(string, PG `knowledge_l1.id`)** 를 기준으로 둔다. 그러면 **적재된 모든 L1**이 그래프에서 문서 후보가 되어, RQG 시 코퍼스 전체를 대상으로 할 수 있다.
- **`documents.id`**: 필요 시 별도 속성/엣지로 연결해 “앵커 문서”와 묶는다(동일 논리 문서의 여러 L1이 있을 수 있음).
- **L2 / L3**: `l2_id`·`l3_id`(string), `section_id` 등 레거시 호환 필드는 정책에 따름.

---

## 4. Redis (Tuber — 키워드 캐시)

- **역할**: **Keyword ↔ `keyword_so.machine_id`** 캐시. `documents.machine_id` 와 **다른 엔티티**이다.
- **Key**: `kw:{keyword_text}`
- **Value**: `{machine_id}` (키워드 쪽 ID)

---

## 5. 요약 표

| 저장소 | 앵커·원본 | 벡터·필터 | 관계·탐색 |
|--------|-----------|-----------|-----------|
| PostgreSQL | `documents`, L1–L3 UUID, 해시, child hashes | — | 논리 문서·리비전·Explorer |
| Qdrant | — | L1/L3 벡터, Payload(`l1_id`,`l2_id`,`l3_id`,…) | 시맨틱 검색 |
| TypeDB | — | — | **`l1_id` 문서 노드** + L2/L3·관계 |
| Redis | — | `kw:*` → keyword `machine_id` | Tuber |

---

## 6. End-to-end 키 계약

1. **시맨틱 검색**: Qdrant → Payload로 `l1_id`·`l2_id`·`l3_id`·`source_type` 등 확보(`documents.id` 는 Qdrant에 두지 않음).
2. **구조·RQG**: **`l1_id`** 로 TypeDB에서 문서 노드·하위 섹션 확장. 동일 `document_id`의 다른 리비전은 별 `l1_id` 행으로 공존 가능.
3. **인제스트 기준선**: `documents.id` / `documents.machine_id` 가 API·외부 시스템과 맞는 **글로벌 대표** 키.
