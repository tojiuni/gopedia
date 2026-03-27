"""Embed query, Qdrant L3 search, PostgreSQL rich context."""

from __future__ import annotations

import os
from typing import Any, List, Optional

CONTEXT_FOR_L3_SQL = """
SELECT
  l2.summary AS l2_summary,
  tl3.content AS section_heading,
  l3.id::text AS l3_id,
  l3.content AS matched_content,
  l3.sort_order AS sort_order,
  l2.id::text AS l2_id,
  k1.title AS l1_title
FROM knowledge_l3 l3
JOIN knowledge_l2 l2 ON l3.l2_id = l2.id
JOIN knowledge_l1 k1 ON l2.l1_id = k1.id
LEFT JOIN knowledge_l3 tl3 ON l2.title_id = tl3.id
WHERE l3.id = %s::uuid
"""

NEIGHBORS_SQL = """
SELECT l3.content, l3.sort_order
FROM knowledge_l3 l3
WHERE l3.l2_id = %s::uuid
  AND l3.sort_order BETWEEN %s AND %s
ORDER BY l3.sort_order ASC
"""


def embed_query_openai(query: str, model: Optional[str] = None) -> List[float]:
    from openai import OpenAI

    client = OpenAI(api_key=os.environ.get("OPENAI_API_KEY"))
    m = model or os.environ.get("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
    r = client.embeddings.create(model=m, input=query)
    return list(r.data[0].embedding)


def qdrant_search_l3_points(
    query_vector: List[float],
    host: str,
    port: int,
    collection: str,
    limit: int = 5,
    vector_name: Optional[str] = None,
) -> List[Any]:
    from qdrant_client import QdrantClient

    qc = QdrantClient(host=host, port=port)
    kwargs: dict = {}
    if vector_name:
        kwargs["using"] = vector_name
    result = qc.query_points(
        collection_name=collection,
        query=query_vector,
        limit=limit,
        **kwargs,
    )
    return list(result.points or [])


def fetch_rich_context(conn: Any, l3_id: str, neighbor_window: int = 2000) -> dict:
    with conn.cursor() as cur:
        cur.execute(CONTEXT_FOR_L3_SQL, (l3_id,))
        row = cur.fetchone()
        if not row:
            return {}
        l2_summary, section_heading, lid, matched, sort_order, l2_id, l1_title = row
        so = int(sort_order) if sort_order is not None else 0
        lo = max(0, so - neighbor_window)
        hi = so + neighbor_window
        cur.execute(NEIGHBORS_SQL, (l2_id, lo, hi))
        neighbors = cur.fetchall()
    window_parts = [str(c[0]).strip() for c in neighbors if c[0] and str(c[0]).strip()]
    window_text = "\n\n".join(window_parts)
    return {
        "l1_title": l1_title or "",
        "l2_summary": l2_summary or "",
        "section_heading": section_heading or "",
        "matched_l3_id": lid,
        "matched_content": matched or "",
        "surrounding_context": window_text,
    }


def retrieve_and_enrich(
    query: str,
    conn: Any,
    qdrant_host: Optional[str] = None,
    qdrant_port: Optional[int] = None,
    collection: Optional[str] = None,
    vector_name: Optional[str] = None,
    limit: int = 5,
    neighbor_window: int = 2000,
) -> List[dict]:
    host = qdrant_host or os.environ.get("QDRANT_HOST", "localhost")
    port = qdrant_port if qdrant_port is not None else int(os.environ.get("QDRANT_PORT", "6333"))
    coll = collection or os.environ.get("QDRANT_COLLECTION", "gopedia")
    vn = vector_name if vector_name is not None else (os.environ.get("QDRANT_VECTOR_NAME") or None)

    vec = embed_query_openai(query)
    hits = qdrant_search_l3_points(
        vec, host=host, port=port, collection=coll, limit=limit, vector_name=vn
    )
    out: List[dict] = []
    for h in hits:
        payload = h.payload or {}
        lid = payload.get("l3_id")
        if not lid:
            continue
        ctx = fetch_rich_context(conn, str(lid), neighbor_window=neighbor_window)
        if not ctx:
            continue
        ctx["qdrant_score"] = getattr(h, "score", None)
        out.append(ctx)
    return out
