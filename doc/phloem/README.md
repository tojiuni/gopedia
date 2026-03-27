# Phloem Flow 아키텍처

Stem 파이프라인: Root → Phloem (gRPC) → Rhizome (PostgreSQL, TypeDB, Qdrant).

**실행·Docker 인제스트 요약**: [USAGE.md](./USAGE.md)

파이프라인은 **도메인별로 운영**하고, chunking / sink / toc / embedder 등은 **boilerplate 컴포넌트**로 두어 다른 도메인에서 재사용할 수 있도록 구성한다.

---

## 1. 설계 원칙

- **공통 타입**: `Chunk`, `TOCNode`, `FlatTOCItem` 등은 한 패키지(`types`)에만 두고, 모든 컴포넌트가 동일 타입을 사용한다.
- **컴포넌트**: 인터페이스 + 여러 구현(파일별). 도메인에 무관하게 재사용 가능하다.
- **도메인 파이프라인**: 위 컴포넌트를 조합하는 얇은 레이어만 둔다.
- **순환 의존 방지**: 공통 타입을 별도 패키지로 분리하고, toc / chunker / sink 는 그 패키지만 의존한다.

---

## 2. 폴더/파일 구조

```
internal/phloem/
├── types/
│   └── types.go              # Chunk, TOCNode, FlatTOCItem 등 공통 타입
│
├── toc/                      # TOC/구조 추출 (boilerplate)
│   ├── toc.go                # TOCParser 인터페이스, FlattenTOC, TOCToJSON
│   ├── markdown.go           # MarkdownTOCParser (# 헤딩)
│   ├── code.go               # CodeTOCParser (함수/클래스 등, 선택)
│   └── pdf.go                # PDFTOCParser (페이지/블록, 선택)
│
├── chunker/                  # 청킹 전략 (boilerplate)
│   ├── chunker.go            # Chunker 인터페이스
│   ├── heading.go            # ByHeadingChunker (TOC 섹션 단위)
│   ├── fixed.go              # ByFixedSizeChunker (고정 길이/토큰)
│   └── symbol.go             # BySymbolChunker (코드 심볼 단위, 선택)
│
├── sink/                     # 저장 (boilerplate)
│   ├── sink.go               # SinkWriter 인터페이스, SinkConfig
│   └── writer.go             # DefaultSink (PG + Qdrant L1/L2)
│
├── embedder/                 # 임베딩 (boilerplate)
│   └── embedder.go           # Embedder 인터페이스 + OpenAI 구현
│
├── domain/                   # 도메인별 파이프라인 (컴포넌트 조합)
│   ├── domain.go             # 도메인 이름 상수, 공통 헬퍼
│   ├── wiki.go               # NewWikiPipeline() → Pipeline
│   ├── code.go               # NewCodePipeline()
│   └── pdf.go                # NewPDFPipeline()
│
├── pipeline.go               # Pipeline 인터페이스 (Process(ctx, req) → resp)
├── registry.go               # domain → Pipeline 등록/조회
├── server.go                 # gRPC Server (registry로 도메인별 Pipeline 호출)
└── ...
```

---

## 3. 파일별 역할

