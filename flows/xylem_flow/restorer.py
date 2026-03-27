"""Reconstruct document text for an L1 snapshot from PostgreSQL (L2/L3 order)."""

from __future__ import annotations

import json
from typing import Any, List, Sequence

# Join L3 rows in document order: section order then atomic order within section.
RESTORE_L3_ORDERED_SQL = """
SELECT l3.content
FROM knowledge_l3 l3
INNER JOIN knowledge_l2 l2 ON l3.l2_id = l2.id
WHERE l2.l1_id = %s::uuid
ORDER BY l2.sort_order ASC, l3.sort_order ASC
"""

RESTORE_L1_META_SQL = """
SELECT k.title, k.source_type, k.source_metadata, k.toc, k.summary
FROM knowledge_l1 k
WHERE k.id = %s::uuid
"""


def rows_join_for_format(rows: Sequence[tuple[Any, ...]], source_type: str) -> str:
    """Join L3 text rows; markdown uses paragraph breaks, code-like types use single newlines."""
    parts: list[str] = []
    for r in rows:
        if not r:
            continue
        cell = r[0]
        if cell is None:
            continue
        s = str(cell).strip()
        if s:
            parts.append(s)
    if not parts:
        return ""
    st = (source_type or "md").lower()
    if st in ("md", "markdown", "wiki"):
        return "\n\n".join(parts)
    return "\n".join(parts)


def rows_to_markdown(rows: Sequence[tuple[Any, ...]]) -> str:
    """Join query result rows (single text column) into markdown with blank lines between blocks."""
    return rows_join_for_format(rows, "md")


def restore_content_for_l1(conn: Any, l1_id: str) -> dict[str, Any]:
    """
    Load knowledge_l1 metadata and all L3 content under this snapshot, ordered for viewer restore.

    Returns a dict: content, title, source_type, source_metadata, toc, l1_summary, l1_id.
    """
    with conn.cursor() as cur:
        cur.execute(RESTORE_L1_META_SQL, (l1_id,))
        meta = cur.fetchone()
        if not meta:
            return {
                "l1_id": l1_id,
                "content": "",
                "title": "",
                "source_type": "md",
                "source_metadata": {},
                "toc": [],
                "l1_summary": "",
            }
        title, source_type, smeta_raw, toc_raw, summary_blob = meta
        cur.execute(RESTORE_L3_ORDERED_SQL, (l1_id,))
        rows = cur.fetchall()

    smeta: dict | Any
    if smeta_raw is None:
        smeta = {}
    elif isinstance(smeta_raw, dict):
        smeta = smeta_raw
    else:
        try:
            smeta = json.loads(smeta_raw) if isinstance(smeta_raw, str) else {}
        except json.JSONDecodeError:
            smeta = {}

    toc: list | Any
    if toc_raw is None:
        toc = []
    elif isinstance(toc_raw, (list, dict)):
        toc = toc_raw
    else:
        try:
            toc = json.loads(toc_raw) if isinstance(toc_raw, str) else []
        except json.JSONDecodeError:
            toc = []

    summary_text = ""
    if summary_blob is not None:
        if isinstance(summary_blob, memoryview):
            summary_blob = bytes(summary_blob)
        if isinstance(summary_blob, bytes):
            summary_text = summary_blob.decode("utf-8", errors="replace")
        else:
            summary_text = str(summary_blob)

    st = (source_type or "md").lower()
    content = rows_join_for_format(rows, st)

    return {
        "l1_id": l1_id,
        "content": content,
        "title": title or "",
        "source_type": st,
        "source_metadata": smeta,
        "toc": toc,
        "l1_summary": summary_text,
    }


def restore_markdown_for_l1(conn: Any, l1_id: str) -> str:
    """
    Flatten L2/L3 under knowledge_l1.id = l1_id into one string (format-aware via source_type).
    """
    return restore_content_for_l1(conn, l1_id).get("content") or ""
