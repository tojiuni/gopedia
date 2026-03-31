# Changelog

All notable changes to Gopedia are documented here.  
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).  
Versioning follows `v<major>.<minor>.<patch>`:

| Bump | When |
|------|------|
| `major` | Breaking schema changes, pipeline architecture overhaul |
| `minor` | New feature (domain, entrypoint, restore API, etc.) |
| `patch` | Bug fix, doc, test |

---

## [Unreleased] — v0.2.0

### Added
- **IMP-04** `property.root_props.run` auto-routes code files (`.py` `.go` `.ts` …) to
  `CodePipeline`; single entrypoint for mixed markdown + code directories.
- **IMP-07** `documents.ingest_version` column — records gopedia semver at ingest time
  (`GOPEDIA_VERSION` env var, default `dev`).
- **IMP-05** Gardener code domain smoke dataset (`dataset/code_domain_smoke.json`) —
  6 queries against `verify_xylem_flow.py` and `chunker/code.go`.
- `CHANGELOG.md` and `scripts/tag-release.sh` for version management.

### Fixed
- **IMP-01** Title-based duplicate ingest prevention in `Sink.Write()` — same file
  ingested from a different project no longer creates a second `knowledge_l1` row when
  content hash matches.
- **IMP-02** `source_path` populated in Xylem search results — code hits now return the
  absolute file path; markdown hits return the document name from `source_metadata`.

### Improved
- **IMP-03** Bilingual embedding: `l2_source_metadata` Korean technical terms annotated
  with English equivalents before embedding to improve cross-language recall.

---

## [v0.1.0] — 2026-04-01

### Added
- **Code domain pipeline** (Phase 1): tree-sitter based ingest for Python / Go source
  files. Each source line becomes one `knowledge_l3` row (`source_metadata` JSONB with
  `line_num`, `node_type`, `is_anchor`, `is_block_start`).
- `CodeTOCParser` — calls `python3 -m flows.code_parser.cli` via subprocess.
- `CodeChunker` — groups lines into L2 chunks by top-level anchor boundary; re-indexes
  `parent_idx` per chunk.
- `insertCodeL3Lines` — 1 line = 1 L3 with `parent_id` → anchor UUID hierarchy.
- Selective Qdrant embedding — only `is_anchor=true` lines get vectors.
- `property.root_props.run_code` — ingest entrypoint; sets `domain=code`,
  `source_type=code`, `language=<lang>`.
- `restore_code_for_l2` / `fetch_code_snippet` helpers in `flows/xylem_flow/restorer.py`.
- `doc/guide/code-domain.md` — full code domain guide including Gardener quality test
  workflow.
- `doc/rag-test-reports/` — version-tagged RAG quality test reports.

### Fixed
- `source_type` in `Sink.Write()` now reads `SourceMetadata["source_type"]` first
  (was always writing `"md"`).
- `restore_content_for_l1` for `source_type=code` — joins L3 lines by `\n` without
  markdown fences (was wrapping each L2 chunk in triple-backticks).
- `CodePipeline` registration in the embedded Phloem gRPC server (`internal/api/api.go`
  was missing it).

### RAG test results (v0.1.0)
| Target | Queries | Avg score | Result |
|--------|---------|-----------|--------|
| neunexus | 6 | 0.675 | ✅ PASS |
| gopedia universitas | 8 | 0.594 | ⚠️ PARTIAL |

Full report: [`doc/rag-test-reports/v0.1.0_2026-04-01_neunexus-gopedia.md`](doc/rag-test-reports/v0.1.0_2026-04-01_neunexus-gopedia.md)
