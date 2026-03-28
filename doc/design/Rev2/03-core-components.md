# 03. Core Components

The Gopedia platform consists of multiple specialized services working together. This modularity allows us to use the best tool for the job (e.g., Go for high-throughput I/O, Python for NLP and Machine Learning).

## 1. Component Overview
* **Phloem gRPC Server**: The main ingestion gateway, written in Go.
* **NLP Worker**: A Python-based worker for linguistic processing.
* **Xylem Restorer**: The retrieval and formatting engine, written in Python.

## 2. Component References
For detailed technical specifications of each component, refer to the documents below:

👉 **[Go Phloem Server (`cmd/phloem/`)](./references/go-phloem-server.md)**: Domain-driven ingestion, TOC chunking, Smart Sink.

👉 **[Python NLP Worker (`python/nlp_worker/`)](./references/python-nlp-worker.md)**: gRPC Unary service, sentence splitting, text masking.

👉 **[Xylem Restorer (`flows/xylem_flow/restorer.py`)](./references/xylem-restorer.md)**: L1/L2/L3 data retrieval and contextual reconstruction.
