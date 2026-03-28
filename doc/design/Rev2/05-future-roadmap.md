# 05. Future Roadmap

As Gopedia transitions fully into the **Rev2 (Growth & Fruition)** phase, the architecture will be enhanced in the following areas.

## 1. Expanding Roots
*   **AST Code Parsing**: Full rollout of the Code TOC parser to represent functions and classes as L2 structures.
*   **PDF Parsing**: Implementing fixed-size layout block ingestion.

## 2. Advanced NLP & Graph
*   **NER Integration**: Upgrading the Python NLP worker to extract Entities (People, Organizations, Tools).
*   **TypeDB Utilization**: Mapping NER results into TypeDB via `machine_id` for deep graph reasoning (Enterprise Ontology).

## 3. Security & Access
*   **ReBAC Integration**: Implementing Relationship-Based Access Control via SpiceDB. This will ensure that chunk retrieval at the L1/L2 level is strictly filtered by user permissions before it hits the LLM.
