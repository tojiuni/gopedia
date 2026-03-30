"""Unit tests for flows.xylem_flow.project_config."""

from __future__ import annotations

from unittest.mock import MagicMock

from flows.xylem_flow.project_config import (
    fetch_project_source_metadata,
    resolve_retrieval_settings,
)


def test_resolve_explicit_over_meta_over_env() -> None:
    meta = {
        "QDRANT_COLLECTION": "from_meta",
        "OPENAI_EMBEDDING_MODEL": "meta-model",
    }
    env = {
        "QDRANT_COLLECTION": "from_env",
        "OPENAI_EMBEDDING_MODEL": "env-model",
        "QDRANT_HOST": "env-host",
        "QDRANT_PORT": "6333",
    }
    r = resolve_retrieval_settings(meta=meta, env=env, collection="explicit")
    assert r.collection == "explicit"
    assert r.embedding_model == "meta-model"
    r2 = resolve_retrieval_settings(meta=meta, env=env)
    assert r2.collection == "from_meta"
    r3 = resolve_retrieval_settings(meta={}, env=env)
    assert r3.collection == "from_env"
    assert r3.embedding_model == "env-model"

def test_resolve_embedding_model_explicit() -> None:
    env = {
        "QDRANT_HOST": "localhost",
        "QDRANT_PORT": "6333",
        "QDRANT_COLLECTION": "c",
        "OPENAI_EMBEDDING_MODEL": "env-m",
    }
    r = resolve_retrieval_settings(
        meta={"OPENAI_EMBEDDING_MODEL": "meta-m"},
        env=env,
        embedding_model="arg-m",
    )
    assert r.embedding_model == "arg-m"


def test_fetch_project_source_metadata_normalizes() -> None:
    conn = MagicMock()
    cur = MagicMock()
    conn.cursor.return_value.__enter__.return_value = cur
    cur.fetchone.return_value = ({"QDRANT_COLLECTION": "c1", "n": 42},)

    m = fetch_project_source_metadata(conn, 7)
    assert m["QDRANT_COLLECTION"] == "c1"
    assert m["n"] == "42"
    cur.execute.assert_called_once()


def test_fetch_project_source_metadata_missing_row() -> None:
    conn = MagicMock()
    cur = MagicMock()
    conn.cursor.return_value.__enter__.return_value = cur
    cur.fetchone.return_value = None

    assert fetch_project_source_metadata(conn, 99) == {}
