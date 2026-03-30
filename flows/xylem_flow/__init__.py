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
from flows.xylem_flow.tree import build_project_l1_tree, get_project_tree_for_viewer

__all__ = [
    "build_project_l1_tree",
    "fetch_rich_context",
    "get_project_tree_for_viewer",
    "restore_content_for_l1",
    "restore_markdown_for_l1",
    "retrieve_and_enrich",
    "rows_join_for_format",
    "rows_to_markdown",
]
