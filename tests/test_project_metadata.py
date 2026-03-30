"""Unit tests for property.root_props.project_metadata."""

from __future__ import annotations

from property.root_props.project_metadata import (
    build_register_project_metadata,
    pipeline_version_name_from_env,
)


def test_pipeline_version_name_from_env(monkeypatch) -> None:
    monkeypatch.delenv("OPENAI_EMBEDDING_MODEL", raising=False)
    assert pipeline_version_name_from_env() == "v1"
    monkeypatch.setenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
    assert pipeline_version_name_from_env() == "v1-text-embedding-3-small"


def test_build_register_project_metadata(monkeypatch) -> None:
    monkeypatch.setenv("QDRANT_COLLECTION", "my_coll")
    monkeypatch.setenv("OPENAI_EMBEDDING_MODEL", "m1")
    m = build_register_project_metadata()
    assert m["QDRANT_COLLECTION"] == "my_coll"
    assert m["OPENAI_EMBEDDING_MODEL"] == "m1"
    assert m["domain"] == "wiki"
    assert m["pipeline_version_name"] == "v1-m1"


def test_build_register_domain_from_env(monkeypatch) -> None:
    monkeypatch.delenv("OPENAI_EMBEDDING_MODEL", raising=False)
    monkeypatch.setenv("GOPEDIA_INGEST_DOMAIN", "wiki")
    m = build_register_project_metadata()
    assert m["domain"] == "wiki"
