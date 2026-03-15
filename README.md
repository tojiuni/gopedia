# 🌿 Gopedia: Enterprise Knowledge Rhizome

> **"Deepening the roots of knowledge in the soil of data to bear the fruits of organically connected wisdom."**

Gopedia is a high-efficiency Enterprise Knowledge Graph Platform specializing in **Ingestion** and **RAG (Retrieval-Augmented Generation)**. It integrates fragmented information into a cohesive "knowledge neural network," providing a foundation for **Enterprise Ontology** where relationship reasoning and contextual understanding are at the core.

---

## ✨ Key Values

* 🔌 **Pluggable (Root)**: Seamlessly connect any external data source.
* 📈 **Scalable (Stem)**: High-throughput pipelines built on gRPC/Protobuf.
* 🔗 **Relational (Rhizome)**: Beyond simple storage—building an organic network of knowledge.
* 🍎 **Actionable (Fruit)**: Transforming retrieved data into decision-ready reports and insights.

---

## 🏗️ Architecture: The Rhizome Metaphor

Inspired by the **Rhizome**—a horizontal, non-hierarchical, and infinitely expandable root system—Gopedia’s architecture ensures that every component is modular yet organically linked.

### 1. Root — *Pluggable Sources*

The "entry point" where nutrients (data) are absorbed. It defines connection standards for external sources like Databases, APIs, Streams, and File Systems.

### 2. Stem — *Scalable Pipelines*

The transport system for data, divided into two vital flows:

* **Phloem (Ingestion)**: **Root → Stem → Rhizome**. Encapsulates raw data into "Envelopes," breaks them down into L1/L2/L3 hierarchies, and records them into the Rhizome via **Smart Sinks**.
* **Xylem (RAG)**: **Rhizome → Leaf/Fruit**. Filters data through L1/L2 structures, retrieves L3 chunks, and delivers them to the Skill Engine for user requests.

### 3. Rhizome — *Relational & Polyglot Storage*

The "Knowledge Soil." This layer handles identity and relationship reasoning using **Polyglot Persistence**:

* **TypeDB / Qdrant**: For relationship reasoning and vector search.
* **PostgreSQL / ClickHouse**: For structured data and high-speed logging.

### 4. Leaf & Fruit — *Views & Actionable Outputs*

* **Leaf (Indexing View)**: Domain-specific views such as Markdown, Code, or Ticket indexes.
* **Fruit (Reports)**: Final templates or generated answers that combine data from multiple Roots and Leaves into a human-readable format.

---

## 📊 Data Hierarchy (L1 → L2 → L3)

Gopedia does not just "chunk" data; it categorizes it into a meaningful hierarchy to ensure high-fidelity retrieval.

| Level | Name | Description |
| --- | --- | --- |
| **L1** | **Identity** | Source provenance and metadata (Author, Date, Source Domain). |
| **L2** | **Structure** | The "Skeleton" of data (Table of Contents, AST structures, Logic flows). |
| **L3** | **Content** | Detailed knowledge chunks and raw content snippets. |

---

## 🚀 Roadmap: Design Phases

We are currently in the **Verify (Germination)** phase, focusing on core source integration.

1. **Verify (Germination) `IN PROGRESS**`: Validating the flow from Markdown and Code (AST) sources into the Rhizome.
2. **Expand (Growth)**: Activating distributed processing via Machine IDs and enhancing DB View visibility.
3. **Connect (Fruition)**: Full integration with the GeneSo ecosystem, featuring complex RAG Fruit (Skill Engine) and ReBAC (SpiceDB).

---

## 📝 License

This project is licensed under the **MIT License**.

---

Gopedia is designed to be the "Single Source of Truth" (SSoT) and the reasoning engine for your enterprise data. For detailed technical specifications (Machine ID, Smart Sinks, Skill Engine), please refer to `reference/gopedia-feature-guide.md`.

