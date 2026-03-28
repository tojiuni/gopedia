# 🌿 Gopedia: Enterprise Knowledge Rhizome

> **"Deepening the roots of knowledge in the soil of data to bear the fruits of organically connected wisdom."**

Gopedia is a high-efficiency Enterprise Knowledge Graph Platform specializing in **Ingestion** and **RAG (Retrieval-Augmented Generation)**. It integrates fragmented information into a cohesive "knowledge neural network," providing a foundation for **Enterprise Ontology** where relationship reasoning and contextual understanding are at the core.

---

## ✨ Key Values

* 🔌 **Pluggable (Root)**: Seamlessly connect any external data source at a workspace/project scale.
* 📈 **Scalable (Stem)**: High-throughput pipelines built on gRPC/Protobuf.
* 🔗 **Relational (Rhizome)**: Beyond simple storage—building an organic network of knowledge using vector and graph databases.
* 🍎 **Actionable (Fruit)**: Transforming retrieved data into decision-ready reports and insights.

---

## 🏗️ Architecture: The Rhizome Metaphor

Inspired by the **Rhizome**—a horizontal, non-hierarchical, and infinitely expandable root system—Gopedia’s architecture ensures that every component is modular yet organically linked.

### 1. Root — *Pluggable Sources & Workspaces*
The "entry point" where nutrients (data) are absorbed. It registers entire Project Workspaces (directories or repos) and defines connection standards for external sources like Databases, APIs, Streams, and File Systems.

### 2. Stem — *Scalable Pipelines*
The transport system for data, divided into two vital flows:
* **Phloem (Ingestion)**: **Root → Stem → Rhizome**. Encapsulates raw data, structures it hierarchically (Project → Document → L1/L2/L3), handles NLP tasks (Sentence Splitting, Entity masking), and records them into the Rhizome via **Smart Sinks**.
* **Xylem (RAG)**: **Rhizome → Leaf/Fruit**. Retrieves L3 chunks via vector search and intelligently reconstructs their parent structural context (L2 sections, tables, code blocks) for rich prompt injection.

### 3. Rhizome — *Relational & Polyglot Storage*
The "Knowledge Soil." This layer handles identity and relationship reasoning using **Polyglot Persistence**:
* **PostgreSQL**: For canonical storage, strict structural hierarchy, idempotency hashing, and Tuber entities (`keyword_so`).
* **Qdrant**: For semantic vector search.
* **TypeDB**: For relationship reasoning and deep graph traversal.

### 4. Leaf & Fruit — *Views & Actionable Outputs*
* **Leaf (Indexing View)**: Domain-specific views such as Markdown, Code, or Ticket indexes.
* **Fruit (Reports)**: Final templates or generated answers that combine data from multiple Roots and Leaves into a human-readable format.

---

## 📊 Data Hierarchy

Gopedia does not just "chunk" data; it categorizes it into a meaningful hierarchy to ensure high-fidelity retrieval and idempotency.

| Level | Entity | Description |
| --- | --- | --- |
| **Project** | `projects` | The root workspace container. Has a globally stable `machine_id`. |
| **Doc** | `documents` | The logical file anchor within a Project. |
| **L1** | `knowledge_l1` | Document snapshot/revision. Holds the Table of Contents and summary. |
| **L2** | `knowledge_l2` | The "Skeleton" of data (Sections, Tables, AST structures, Logic flows). |
| **L3** | `knowledge_l3` | Atomic content (e.g., sentences) that are vectorized for search. |
| **Keyword** | `keyword_so` | Tuber entity (Tags/Keywords) mapped to a stable `machine_id`. |

---

## 🚀 Roadmap: Design Phases

We are currently transitioning into the **Rev2 (Growth & Fruition)** phase.

1. **Verify (Germination) `COMPLETED`**: Validating the flow from Markdown and Code sources into the Rhizome.
2. **Expand (Growth) `IN PROGRESS`**: Activating distributed processing via Project-level Ingestion, Tuber `machine_id` mappings, AST parsing, and NLP entity extraction (NER).
3. **Connect (Fruition)**: Full integration with the GeneSo ecosystem, featuring complex RAG Fruit (Skill Engine) and ReBAC (Relationship-Based Access Control via SpiceDB).

---

## 📚 Documentation

For detailed architecture diagrams, pipeline specifications, and database schemas, please refer to the **[Rev2 Design Documentation](./doc/design/Rev2/01-overview.md)**.

---

## 📝 License

This project is licensed under the **MIT License**.
