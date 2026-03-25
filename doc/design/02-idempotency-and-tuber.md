# Idempotency & The Tuber (역할 분리)

## 0. 앵커·`machine_id` (용어)

- **인제스트 앵커**는 **`documents`** 한 행(논리 문서). `documents.machine_id` 는 전역 대표·외부 연동용 **BIGINT**다.
- **`machine_id`** 라는 이름은 온톨로지에서 **`documents`**, **`keyword_so`**, 향후 **`user`** 등 여러 엔티티에 **같은 패턴의 안정 ID**로 쓸 수 있다. 값이 겹치지 않게 할당 규칙만 정하면 되며, **문서 ID와 키워드 ID는 서로 다른 엔티티**이다.
- 멱등·해시 판단은 주로 **`documents` + `knowledge_l1` 리비전** 단위에서 이루어진다. 스킵 여부는 **`documents.current_l1_id`**(없으면 최신 L1) 아래에 L2/L3가 있는지로 본다. 리비전 규칙은 [05-storage-and-payloads.md](05-storage-and-payloads.md) § 1.3 참고.

## 1. Idempotency — 계층형 해시 (L1 / L2 / L3)

단순 문서 단위가 아니라 **L2(섹션) 및 L3(문장/원자) 단위**로 변경을 감지하여 처리 효율을 극대화한다. 해시와 원문(Protobuf)은 PostgreSQL **`BYTEA`** 로 저장한다.

### 1.1 통합 해시 — `l2_child_hash` / `l3_child_hash` (권장: 선별 인제스트의 기준)

에이전트는 **전체를 다시 읽지 않고**, 상·하위 통합 해시만 비교해 **갱신된 부분만** 인제스트할 수 있다.

| 필드 | 위치 | 의미 | 갱신 시 의미 |
|------|------|------|----------------|
| **`l2_child_hash`** | `knowledge_l1` | 하위 **L2** 목록의 구조·식별·제목 등에 기반한 **통합 해시** | 목차(헤더 트리)가 바뀜 → L1 관점에서 L2 스켈레톤 변경 |
| **`l3_child_hash`** | `knowledge_l2` | 해당 섹션 안 **L3** 문장들의 **내용·순서**에 기반한 **통합 해시** | 섹션 내 문장이 하나라도 바뀌거나 순서가 바뀜 |

- **계산 예시(개념)**: 자식 id·정렬 키(`sort_order`)·콘텐츠 해시 등을 정해진 순서로 나열한 뒤 SHA256 → `BYTEA`.
- **플로우**: 인제스트 전 `l2_child_hash` / `l3_child_hash` 를 읽어 이전 값과 비교 → **동일하면** 해당 범위의 요약·L3 파이프라인·벡터 동기화를 **스킵**할 수 있다.

### 1.2 L2 단위 세부 해시 (Section Level)

- **목적**: L2 블록 단위로 더 촘촘한 스킵(예: 요약 재생성 여부).
- **방식**:
  - 각 L2에 대해 **content hash**(SHA256)를 계산하거나, 정책에 따라 `l3_child_hash` 만으로 대체할 수 있다.
  - 기존 hash와 비교 → **동일하면** 해당 L2의 요약 및 하위 L3 처리 일부를 스킵.
  - **변경되면** L2 요약을 재처리하고, 하위 L3 비교 단계로 진입한다.

### 1.3 L3 단위 해시 (Sentence Level)

- **목적**: 섹션이 바뀌었더라도 **실제로 바뀐 문장만** 선별 처리(Embedding, NER, Qdrant/TypeDB).
- **방식**:
  - 각 L3에 대해 **content hash**를 계산하고 `knowledge_l3`의 기존 값과 비교한다.
  - **동일하면** 해당 L3의 임베딩·NER·저장소 동기화를 스킵한다.
  - **변경되면** 해당 L3만 갱신하고, 상위 **`l3_child_hash`** 를 재계산한다.

### 1.4 `l2_child_hash` / `l3_child_hash` 와 기존 필드와의 관계