| 경로 | 역할 |
|------|------|
| **types/types.go** | `Chunk`, `TOCNode`, `FlatTOCItem` 등 파이프라인 전역에서 쓰는 공통 타입. toc / chunker / sink 가 모두 여기만 참조해 순환 의존을 막는다. |
| **toc/toc.go** | `TOCParser` 인터페이스(예: `Parse(content string) ([]types.TOCNode, error)`), `FlattenTOC`, `TOCToJSON` 등 공통 함수. |
| **toc/markdown.go** | `MarkdownTOCParser` 구조체 + `Parse()` — 마크다운 `#` 헤딩 기반. |
| **toc/code.go** | `CodeTOCParser` — AST/심볼 기반 구조 추출 (선택). |
| **toc/pdf.go** | `PDFTOCParser` — 페이지/블록 단위 구조 (선택). |
| **chunker/chunker.go** | `Chunker` 인터페이스(예: `Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error)`). |
| **chunker/heading.go** | `ByHeadingChunker` — FlattenTOC 결과를 그대로 청크로. |
| **chunker/fixed.go** | `ByFixedSizeChunker` — 고정 문자/토큰 단위 (PDF 등). |
| **chunker/symbol.go** | `BySymbolChunker` — 함수/클래스 단위 (선택). |
| **sink/sink.go** | `SinkWriter` 인터페이스(`Write(ctx, msg, chunks) (docID string, err error)`), `SinkConfig`. |
| **sink/writer.go** | `DefaultSink` — `documents` 앵커 + `knowledge_l1`·L2/L3, `current_l1_id` 갱신, Qdrant payload는 `l1_id` 중심(`doc_id`/`machine_id` 미포함). |
| **embedder/embedder.go** | `Embedder` 인터페이스 + OpenAI 구현. |
| **domain/domain.go** | 도메인 이름 상수(`Wiki`, `Code`, `PDF`), 필요 시 공통 헬퍼. |
| **domain/wiki.go** | `NewWikiPipeline(toc, chunker, sink)` 등으로 조합해 `Pipeline` 반환. |
| **domain/code.go** | `NewCodePipeline(toc.CodeTOCParser, chunker.BySymbolChunker, ...)`. |
| **domain/pdf.go** | `NewPDFPipeline(toc.PDFTOCParser, chunker.ByFixedSizeChunker, ...)`. |
| **pipeline.go** | `Pipeline` 인터페이스(예: `Process(ctx, req) (*IngestResponse, error)`). 도메인 구현체는 내부에서 toc.Parse → chunker.Chunks → sink.Write 호출. |
| **registry.go** | `Register(domain string, p Pipeline)`, `Get(domain string) (Pipeline, bool)`. `cmd/phloem/main.go`에서 `domain.RegisterWiki(...)` 등으로 등록. |
| **server.go** | `IngestMarkdown`에서 `req.Domain`(또는 메타데이터)으로 registry에서 Pipeline 조회 후 `Process(ctx, req)` 호출. |

---

## 4. 의존 관계

```
types ← toc, chunker, sink, embedder
  ↑
  ├── domain (toc + chunker + sink + embedder 조합)
  └── pipeline, registry, server
```

- **toc / chunker / sink / embedder**: 서로 참조하지 않고, 모두 `types`만 사용.
- **domain**: toc, chunker, sink, embedder를 조합해 `Pipeline` 구현만 담당.
- **server**: registry와 pipeline 인터페이스만 사용.

---

## 5. 도메인 식별 (API)

- **권장**: `IngestRequest`에 `domain` 필드 추가 (예: `string domain = 4;`, 기본값 `"wiki"`). Root에서 `domain = "wiki" | "code" | "pdf"` 등으로 설정해 보내면 Phloem이 이 값으로 파이프라인을 선택.
- **대안**: Proto는 그대로 두고 `source_metadata["domain"]`으로만 구분. 구현은 빠르지만 API 계약이 명시적이지 않음.

---

## 6. 새 도메인 추가 시 보일러플레이트

1. **types**: 보통 수정 없음.
2. **toc**: 새 형식이면 `toc/xxx.go` 추가 후 `TOCParser` 구현.
3. **chunker**: 새 전략이면 `chunker/xxx.go` 추가 후 `Chunker` 구현.
4. **domain**: `domain/xxx.go`에서 기존 toc/chunker/sink 구현체를 조합해 `NewXxxPipeline()`만 추가.
5. **등록**: `registry.Register(domain.Xxx, NewXxxPipeline(...))` 한 줄 추가.

---

## 7. 실행

- **요약 가이드**: [USAGE.md](./USAGE.md)
- **서버**: `go run ./cmd/phloem` (또는 `docker compose up phloem-flow`).
- **마크다운 전송**: `python -m property.root_props.run /path/to/doc.md` (see `property/root_props/`).
- **Docker로 프로젝트 디렉터리만 인제스트** (DB 리셋·init 없음): `scripts/run_project_ingestion_docker.sh` — 세부는 [docker-ingestion.md](./docker-ingestion.md).

구현 위치: `cmd/phloem` (Go gRPC 서버), `internal/phloem` (TOC, sink, embedding 및 도메인 파이프라인).

---

## 8. 마이그레이션 참고

현재 `server.go`는 기존 단일 파이프라인(마크다운 전용)을 그대로 사용한다. 도메인별 파이프라인으로 전환하려면:

1. `cmd/phloem/main.go`에서 각 도메인 파이프라인을 등록한다:
   - `phloem.Register(domain.Wiki, domain.NewWikiPipeline(toc.MarkdownTOCParser{}, chunker.ByHeadingChunker{}, defaultSink, idGen))`
   - 필요 시 `domain.Code`, `domain.PDF` 등도 등록.
2. `IngestRequest`에 `domain` 필드가 있다면(또는 `source_metadata["domain"]`) `server.IngestMarkdown`에서 `phloem.GetOrDefault(domain).Process(ctx, req)`로 위임하도록 수정한다.
