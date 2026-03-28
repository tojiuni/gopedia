"""Reconstruct document text for an L1 snapshot from PostgreSQL (L2/L3 order)."""

from __future__ import annotations

import json
from typing import Any, List, Sequence, Tuple

# Per L2 block: sort_order groups; within L2, L3 ordered by sort_order.
RESTORE_L2_L3_SQL = """
SELECT l2.sort_order,
       l2.section_id,
       l2.source_metadata,
       l3.sort_order AS l3_sort,
       l3.content
FROM knowledge_l2 l2
LEFT JOIN knowledge_l3 l3 ON l3.l2_id = l2.id
WHERE l2.l1_id = %s::uuid
ORDER BY l2.sort_order ASC, l3.sort_order ASC NULLS LAST
"""

RESTORE_L1_META_SQL = """
SELECT k.title, k.source_type, k.source_metadata, k.toc, k.summary
FROM knowledge_l1 k
WHERE k.id = %s::uuid
"""

def _parse_l2_meta(raw: Any) -> dict[str, Any]:
    if raw is None:
        return {}
    if isinstance(raw, dict):
        return raw
    if isinstance(raw, str):
        try:
            return dict(json.loads(raw))
        except json.JSONDecodeError:
            return {}
    return {}


def _format_table(meta: dict[str, Any], row_lines: List[str]) -> str:
    headers = meta.get("headers")
    sep = (meta.get("separator_row") or "").strip()
    if isinstance(headers, list) and headers:
        header_line = "| " + " | ".join(str(h) for h in headers) + " |"
        if not sep:
            sep = "|" + "|".join([" --- " for _ in headers]) + "|"
        body = "\n".join(row_lines)
        parts = [header_line, sep]
        if body.strip():
            parts.append(body.strip())
        return "\n".join(parts).strip()
    return "\n".join(row_lines).strip()


def _format_code(meta: dict[str, Any], lines: List[str]) -> str:
    lang = (meta.get("language") or "").strip()
    body = "\n".join(lines).strip()
    if body.startswith("```"):
        return body
    return f"```{lang}\n{body}\n```".strip()


def _format_block(section_id: str, meta: dict[str, Any], l3_items: List[Tuple[Any, str]]) -> str:
    """l3_items: (sort_order, content) sorted."""
    meta = meta or {}
    bt = (meta.get("block_type") or "").lower()
    sid = section_id or ""

    lines: List[str] = []
    for _so, text in l3_items:
        if text is None:
            continue
        # Preserve indentation for code blocks, but we can right-strip newlines
        s_text = str(text)
        if not s_text.strip():
            continue
        if sid.startswith("c") or bt == "code":
            lines.append(s_text.rstrip("\r\n"))
        else:
            lines.append(s_text.strip())

    if sid.startswith("t") or bt == "table":
        return _format_table(meta, lines)
    if sid.startswith("c") or bt == "code":
        return _format_code(meta, lines)
    if sid.startswith("i") or bt == "image":
        return "\n".join(lines)
        
    if sid.startswith("s") or sid.startswith("o") or bt in ("heading", "ordered"):
        if lines:
            if lines[0].strip().startswith("#"):
                # Always put a blank line after the heading
                heading = lines[0].strip()
                body = " ".join(lines[1:])
                return heading + "\n\n" + body if body else heading
            
            if sid.startswith("o") or bt == "ordered":
                # Ordered lists typically shouldn't be smushed together if they are different items, 
                # but an o* chunk represents ONE item. So its sentences can be joined by space.
                return " ".join(lines)
                
            return " ".join(lines)
            
    if lines:
        return "\n\n".join(lines)
    return ""


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
    Load knowledge_l1 metadata and all L2/L3 under this snapshot, ordered for viewer restore.

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
        cur.execute(RESTORE_L2_L3_SQL, (l1_id,))
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

    blocks: list[str] = []
    cur_key: tuple[Any, str] | None = None
    cur_meta: dict[str, Any] = {}
    cur_sid = ""
    cur_items: List[Tuple[Any, str]] = []

    def flush() -> None:
        nonlocal cur_key, cur_meta, cur_sid, cur_items
        if cur_key is None:
            return
        piece = _format_block(cur_sid, cur_meta, cur_items)
        if piece.strip():
            blocks.append(piece.strip())
        cur_key = None
        cur_meta = {}
        cur_sid = ""
        cur_items = []

    for row in rows:
        l2_sort, section_id, smeta_l2, l3_sort, content = row
        key = (l2_sort, section_id)
        if cur_key is None or key != cur_key:
            flush()
            cur_key = key
            cur_sid = section_id or ""
            cur_meta = _parse_l2_meta(smeta_l2)
        if content is not None:
            cur_items.append((l3_sort, str(content)))
    flush()

    content = "\n\n".join(blocks) if blocks else ""

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
