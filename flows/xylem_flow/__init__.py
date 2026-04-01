"""Xylem flow: PostgreSQL-backed markdown restore and context-enriched RAG helpers."""

from __future__ import annotations

from flows.xylem_flow.restorer import (
    restore_content_for_l1,
    restore_markdown_for_l1,
    rows_join_for_format,
    rows_to_markdown,
)
from flows.xylem_flow.retriever import (
    fetch_rich_context,
    retrieve_and_enrich,
)

__all__ = [
    "fetch_rich_context",
    "restore_content_for_l1",
    "restore_markdown_for_l1",
    "retrieve_and_enrich",
    "rows_join_for_format",
    "rows_to_markdown",
]
