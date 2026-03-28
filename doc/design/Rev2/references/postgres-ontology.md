# PostgreSQL Ontology Schema

**Canonical Source:** `core/ontology_so/postgres_ddl.sql`

## 1. Core Tables

*   **`projects`**: The top-level scope for ingestion representing a filesystem root or workspace.
    *   **Keys**: `id` (BIGSERIAL), `machine_id` (BIGINT, UNIQUE).
    *   **Fields**: `root_path` (UNIQUE), `name`, `source_metadata`.
*   **`documents`**: The logical file anchor.
    *   **Keys**: `id` (UUID), `machine_id` (BIGINT, UNIQUE), `current_l1_id`.
    *   **Relationships**: Belongs to a project via `project_id REFERENCES projects(id) ON DELETE SET NULL`.
*   **`knowledge_l1`**: Represents revisions (snapshots) of a document.
    *   **Relationships**: Belongs to both a document (`document_id`) and a project (`project_id`). Inherits the `project_id` for efficient workspace-level filtering during RAG. Contains `l2_child_hash` for idempotency, `toc`, and `summary`.
*   **`knowledge_l2`**: Child of L1. Stores `sort_order` and `source_metadata` (e.g., table columns, block types).
*   **`knowledge_l3`**: Atomic content (child of L2). Includes `qdrant_point_id` mapping for vector search and `content_hash`.
*   **`keyword_so`**: Maps Tuber keyword text to a `machine_id`.
    *   **Keys**: `machine_id` (BIGINT PRIMARY KEY).
    *   **Fields**: `canonical_name`, `aliases`, `lang`, `content_hash_bin`.
    *   **Logic**: The `machine_id` is derived from the first 8 bytes of the full SHA-256 digest of the normalized key (e.g., `kw:<lowercase keyword>`). The `content_hash_bin` stores the full 32-byte hash to detect suspected `machine_id` collisions and audit rows.

## 2. Idempotency Tracking
Hashing (`l2_child_hash`, `l3_child_hash`) is used to deduplicate chunks during ingestion. Unchanged content between L1 snapshots reuses the same underlying structure to save compute.
