"""Unit + integration tests for flows.xylem_flow (restore + retriever)."""

from __future__ import annotations

import os

import pytest

from flows.xylem_flow import restorer as restorer_mod
from flows.xylem_flow.restorer import (
    restore_content_for_l1,
    rows_join_for_format,
    rows_to_markdown,
    restore_markdown_for_l1,
)


def test_rows_to_markdown_joins_non_empty() -> None:
    rows = [("# A",), ("",), (None,), ("## B",)]
    assert rows_to_markdown(rows) == "# A\n\n## B"


def test_rows_to_markdown_empty() -> None:
    assert rows_to_markdown([]) == ""
    assert rows_to_markdown([(None,)]) == ""


def test_format_block_table_rebuilds_pipe_table() -> None:
    meta = {
        "block_type": "table",
        "headers": ["a", "b"],
        "separator_row": "|---|---|",
    }
    got = restorer_mod._format_block("t1", meta, [(1000, "| 1 | 2 |")])
    assert "| a |" in got
    assert "| 1 |" in got


def test_rows_join_for_format_code_uses_single_newline() -> None:
    rows = [("line1",), ("line2",)]
    assert rows_join_for_format(rows, "go") == "line1\nline2"
    assert rows_join_for_format(rows, "md") == "line1\n\nline2"


def _postgres_configured() -> bool:
    return bool(os.environ.get("POSTGRES_HOST") and os.environ.get("POSTGRES_USER"))


@pytest.mark.integration
@pytest.mark.skipif(not _postgres_configured(), reason="POSTGRES_HOST/POSTGRES_USER not set")
def test_restore_markdown_for_l1_latest_snapshot() -> None:
    import psycopg

    conninfo = (
        f"host={os.environ.get('POSTGRES_HOST','')} "
        f"port={os.environ.get('POSTGRES_PORT','5432')} "
        f"user={os.environ.get('POSTGRES_USER','')} "
        f"password={os.environ.get('POSTGRES_PASSWORD','')} "
        f"dbname={os.environ.get('POSTGRES_DB','gopedia')} sslmode=disable"
    )
    with psycopg.connect(conninfo) as conn:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id::text FROM knowledge_l1 ORDER BY created_at DESC NULLS LAST LIMIT 1"
            )
            row = cur.fetchone()
        if not row:
            pytest.skip("no knowledge_l1 rows")
        l1_id = row[0]
        bundle = restore_content_for_l1(conn, l1_id)
        md = bundle.get("content") or ""
    assert md, "restored markdown should be non-empty after ingest"
    assert bundle.get("title") is not None
    assert bundle.get("source_type")
    # After Phloem heading preservation, expect at least one markdown heading line in body
    assert "#" in md


@pytest.mark.integration
@pytest.mark.skipif(not _postgres_configured(), reason="POSTGRES_HOST/POSTGRES_USER not set")
def test_knowledge_l2_source_metadata_column_exists() -> None:
    import psycopg

    conninfo = (
        f"host={os.environ.get('POSTGRES_HOST','')} "
        f"port={os.environ.get('POSTGRES_PORT','5432')} "
        f"user={os.environ.get('POSTGRES_USER','')} "
        f"password={os.environ.get('POSTGRES_PASSWORD','')} "
        f"dbname={os.environ.get('POSTGRES_DB','gopedia')} sslmode=disable"
    )
    with psycopg.connect(conninfo) as conn:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'public' AND table_name = 'knowledge_l2'
                  AND column_name = 'source_metadata'
                """
            )
            assert cur.fetchone() is not None


@pytest.mark.integration
@pytest.mark.skipif(not _postgres_configured(), reason="POSTGRES_HOST/POSTGRES_USER not set")
def test_knowledge_l2_title_id_column_exists() -> None:
    import psycopg

    conninfo = (
        f"host={os.environ.get('POSTGRES_HOST','')} "
        f"port={os.environ.get('POSTGRES_PORT','5432')} "
        f"user={os.environ.get('POSTGRES_USER','')} "
        f"password={os.environ.get('POSTGRES_PASSWORD','')} "
        f"dbname={os.environ.get('POSTGRES_DB','gopedia')} sslmode=disable"
    )
    with psycopg.connect(conninfo) as conn:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'public' AND table_name = 'knowledge_l2' AND column_name = 'title_id'
                """
            )
            assert cur.fetchone() is not None


@pytest.mark.integration
@pytest.mark.skipif(
    not _postgres_configured()
    or not os.environ.get("OPENAI_API_KEY")
    or not os.environ.get("QDRANT_HOST"),
    reason="POSTGRES + OPENAI_API_KEY + QDRANT_HOST required",
)
def test_retrieve_and_enrich_returns_context() -> None:
    import psycopg

    from flows.xylem_flow.retriever import retrieve_and_enrich

    # Avoid downloading CrossEncoder weights in CI unless explicitly enabled.
    os.environ.setdefault("GOPEDIA_RERANK", "0")

    conninfo = (
        f"host={os.environ.get('POSTGRES_HOST','')} "
        f"port={os.environ.get('POSTGRES_PORT','5432')} "
        f"user={os.environ.get('POSTGRES_USER','')} "
        f"password={os.environ.get('POSTGRES_PASSWORD','')} "
        f"dbname={os.environ.get('POSTGRES_DB','gopedia')} sslmode=disable"
    )
    with psycopg.connect(conninfo) as conn:
        enriched = retrieve_and_enrich("Introduction", conn, limit=3)
    if not enriched:
        pytest.skip("no Qdrant hits for query (ingest sample first)")
    first = enriched[0]
    assert "surrounding_context" in first
    assert "matched_content" in first
    assert "breadcrumb" in first
    assert "[문서:" in first["breadcrumb"]
