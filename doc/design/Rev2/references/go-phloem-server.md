# Go Phloem Server Details

**Implementation Path:** `cmd/phloem/`, `internal/phloem/`

## 1. Role
The Go Phloem server is the high-throughput ingestion engine. It receives raw data, parses its structure, and saves it into the storage layer.

## 2. Design Principles
*   **Pluggable Domains**: Features specific pipelines for Markdown (`wiki`), Code (`code`), and PDF (`pdf`).
*   **Boilerplate Reusability**: Components like `toc`, `chunker`, `embedder`, and `sink` are decoupled interfaces.

## 3. The `DefaultSink` (`internal/phloem/sink/writer.go`)
Manages the distribution of data:
- Inserts logical anchors into `documents`.
- Manages `knowledge_l1`, `l2`, and `l3` insertions into PostgreSQL.
- Inserts payload data into Qdrant indexed by `l1_id`.
