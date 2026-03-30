"""Load projects.source_metadata and merge with env for Xylem retrieval."""

from __future__ import annotations

import json
import os
from dataclasses import dataclass
from typing import Any, Mapping, Optional


def fetch_project_source_metadata(conn: Any, project_id: int) -> dict[str, str]:
    """Return flat string map from projects.source_metadata (JSONB)."""
    with conn.cursor() as cur:
        cur.execute(
            "SELECT source_metadata FROM projects WHERE id = %s",
            (project_id,),
        )
        row = cur.fetchone()
    if not row or row[0] is None:
        return {}
    raw = row[0]
    if isinstance(raw, str):
        try:
            raw = json.loads(raw)
        except json.JSONDecodeError:
            return {}
    if not isinstance(raw, dict):
        return {}
    out: dict[str, str] = {}
    for k, v in raw.items():
        if v is None:
            continue
        if isinstance(v, str):
            out[str(k)] = v
        elif isinstance(v, (dict, list)):
            out[str(k)] = json.dumps(v, ensure_ascii=False)
        else:
            out[str(k)] = str(v)
    return out


@dataclass(frozen=True)
class ResolvedRetrievalSettings:
    qdrant_host: str
    qdrant_port: int
    collection: str
    vector_name: Optional[str]
    embedding_model: str


def _meta_str(meta: Mapping[str, str], key: str) -> Optional[str]:
    v = meta.get(key)
    if v is None or str(v).strip() == "":
        return None
    return str(v).strip()


def resolve_retrieval_settings(
    *,
    meta: dict[str, str],
    env: Optional[Mapping[str, str]] = None,
    qdrant_host: Optional[str] = None,
    qdrant_port: Optional[int] = None,
    collection: Optional[str] = None,
    vector_name: Optional[str] = None,
    embedding_model: Optional[str] = None,
) -> ResolvedRetrievalSettings:
    """
    Priority: explicit keyword args (non-None) > project source_metadata > env defaults.
    For vector_name, explicit empty string means unnamed default vector (None).
    """
    env = env or os.environ

    def pick_host() -> str:
        if qdrant_host is not None:
            return qdrant_host
        v = _meta_str(meta, "QDRANT_HOST")
        if v is not None:
            return v
        return env.get("QDRANT_HOST", "localhost")

    def pick_port() -> int:
        if qdrant_port is not None:
            return qdrant_port
        v = _meta_str(meta, "QDRANT_PORT")
        if v is not None:
            return int(v)
        return int(env.get("QDRANT_PORT", "6333"))

    def pick_collection() -> str:
        if collection is not None:
            return collection
        v = _meta_str(meta, "QDRANT_COLLECTION")
        if v is not None:
            return v
        return env.get("QDRANT_COLLECTION", "gopedia")

    def pick_vector_name() -> Optional[str]:
        if vector_name is not None:
            return vector_name or None
        v = _meta_str(meta, "QDRANT_VECTOR_NAME")
        if v is not None:
            return v
        vn = env.get("QDRANT_VECTOR_NAME") or None
        return vn if vn else None

    def pick_embedding_model() -> str:
        if embedding_model is not None:
            return embedding_model
        v = _meta_str(meta, "OPENAI_EMBEDDING_MODEL")
        if v is not None:
            return v
        return env.get("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")

    return ResolvedRetrievalSettings(
        qdrant_host=pick_host(),
        qdrant_port=pick_port(),
        collection=pick_collection(),
        vector_name=pick_vector_name(),
        embedding_model=pick_embedding_model(),
    )
