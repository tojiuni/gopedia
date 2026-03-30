# 01. Gopedia Architecture Design: Rev3 Overview

## Executive Summary
Rev3 keeps the Rev2 core architecture (Phloem ingestion + Xylem retrieval on PostgreSQL/Qdrant/TypeDB) and adds a first-class **Agent Interop contract** for machine clients.

The biggest change is that search JSON is now explicitly designed for staged agent execution:
- `detail=summary` for low-token discovery
- `detail=standard` for balanced context
- `detail=full` (or omit) for full reconstruction context
- `fields=` for sparse field selection when clients need tighter payload control

## Documentation Index
This documentation suite follows the same concise "Main + References" structure as Rev2.
*   **Main files** provide architectural direction and high-level contracts.
*   **References** (`references/`) provide concrete API and flow details.

### Main Documents
1. [02. Rhizome Pipeline in Rev3](./02-rhizome-pipeline.md) - Updated ingestion/retrieval flow with staged JSON retrieval.
2. [03. Core Components](./03-core-components.md) - Service boundaries and new API shaping responsibility.
3. [04. Data Hierarchy & Retrieval Contract](./04-data-hierarchy-schema.md) - L1/L2/L3 hierarchy and `summary/standard/full` fieldsets.
4. [05. Future Roadmap](./05-future-roadmap.md) - Next technical steps after Rev3 stabilization.

### References
* [Agent Interop API Contract](./references/agent-interop-api.md)
* [Xylem Flow: Retrieval Pipeline (Rev3)](./references/xylem-flow.md)
