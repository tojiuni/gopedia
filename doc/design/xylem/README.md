# Xylem (RAG retrieval) — Overview

**Xylem** is the Python retrieval path: **query embedding → Qdrant L3 search → optional cross-encoder rerank → PostgreSQL “rich context”** (neighbors, headings, metadata). It is invoked via **`flows.xylem_flow.cli`** and, in production API mode, from the **Fuego** HTTP server as a subprocess (same module).

- **Core API**: `flows/xylem_flow/retriever.py` — `retrieve_and_enrich()`, `fetch_rich_context()`, `rerank_candidates()`  
- **CLI**: `flows/xylem_flow/cli.py` — `search`, `restore`  
- **HTTP**: `GET /api/search` in `internal/api/api.go` → `python -m flows.xylem_flow.cli search ...`  
- **Deep dive**: [pipeline.md](./pipeline.md)

## Architecture (current code)

```mermaid
flowchart TB
  subgraph entry [Entry points]
    HTTP[GET /api/search<br/>internal/api/api.go]
    CLI[python -m flows.xylem_flow.cli search]
  end

  subgraph xylem [flows.xylem_flow]
    RE[retrieve_and_enrich]
    PC[project_config<br/>fetch_project_source_metadata<br/>resolve_retrieval_settings]
    EMB{Embedding backend?}
    EOpenAI[embed_query_openai]
    ELocal[embed_query_local<br/>query: prefix]
    QD[qdrant_search_l3_points<br/>candidate_limit default 30]
    RR{use_reranker?}
    RER[rerank_candidates<br/>CrossEncoder BGE default]
    FR[fetch_rich_context<br/>per hit]
  end

  subgraph stores [Stores]
    QDR[(Qdrant)]
    PG[(PostgreSQL)]
  end

  HTTP --> CLI
  CLI --> RE
  RE --> PC
  RE --> EMB
  EMB -->|GOPEDIA_EMBEDDING_BACKEND=local| ELocal
  EMB -->|else OpenAI| EOpenAI
  ELocal --> QD
  EOpenAI --> QD
  QD --> QDR
  RE --> RR
  RR -->|no| FR
  RR -->|yes| RER
  RER --> FR
  RER --> PG
  FR --> PG
```

**Rerank order (important)**: Reranking runs on **(hit, l3_id)** pairs **after** Qdrant returns up to `candidate_limit` points and **before** trimming to `final_limit`. It uses **L3 `content` from PostgreSQL** plus the **cross-encoder** (`sentence_transformers.CrossEncoder`), not the Rev4 “metadata-weight table” in code — that document is a **design target**.

**TypeDB**: Not used in `flows/xylem_flow/`; semantic RAG is Qdrant + PG.

## API search parameters (rerank)

From `internal/api/api.go` query string:

- `reranker=true` → adds `--reranker` to CLI  
- `reranker_model=...` → `--reranker-model`  
- `project_id`, `top_k` → `--project-id`, `--limit`

## Related docs

| Doc | Role |
|-----|------|
| [pipeline.md](./pipeline.md) | Parameters, `fetch_rich_context` levels, restorer |
| [../Rev3/references/xylem-flow.md](../Rev3/references/xylem-flow.md) | Older sequence diagram (API stages) |
| [../Rev4/03-atomic-l3-metadata-strategy.md](../Rev4/03-atomic-l3-metadata-strategy.md) | Metadata-aware rerank **design** (not fully implemented in `rerank_candidates`) |

## Known gaps and improvement backlog

| Area | Status / gap | Notes |
|------|----------------|--------|
| **Metadata-aware rerank** | Rev4 describes weighted rerank by `source_path`, `section`, `fact_tags` | Current `rerank_candidates()` is **query–content cross-encoder only** (no metadata scoring in Python). |
| **Subprocess per request** | API runs **new Python process** for each search | Cross-encoder load can repeat if process is cold; see `doc/design/plan/TODO.md` — warm service or long-lived worker. |
| **candidate_limit** | Default **30** in `retrieve_and_enrich` | Not exposed on HTTP API as a first-class param (CLI supports `--limit` for final; API passes `top_k` → `--limit` only for final). |
| **Hybrid retrieval** | Vector-only | TODO mentions BM25/keyword + vector; not in `retriever.py`. |
| **Xylem ↔ ingest alignment** | Project metadata drives Qdrant host/collection/embedding | `resolve_retrieval_settings` in `project_config.py` — must match Phloem ingest for collection and embedding space. |

## Planned / possible directions (not implemented)

- **Rev4-style rerank** or a dedicated scoring layer after dense retrieval.  
- **Long-lived Xylem service** (gRPC/HTTP) to avoid subprocess + model reload.  
- **Expose `candidate_limit` / `neighbor_window` / `context_level`** on `/api/search` for tuning without code change.  
- **Adaptive rerank** (e.g. Rev3 roadmap) when hit-score spread is low.
