"""Reconstruct markdown for an L1 snapshot from PostgreSQL (L2 sort_order, L3 sort_order)."""

from __future__ import annotations

from typing import Any, Sequence

# Join all L3 rows under the L1 in document order: section order then atomic order within section.
RESTORE_L3_FOR_L1_SQL = """
SELECT l3.content
FROM knowledge_l3 l3
INNER JOIN knowledge_l2 l2 ON l3.l2_id = l2.id
WHERE l2.l1_id = %s
ORDER BY l2.sort_order ASC, l3.sort_order ASC
"""


def rows_to_markdown(rows: Sequence[tuple[Any, ...]]) -> str:
    """Join query result rows (single text column) into markdown with blank lines between blocks."""
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
    return "\n\n".join(parts)


def restore_markdown_for_l1(conn: Any, l1_id: str) -> str:
    """
    Flatten L2/L3 under knowledge_l1.id = l1_id into one markdown string.
    Expects a psycopg connection (v3) or any connection with .cursor() returning a cursor
    supporting execute/fetchall.
    """
    with conn.cursor() as cur:
        cur.execute(RESTORE_L3_FOR_L1_SQL, (l1_id,))
        rows = cur.fetchall()
    return rows_to_markdown(rows)
