# Phloem flow (Ingestion)

Stem pipeline: Root → Phloem (gRPC) → Rhizome (PostgreSQL, TypeDB, Qdrant).

**Implementation**: `cmd/phloem` (Go gRPC server) and `internal/phloem` (TOC, sink, embedding).

- **Run server**: `go run ./cmd/phloem` (or `docker compose up phloem-flow`).
- **Send markdown**: `python -m property.root_props.run /path/to/doc.md` (see `property/root_props/`).
