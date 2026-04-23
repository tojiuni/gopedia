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

## HTTP API + CLI (Fuego)

- **API server**: `go run ./cmd/api` — listens on `GOPEDIA_HTTP_ADDR` (default `127.0.0.1:8787`). Routes: `GET /api/health`, `GET /api/search?q=...`, `POST /api/ingest` with JSON `{"path":"/abs/path"}`.
- **CLI client**: `go run ./cmd/gopedia …` — talks to `GOPEDIA_API_URL` (default `http://127.0.0.1:8787`). Examples: `gopedia server`, `gopedia search "Introduction"`, `gopedia ingest /path/to/project`.
- **Python**: the API runs `python3 -m property.root_props.run` and `python3 -m flows.xylem_flow.cli` from the repo root. Set `GOPEDIA_REPO_ROOT` if `go.mod` is not discoverable from the process cwd.

---

## 📚 Documentation

For detailed architecture diagrams, pipeline specifications, and database schemas, please refer to the **[Rev2 Design Documentation](./doc/design/Rev2/01-overview.md)**.

## 설치/시나리오 가이드 (Korean)

사전 요구 사항 : 설치에 필요한 최소 환경 (K8s 버전, CPU/Memory, 필수 도구 등)

- K8s `v1.28+` 또는 Docker Compose 개발 환경
- 최소 `4 vCPU / 8GB RAM` (3개 조합 권장 `8 vCPU / 16GB RAM`)
- 필수 도구: `git`, `docker`, `docker compose`, (선택) `go`, `python`, `node`

설치 (5분 이내)

- 복사-붙여넣기 가능한 설치 명령어 (현재 가이드는 Docker Compose 기준)
- 로컬 빠른 설치는 아래 가이드의 Compose 명령을 그대로 사용
- 상세: [`doc/guide/install.md`](./doc/guide/install.md)
- 요약: [`doc/guide/quick-install-guide.md`](./doc/guide/quick-install-guide.md)

설치 확인 방법 ("이 화면이 뜨면 성공")

- `curl http://127.0.0.1:18787/api/health` 응답 JSON이 확인되면 성공
- `GET /api/search?q=test` 호출 시 결과가 반환되면 정상

삭제 방법

- `docker compose -f docker-compose.dev.yml --env-file .env down -v`

첫 번째 시나리오 (10분 이내)

- 설치 직후 바로 실행할 수 있는 데모 시나리오 1개
- Obsidian Vault에 샘플 노트를 만들고 ingest 후 검색 API 결과 확인
- 이후 [gardener_gopedia](https://github.com/tojiuni/gardener_gopedia/blob/main/README.md)로 품질 측정, [gopedia_mcp](https://github.com/tojiuni/gopedia_mcp/blob/main/README.md)로 Agent 질의 재현

다음 단계 안내 : 프로덕션 적용을 원하시면 [contact@cloudbro.ai](mailto:contact@cloudbro.ai)로 문의 - 컨택 채널은 꼭 cloudbro로 부탁드립니다!

---

## 📝 License

This project is licensed under the **Apache 2.0 License**.
