# 01. Chunking Architecture — Rev4

## 개요

Phloem 인제스트 파이프라인의 핵심 단계인 **청킹(Chunking)**은 원본 문서를
임베딩 및 저장에 적합한 단위(L2 Chunk)로 분해하는 과정이다.

도메인(Markdown / Code / 기타)에 따라 서로 다른 전략을 적용하며,
공통 인터페이스(`Chunker`)를 통해 파이프라인과 결합된다.

> Rev4 확장 전략(원자적 L3 유지 + metadata-aware retrieval/restore)은
> [`03-atomic-l3-metadata-strategy.md`](./03-atomic-l3-metadata-strategy.md)를 참고한다.

```go
// internal/phloem/chunker/chunker.go
type Chunker interface {
    Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error)
}
```

---

## 전체 흐름

```
문서 입력 (Markdown / Code)
        │
        ▼
  TOC 파서
  ┌─────────────────────────────────┐
  │ Markdown  →  MarkdownTOCParser  │
  │ Code      →  CodeTOCParser      │
  └─────────────────────────────────┘
        │  []TOCNode
        ▼
  Chunker.Chunks()
  ┌─────────────────────────────────────────────┐
  │ Markdown  →  ByHeadingChunker               │
  │              + ExpandStructuredChunks()      │
  │ Code      →  CodeChunker                    │
  │ 폴백      →  ByFixedSizeChunker             │
  └─────────────────────────────────────────────┘
        │  []types.Chunk  (L2)
        ▼
  Sink (PostgreSQL knowledge_l2 + knowledge_l3)
  ┌─────────────────────────────────────────────┐
  │ Markdown  →  NLP Worker → 문장 분리 → L3    │
  │ Code      →  L3Lines (1줄 = 1 L3 row)       │
  └─────────────────────────────────────────────┘
```

---

## 전략 1 — `ByHeadingChunker` (Markdown 도메인)

**파일**: `internal/phloem/chunker/heading.go`

ATX 헤딩(`# ~ ######`)을 섹션 경계로 사용하여 **1섹션 = 1 L2 Chunk**를 생성한다.

### 동작 순서

1. `MarkdownTOCParser`가 헤딩 트리를 추출하고 `FlattenTOC`으로 평탄화
2. 첫 번째 헤딩 이전 텍스트(frontmatter, preamble)는 `sectionID: "root"` 청크로 보존
3. 각 섹션은 "다음 헤딩 직전까지"를 본문으로 취함 (overlap 없음)
4. 결과 청크를 `ExpandStructuredChunks()`에 전달하여 내부 구조화 블록 추출

### 섹션 경계 결정

```
# A         ← 섹션 A 시작
  본문 ...
## B         ← 섹션 B 시작 (동시에 A의 끝)
  본문 ...
# C         ← 섹션 C 시작 (동시에 B의 끝)
```

### ExpandStructuredChunks — 구조화 블록 파생 (`structured.go`)

헤딩 청크 내부에서 4가지 블록을 **별도 L2 Chunk로 파생**한다.
파생 청크는 `ParentSectionID`로 부모 청크와 연결되며, 부모 청크에는 나머지 산문만 남는다.

| 블록 타입 | SectionID 패턴 | 감지 기준 |
|-----------|:--------------:|-----------|
| 코드 펜스 | `c1`, `c2`... | ` ```lang ` ~ ` ``` ` |
| GFM 테이블 | `t1`, `t2`... | `\|` 헤더 행 + `---` separator 행 |
| 순서 목록 | `o1`, `o2`... | `1. item` 패턴 |
| 이미지 | `i1`, `i2`... | `![alt](url)` 단독 행 |

```
# 설치 방법
  텍스트 산문...          ← 부모 청크에 잔류 (block_type: heading)
  ```bash                  ┐
  pip install gopedia      ├ → c1 청크 (block_type: code, parent: 부모 sectionID)
  ```                      ┘
  | 이름 | 버전 |          ┐
  |------|------|          ├ → t1 청크 (block_type: table, parent: 부모 sectionID)
  | gopedia | 0.4 |       ┘
```

