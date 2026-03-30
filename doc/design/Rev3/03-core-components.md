# 03. Core Components (Rev3)

Rev3 keeps the same major component topology as Rev2, but the HTTP API now owns a clearer "response contract shaping" role for AI agents.

## 1. Component Overview
* **Fuego HTTP API (`internal/api`)**: Entry point for health, ingest, job status, and search. In Rev3, it validates `detail` / `fields` and shapes JSON response payloads.
* **Phloem gRPC Server (`internal/phloem`)**: High-throughput ingestion and persistence orchestration.
* **Python Xylem Retrieval (`flows/xylem_flow`)**: Query embedding, vector retrieval, rerank (optional), and context enrichment.
* **Data Stores**:
  * PostgreSQL: hierarchy and metadata
  * Qdrant: vector similarity retrieval
  * TypeDB: relationship graph

## 2. Rev3 Responsibilities Added
1. **Staged Search Contract**
   - `detail=summary|standard|full`
   - `fields=` sparse selection override
2. **Stable Agent Backward Compatibility**
   - Existing `format=json` clients still receive full objects by default.
   - `format` omitted still returns markdown.
3. **Snippet Safety Improvement**
   - Snippet generation uses sentence/word boundary preference and rune-safe truncation.

## 3. Reference Documents
👉 [Agent Interop API Contract](./references/agent-interop-api.md)  
👉 [Xylem Flow: Retrieval Pipeline (Rev3)](./references/xylem-flow.md)
