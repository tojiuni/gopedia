# 02. Rhizome Pipeline in Rev3: Ingestion & Agent Retrieval

Rev3 preserves the biological flow model from Rev2 and adds explicit agent-oriented response shaping at the API edge.

## High-Level Data Flow

```mermaid
graph TD
    subgraph Roots [Data Sources]
        MD[Markdown]
        Code[Code]
        Docs[Design Docs]
    end

    subgraph Phloem [Phloem Flow: Ingestion]
        IngestAPI[POST /api/ingest or jobs]
        GoPhloem[Go gRPC Phloem]
        NLPWorker[Python NLP Worker]
        IngestAPI --> GoPhloem
        GoPhloem --> NLPWorker
    end

    subgraph Rhizome [Polyglot Storage]
        PG[(PostgreSQL)]
        Qdrant[(Qdrant)]
        TypeDB[(TypeDB)]
    end

    subgraph Xylem [Xylem Flow: Retrieval]
        SearchPy[flows.xylem_flow.cli]
        Restorer[Retriever + Context restore]
        SearchPy --> Restorer
    end

    subgraph AgentEdge [Agent Interop API]
        SearchAPI[GET /api/search]
        Summary[detail=summary]
        Standard[detail=standard]
        Full[detail=full or omitted]
        Sparse[fields=a,b,c]
    end

    Roots --> Phloem
    Phloem --> Rhizome
    Rhizome --> Xylem
    Xylem --> SearchAPI
    SearchAPI --> Summary
    SearchAPI --> Standard
    SearchAPI --> Full
    SearchAPI --> Sparse
```

## 1. Ingestion Path (Unchanged from Rev2)
- Source documents are parsed and normalized into hierarchical records (L1/L2/L3).
- Vectors are stored in Qdrant, structure in PostgreSQL, relations in TypeDB.
- Async ingest jobs remain the operationally recommended path for agents.

## 2. Retrieval Path (Extended in Rev3)
- Retrieval still starts from semantic vector search over L3.
- Context enrichment reconstructs hierarchy and metadata.
- Final API serialization now supports staged payload sizes for agent workflows.

👉 See detailed retrieval behavior in [`references/xylem-flow.md`](./references/xylem-flow.md).  
👉 See API parameter contract in [`references/agent-interop-api.md`](./references/agent-interop-api.md).
