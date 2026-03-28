# Xylem Restorer Details

**Implementation Path:** `flows/xylem_flow/restorer.py`

## 1. Role
The Restorer is responsible for assembling L3 atomic chunks back into their correct L2 structural format, ensuring that retrieved text provides maximum context to the LLM.

## 2. Reconstruction Logic
*   Executes SQL queries against `knowledge_l1`, `knowledge_l2`, and `knowledge_l3` ordered by `sort_order`.
*   Parses `source_metadata` from L2 blocks.
*   Uses block formatters (`_format_table`, `_format_code`, `_format_block`) to render structured data correctly based on its original markdown/AST context.
