# Python NLP Worker Details

**Implementation Path:** `python/nlp_worker/`

## 1. Role
A Unary gRPC Python service responsible for linguistic operations, primarily L3 sentence splitting, designed to work tightly with the Go Phloem Server.

## 2. Features
*   **Masking**: Substitutes markdown links, paths, and versions with placeholders before sentence splitting to prevent incorrect period (`.`) evaluation.
*   **Language-Aware Splitting**: Uses `kss` when the Korean ratio exceeds a configurable threshold (`GOPEDIA_NLP_LANG_HANGUL_RATIO`), and falls back to `pysbd` for English-heavy content.
*   **Future scope**: Named Entity Recognition (NER) for knowledge graph extraction.

## 3. Communication
*   **Protocol**: gRPC + Protobuf.
*   **Fallback**: If the Python worker is unreachable, Phloem falls back to a basic Go-based punctuation splitter.
