# 01. Gopedia Architecture Design: Rev2 Overview

## Executive Summary
Gopedia is an Enterprise Knowledge Graph Platform designed to process fragmented information into a cohesive "knowledge neural network." It specializes in high-throughput **Ingestion (Phloem)** and high-fidelity **Retrieval-Augmented Generation (Xylem)**. 

The service does not merely store "chunks"; it maintains a strict structural hierarchy (Identity → Structure → Content) to enable advanced relationship reasoning (Enterprise Ontology) and contextual understanding.

## Documentation Index
This documentation suite follows a concise "Main + References" structure. 
*   **Main files** provide high-level context and architectural intent.
*   **References** (`references/`) contain detailed specifications, diagrams, and code mapping.

### Main Documents
1. [02. The Rhizome Pipeline](./02-rhizome-pipeline.md) - High-level data flows (Ingestion & RAG).
2. [03. Core Components](./03-core-components.md) - Key structural modules (Go Server, Python Worker).
3. [04. Data Hierarchy & Schema](./04-data-hierarchy-schema.md) - The L1 → L2 → L3 structure and Database schema.
4. [05. Future Roadmap](./05-future-roadmap.md) - Next steps for Expansion and Connection.
