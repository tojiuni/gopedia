"""
Env snapshot for RegisterProject -> projects.source_metadata (flat string keys).
Must stay aligned with Phloem/Xylem and internal/phloem/sink/writer.go ensurePipelineVersion naming.
"""
from __future__ import annotations

import os


def _set_if_present(out: dict[str, str], key: str, value: str | None) -> None:
    if value is not None and str(value).strip() != "":
        out[key] = str(value).strip()


def pipeline_version_name_from_env() -> str:
    """Mirror ensurePipelineVersion in internal/phloem/sink/writer.go."""
    embed = (os.environ.get("OPENAI_EMBEDDING_MODEL") or "").strip()
    if embed:
        return "v1-" + embed
    return "v1"


def build_register_project_metadata() -> dict[str, str]:
    """Non-secret keys for Xylem / audit; omit empty env values."""
    m: dict[str, str] = {}
    _set_if_present(m, "QDRANT_COLLECTION", os.environ.get("QDRANT_COLLECTION"))
    _set_if_present(m, "QDRANT_VECTOR_NAME", os.environ.get("QDRANT_VECTOR_NAME"))
    _set_if_present(m, "QDRANT_DOC_COLLECTION", os.environ.get("QDRANT_DOC_COLLECTION"))
    _set_if_present(m, "QDRANT_DOC_VECTOR_NAME", os.environ.get("QDRANT_DOC_VECTOR_NAME"))
    _set_if_present(m, "OPENAI_EMBEDDING_MODEL", os.environ.get("OPENAI_EMBEDDING_MODEL"))
    _set_if_present(m, "GOPEDIA_SOURCE_TYPE", os.environ.get("GOPEDIA_SOURCE_TYPE"))
    _set_if_present(m, "domain", os.environ.get("GOPEDIA_INGEST_DOMAIN"))
    if "domain" not in m:
        m["domain"] = "wiki"
    wid = os.environ.get("GOPEDIA_IDENTITY_WORKER_ID") or os.environ.get(
        "GOPEDIA_MARKDOWN_WORKER_ID"
    )
    _set_if_present(m, "GOPEDIA_IDENTITY_WORKER_ID", wid)
    m["pipeline_version_name"] = pipeline_version_name_from_env()
    _set_if_present(m, "GOPEDIA_RERANK", os.environ.get("GOPEDIA_RERANK"))
    return m