- **`content_hash`** (L2/L3 행 단위): 여전히 **행 단위 멱등성**에 유용하다.
- **통합 해시**(`l2_child_hash`, `l3_child_hash`): **트리/섹션 단위로 빠르게 “바뀌었는지”** 만 판별할 때 유리하다.
- 실제 구현에서는 **둘 다** 쓰거나, 운영 단순화를 위해 통합 해시만으로 시작한 뒤 행 단위 해시를 추가할 수 있다.

---

## 2. The Tuber — 엔티티 정규화 및 키워드 캐시

- **역할**: **키워드 ↔ `keyword_so.machine_id`** 매핑의 조회·생성·캐싱. **`documents.machine_id` 와는 다른 엔티티**이다.
- **정규화 및 중복 방지 (Entity Linking)**:
  - **표준어(Canonical Form) 매핑**: "사과", "Apple", "pomme" 등 서로 다른 언어의 텍스트를 내부 사전 또는 NLP 모델을 통해 하나의 대표어(주로 영문)로 정규화한다.
  - **외부 지식 베이스 연동**: Wikidata ID(예: `Q89`)와 같은 글로벌 식별자를 브릿지로 사용하여, 언어에 상관없이 동일한 개념은 동일한 `machine_id`를 가지도록 보장한다.
  - **Alias 관리**: 하나의 `machine_id`에 여러 텍스트(Aliases)를 연결하여 저장한다.
- **저장 및 조회**:
  - **PostgreSQL** (`keyword_so`): `machine_id`, `canonical_name`, `wikidata_id`, `aliases[]`, `lang`, `content_hash_bin`, `created_at`, `modified_at` 등을 저장.
  - **`content_hash_bin`**: 정규화 키(예. `kw:` + 소문자)에 대한 SHA-256 **전체** 바이너리. PK `machine_id`는 이 해시의 앞 8바이트만 사용하므로, **충돌 의심·파생 규칙 변경 후 재검증·감사** 시 저장값과 재계산 해시를 비교하는 용도다. 상세는 [05-storage-and-payloads.md](05-storage-and-payloads.md) § 1.8 및 `postgres_ddl.sql` 의 `COMMENT ON COLUMN`.
  - **Redis**: `kw:{text}` → `{machine_id}` 캐시. 정규화된 대표어뿐만 아니라 주요 Alias들도 캐싱하여 빠른 조회를 지원한다.
- **플로우**: Extraction → (NLP/Wikidata 정규화) → 대표어/ID로 Redis 조회 → 존재 시 기존 ID 반환 → 미존재 시 신규 생성 및 DB/Redis 동시 기록.

---

## 3. 버전 관리 레이어 (Tuber와 분리)

- **역할**: 문서/L2/L3의 **content hash**, **child hash**, **version_id**, **변경 여부** 판단 및 갱신. 키워드·Machine ID 캐시는 Tuber가 담당한다.
- **저장 형식**:
  - **Content**: Protobuf 직렬화 데이터 (`BYTEA`)
  - **Hash**: SHA256 Binary (`BYTEA`, 32bytes)
- **제공 API 예시(개념)**:
  - `LookupL2ChildHash(l1ID) ([]byte)`
  - `LookupL3ChildHash(l2ID) ([]byte)`
  - `LookupL3ContentHash(l3ID) ([]byte)`
  - `UpsertWithHashes(...)` 

---

## 4. 정리

| 관심사 | 담당 | 저장소/캐시 |
|--------|------|-------------|
| L1 `l2_child_hash` / L2 `l3_child_hash` / 행 단위 해시·멱등성 | Diff/Version 레이어 | PostgreSQL (`knowledge_l1`, `knowledge_l2`, `knowledge_l3`, `pipeline_version` 등) |
| 키워드 ↔ Machine ID | The Tuber | PostgreSQL (`keyword_so`) + Redis (`kw:*`) |

Tuber는 **키워드 캐시와 Machine ID 할당**만, 버전/해시 레이어는 **통합 해시·행 해시·버전(pipeline_version)** 으로 멱등성과 선별 인제스트를 담당하도록 경계를 명확히 한다.
