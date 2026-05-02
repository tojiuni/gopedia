"""Embed query, Qdrant L3 search, PostgreSQL rich context."""

from __future__ import annotations

import json
import os
from typing import Any, Dict, List, Optional, Tuple

CONTEXT_FOR_L3_SQL = """
SELECT
  l2.summary AS l2_summary,
  tl3.content AS section_heading,
  l3.id::text AS l3_id,
  l3.content AS matched_content,
  l3.sort_order AS sort_order,
  l2.id::text AS l2_id,
  l2.section_id AS l2_section_id,
  l2.source_metadata AS l2_source_metadata,
  k1.title AS l1_title,
  k1.id::text AS l1_id,
  k1.summary AS l1_summary,
  k1.toc AS l1_toc,
  k1.source_type AS l1_source_type
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

L3_BATCH_CONTENT_SQL = """
SELECT id::text, content
FROM knowledge_l3
WHERE id = ANY(%s::uuid[])
"""

_cross_encoder_cache: Dict[str, Any] = {}


def _rewrite_query(query: str) -> str:
    """Rewrite Korean colloquial query into technical terms using LLM.

    Enabled when GOPEDIA_QUERY_REWRITE=true.  Falls back to original query on
    any error or when the rewrite returns an empty string.
    """
    if os.environ.get("GOPEDIA_QUERY_REWRITE", "false").lower() not in ("true", "1"):
        return query

    ollama_url = os.environ.get("OLLAMA_CHAT_URL", "http://localhost:11434")
    ollama_model = os.environ.get("OLLAMA_CHAT_MODEL", "gemma4:26b")
    import json as _json
    import urllib.request

    prompt = (
        "You are a technical query rewriter. "
        "Rewrite the following query into concise English technical terms "
        "suitable for semantic search in a technical document index. "
        "Output ONLY the rewritten query, no explanation.\n\nQuery: "
        + query
    )
    payload = _json.dumps(
        {"model": ollama_model, "prompt": prompt, "stream": False}
    ).encode()
    req = urllib.request.Request(
        f"{ollama_url.rstrip('/')}/api/generate",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            result = _json.loads(resp.read().decode())
            rewritten = (result.get("response") or "").strip()
            return rewritten if rewritten else query
    except Exception:
        return query


def _get_cross_encoder(model_name: str) -> Any:
    if model_name not in _cross_encoder_cache:
        from sentence_transformers import CrossEncoder
        _cross_encoder_cache[model_name] = CrossEncoder(model_name)
    return _cross_encoder_cache[model_name]


def rerank_candidates(
    query: str,
    rows: List[Tuple[Any, str]],
    conn: Any,
    model_name: str = "BAAI/bge-reranker-v2-m3",
) -> List[Tuple[Any, str]]:
    """Fetch L3 content and rerank using cross-encoder. Returns reranked rows."""
    if not rows:
        return rows
    l3_ids = [lid for _, lid in rows]
    with conn.cursor() as cur:
        cur.execute(L3_BATCH_CONTENT_SQL, (l3_ids,))
        content_rows = cur.fetchall()
    content_map: Dict[str, str] = {r[0]: (r[1] or "") for r in content_rows}
    model = _get_cross_encoder(model_name)
    pairs = [(query, content_map.get(lid, "")) for _, lid in rows]
    scores = model.predict(pairs)
    ranked = sorted(zip(scores, rows), key=lambda x: float(x[0]), reverse=True)
    return [r for _, r in ranked]

def embed_query_openai(query: str, model: Optional[str] = None) -> List[float]:
    from openai import OpenAI

    client = OpenAI(api_key=os.environ.get("OPENAI_API_KEY"))
    m = model or os.environ.get("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
    r = client.embeddings.create(model=m, input=query)
    return list(r.data[0].embedding)


def embed_query_local(query: str, addr: Optional[str] = None) -> List[float]:
    """Embed a query using the local multilingual-e5-large embedding service.

    Uses the "query: " prefix required by multilingual-e5-large for retrieval.
    """
    import urllib.request
    import json as _json

    url = (addr or os.environ.get("LOCAL_EMBEDDING_ADDR", "http://localhost:18789")) + "/embed"
    payload = _json.dumps({"texts": [query], "prefix": "query"}).encode()
    req = urllib.request.Request(url, data=payload, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req) as resp:
        data = _json.loads(resp.read())
    return data["embeddings"][0]


def qdrant_search_l3_points(
    query_vector: List[float],
    host: str,
    port: int,
    collection: str,
    limit: int = 5,
    vector_name: Optional[str] = None,
    project_id_filter: Optional[int] = None,
) -> List[Any]:
    from qdrant_client import QdrantClient
    from qdrant_client.http.models import FieldCondition, Filter, MatchValue

    _api_key = os.environ.get("QDRANT_API_KEY") or None
    qc = QdrantClient(host=host, port=port, api_key=_api_key, https=False)
    kwargs: dict = {}
    if vector_name:
        kwargs["using"] = vector_name
    if project_id_filter is not None:
        kwargs["query_filter"] = Filter(
            must=[
                FieldCondition(
                    key="project_id",
                    match=MatchValue(value=int(project_id_filter)),
                )
            ]
        )
    result = qc.query_points(
        collection_name=collection,
        query=query_vector,
        limit=limit,
        **kwargs,
    )
    return list(result.points or [])


def _decode_byte_summary(blob: Any) -> str:
    if blob is None:
        return ""
    if isinstance(blob, memoryview):
        blob = bytes(blob)
    if isinstance(blob, bytes):
        return blob.decode("utf-8", errors="replace")
    return str(blob)


def _breadcrumb(l1_title: str, section_heading: str) -> str:
    doc = (l1_title or "").strip() or "(untitled)"
    sec = (section_heading or "").strip() or "(section)"
    return f"[문서: {doc}] > [섹션: {sec}]"


def _token_count(text: str, model: Optional[str] = None) -> int:
    try:
        import tiktoken

        m = model or os.environ.get("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
        try:
            enc = tiktoken.encoding_for_model(m)
        except KeyError:
            enc = tiktoken.get_encoding("cl100k_base")
        return len(enc.encode(text or ""))
    except Exception:
        return max(1, len((text or "").split()) * 2)


def _toc_to_str(toc: Any) -> str:
    if toc is None:
        return ""
    if isinstance(toc, (dict, list)):
        try:
            return json.dumps(toc, ensure_ascii=False)
        except TypeError:
            return str(toc)
    return str(toc)


def fetch_rich_context(
    conn: Any,
    l3_id: str,
    neighbor_window: int = 2000,
    level: int = 1,
    max_tokens: Optional[int] = None,
    embedding_model: Optional[str] = None,
) -> dict:
    """
    level:
      0 — matched L3 only (no neighbor window).
      1 — neighbors within neighbor_window (sort_order span).
      2 — emphasize L2 summary; tight neighbor span.
      3 — adds L1 summary + TOC; neighbor span like level 1.
    """
    with conn.cursor() as cur:
        cur.execute(CONTEXT_FOR_L3_SQL, (l3_id,))
        row = cur.fetchone()
        if not row:
            return {}
        (
            l2_summary,
            section_heading,
            lid,
            matched,
            sort_order,
            l2_id,
            l2_section_id,
            l2_source_metadata,
            l1_title,
            l1_id,
            l1_summary_blob,
            l1_toc,
            l1_source_type,
        ) = row

    so = int(sort_order) if sort_order is not None else 0

    window_parts: List[str] = []
    if level == 0:
        pass
    elif level == 2:
        span = min(neighbor_window, 1500)
        win_lo = max(0, so - span)
        win_hi = so + span
        with conn.cursor() as cur:
            cur.execute(NEIGHBORS_SQL, (l2_id, win_lo, win_hi))
            neighbors = cur.fetchall()
        window_parts = [str(c[0]).strip() for c in neighbors if c[0] and str(c[0]).strip()]
    else:
        win_lo = max(0, so - neighbor_window)
        win_hi = so + neighbor_window
        with conn.cursor() as cur:
            cur.execute(NEIGHBORS_SQL, (l2_id, win_lo, win_hi))
            neighbors = cur.fetchall()
        window_parts = [str(c[0]).strip() for c in neighbors if c[0] and str(c[0]).strip()]

    if level == 2:
        # Prefer section summary + matched line; keep neighbors minimal in the assembled narrative.
        head = "\n\n".join([p for p in [l2_summary or "", matched or ""] if p.strip()])
        window_text = head if head.strip() else "\n\n".join(window_parts)
    else:
        window_text = "\n\n".join(window_parts)

    l1_summary_text = _decode_byte_summary(l1_summary_blob)
    l1_toc_str = _toc_to_str(l1_toc)

    l2_meta: dict | Any = {}
    if isinstance(l2_source_metadata, dict):
        l2_meta = l2_source_metadata
    elif isinstance(l2_source_metadata, str):
        try:
            l2_meta = json.loads(l2_source_metadata)
        except json.JSONDecodeError:
            l2_meta = {}

    source_path = l1_title if (l1_source_type or "") == "code" else ""
    doc_name = l2_meta.get("name") or l2_meta.get("project_id", "") if l2_meta else ""

    ctx: dict = {
        "breadcrumb": _breadcrumb(l1_title or "", section_heading or ""),
        "l1_id": l1_id or "",
        "l1_title": l1_title or "",
        "l2_id": str(l2_id) if l2_id is not None else "",
        "l2_summary": l2_summary or "",
        "section_heading": section_heading or "",
        "l2_section_id": l2_section_id or "",
        "l2_block_type": (l2_meta.get("block_type") or "") if l2_meta else "",
        "matched_l3_id": lid,
        "matched_content": matched or "",
        "surrounding_context": window_text,
        "source_path": source_path,
        "doc_name": doc_name,
    }
    if level >= 3:
        ctx["l1_summary"] = l1_summary_text
        ctx["l1_toc"] = l1_toc_str

    if max_tokens is not None and max_tokens > 0:
        embed_model = embedding_model or os.environ.get(
            "OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"
        )
        fixed_keys = ("breadcrumb", "l1_title", "l2_summary", "section_heading", "matched_content")
        if level >= 3:
            fixed_keys = fixed_keys + ("l1_summary", "l1_toc")
        fixed_text = "\n".join(str(ctx.get(k, "")) for k in fixed_keys)
        used = _token_count(fixed_text, embed_model)
        budget = max_tokens - used
        if budget <= 0:
            ctx["surrounding_context"] = ""
            return ctx
        acc: List[str] = []
        for part in window_parts:
            trial = "\n\n".join(acc + [part]) if acc else part
            if _token_count(trial, embed_model) <= budget:
                acc.append(part)
            else:
                break
        ctx["surrounding_context"] = "\n\n".join(acc)

    return ctx


def retrieve_and_enrich(
    query: str,
    conn: Any,
    qdrant_host: Optional[str] = None,
    qdrant_port: Optional[int] = None,
    collection: Optional[str] = None,
    vector_name: Optional[str] = None,
    candidate_limit: int = 30,
    final_limit: int = 5,
    neighbor_window: int = 2000,
    context_level: int = 1,
    max_tokens: Optional[int] = None,
    limit: Optional[int] = None,
    project_id: Optional[int] = None,
    embedding_model: Optional[str] = None,
    use_reranker: Optional[bool] = None,
    reranker_model: Optional[str] = None,
) -> List[dict]:
    """If ``limit`` is passed (legacy), it overrides ``final_limit``."""
    query = _rewrite_query(query)
    if use_reranker is None:
        use_reranker = os.environ.get("GOPEDIA_RERANKER_ENABLED", "false").lower() in ("true", "1")
    from flows.xylem_flow.project_config import (
        fetch_project_source_metadata,
        resolve_retrieval_settings,
    )

    if limit is not None:
        final_limit = int(limit)

    meta: dict[str, str] = {}
    if project_id is not None:
        meta = fetch_project_source_metadata(conn, project_id)

    resolved = resolve_retrieval_settings(
        meta=meta,
        qdrant_host=qdrant_host,
        qdrant_port=qdrant_port,
        collection=collection,
        vector_name=vector_name,
        embedding_model=embedding_model,
    )

    if resolved.embedding_backend == "local":
        vec = embed_query_local(query, addr=resolved.local_embedding_addr)
    else:
        vec = embed_query_openai(query, model=resolved.embedding_model)
    hits = qdrant_search_l3_points(
        vec,
        host=resolved.qdrant_host,
        port=resolved.qdrant_port,
        collection=resolved.collection,
        limit=candidate_limit,
        vector_name=resolved.vector_name,
        project_id_filter=project_id,
    )
    rows: List[Tuple[Any, str]] = []
    for h in hits:
        payload = h.payload or {}
        lid = payload.get("l3_id")
        if not lid:
            continue
        rows.append((h, str(lid)))

    if use_reranker and rows:
        _rmodel = reranker_model or os.environ.get(
            "GOPEDIA_RERANKER_MODEL", "BAAI/bge-reranker-v2-m3"
        )
        rows = rerank_candidates(query, rows, conn, model_name=_rmodel)

    rows = rows[:final_limit]

    out: List[dict] = []
    for h, lid in rows:
        ctx = fetch_rich_context(
            conn,
            lid,
            neighbor_window=neighbor_window,
            level=context_level,
            max_tokens=max_tokens,
            embedding_model=resolved.embedding_model,
        )
        if not ctx:
            continue
        ctx["qdrant_score"] = getattr(h, "score", None)
        payload = getattr(h, "payload", None) or {}
        if hasattr(payload, "items"):
            pd = dict(payload)
        elif isinstance(payload, dict):
            pd = payload
        else:
            pd = {}
        if pd.get("project_id") is not None:
            ctx["project_id"] = pd.get("project_id")
        if pd.get("doc_id") is not None and pd.get("doc_id") != "":
            ctx["doc_id"] = pd.get("doc_id")
        if pd.get("l2_id") and not ctx.get("l2_id"):
            ctx["l2_id"] = str(pd.get("l2_id"))
        out.append(ctx)
    return out