---

## 전략 2 — `CodeChunker` (코드 도메인)

**파일**: `internal/phloem/chunker/code.go`

tree-sitter 파서 기반 AST 청킹. **top-level 선언 단위(함수/클래스) = 1 L2 Chunk**.

### 동작 순서

1. `CodeTOCParser.ParseWithLines()`로 각 줄에 메타 부여:
   - `IsAnchor`: 함수/클래스 최상위 선언 여부
   - `ParentIdx`: 부모 라인 인덱스 (`-1` = 최상위)
   - `LineNum`: 원본 파일 라인 번호
2. `parent_idx == -1 && is_anchor == true` 라인이 L2 청크 경계
3. 첫 번째 top-level anchor 이전 줄(import, 상수 등)은 `preamble` 청크로 보존
4. 각 L2 Chunk는 `L3Lines` 필드에 줄 단위 메타를 포함 → Sink에서 **1줄 = 1 L3 row** 저장

### 구조 예시

```python
import os            ← preamble 청크 (sectionID: "pre")

def foo():           ← fn1 청크 (sectionID: "fn1", path: "foo")
    ...

class Bar:           ← fn2 청크 (sectionID: "fn2", path: "Bar")
    def method():    │
        ...          │  (L3 라인으로 저장, parent_idx로 계층 추적)
```

---

## 전략 3 — `ByFixedSizeChunker` (폴백)

**파일**: `internal/phloem/chunker/fixed.go`

TOC가 없거나 적용 불가한 경우 **문자 수 기반(기본 500자)** 분할.

- semantic break 우선순위: `\n` > `.!?` (문장 끝) > `,|;` > `)}` > 공백
- break 없으면 hard cut
- `SemanticL3Split: true` 플래그로 Sink의 추가 L3 분리 지시

---

## 전략 4 — `BySymbolChunker` (미완성 플레이스홀더)

**파일**: `internal/phloem/chunker/symbol.go`

TOC 루트 노드를 그대로 청크로 변환하는 stub.
실제 구현 없음 — 추후 AST symbol range 기반으로 교체 예정.

---

## 도메인별 청커 선택 매핑

| 도메인 | 청커 | L3 분리 방식 |
|--------|------|-------------|
| `wiki` (Markdown) | `ByHeadingChunker` + `ExpandStructuredChunks` | NLP Worker 문장 분리 |
| `code` (Python/Go) | `CodeChunker` | L3Lines (1줄 = 1 L3) |
| 기타 / 폴백 | `ByFixedSizeChunker` | `SemanticL3Split` |

---

## L2 / L3 데이터 계층

```
knowledge_l2 (섹션 단위)
  ├── section_id       청커가 부여한 고유 ID
  ├── summary          섹션 요약 (NLP 또는 헤딩 텍스트)
  ├── source_metadata  {"block_type": "heading|code|table|..."}
  └── title_id         섹션 헤딩 L3 행 ID (검색 breadcrumb용)

knowledge_l3 (원자 단위 — 임베딩 대상)
  ├── content          실제 텍스트 조각 (1문장 or 1줄)
  ├── sort_order       섹션 내 순서 (neighbor window 검색용)
  ├── parent_id        코드 도메인: 부모 L3 라인 ID
  └── source_metadata  {"is_anchor": true/false} (코드 도메인)
```

### Rev4 확장 메타데이터 (권장)

atomic L3 전략에서는 저장 시 병합을 최소화하고, 아래 메타로 검색/복원을 보강한다.

- 구조: `block_type`, `block_group_id`, `list_level`, `list_item_no`
- 위치: `chunk_index`, `char_start`, `char_end`
- 연결: `prev_l3_id`, `next_l3_id`
- 힌트: `section_heading`, `breadcrumb`, `fact_tags`, `domain_tags`

상세 설계와 운영 가드레일은 `03-atomic-l3-metadata-strategy.md`에 정리되어 있다.
