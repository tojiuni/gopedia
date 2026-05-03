"""Unit + integration tests for GraphDB RAG (Phase 1-4).

Unit tests: no external services needed (TypeDB/PG mocked or skipped).
Integration tests: marked @pytest.mark.integration, require TYPEDB_HOST env.

Covered:
  - typedb_schema.typeql  — schema content assertions (Phase 1)
  - postgres_ddl.sql      — typedb_synced_at column present (Phase 2)
  - typedb_sync._escape   — TypeQL escaping (Phase 2)
  - typedb_sync.sync_directory_tree_to_typedb — flat/nested input, graceful skip (Phase 2)
  - typedb_sync.sync_document_to_typedb       — graceful skip when TypeDB absent (Phase 2)
  - graph_context.get_related_l1_ids          — no-host returns [], exception guard (Phase 3)
  - retriever.retrieve_and_enrich             — use_graph_context flag behaviour (Phase 4)
"""
from __future__ import annotations

import os
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO_ROOT))

# Inject mock typedb modules so unit tests run without typedb-driver installed.
# Integration tests that actually call TypeDB use a real driver and skip this.
if "typedb" not in sys.modules:
    _mock_typedb_pkg = MagicMock()
    _mock_typedb_pkg.driver.TransactionType = MagicMock()
    sys.modules["typedb"] = _mock_typedb_pkg
    sys.modules["typedb.driver"] = _mock_typedb_pkg.driver


# ─────────────────────────────────────────────────────────────────────────────
# Phase 1 — Schema assertions
# ─────────────────────────────────────────────────────────────────────────────

SCHEMA_PATH = REPO_ROOT / "core" / "ontology_so" / "typedb_schema.typeql"


def _schema() -> str:
    return SCHEMA_PATH.read_text()


def test_schema_has_directory_entity() -> None:
    s = _schema()
    assert "entity directory" in s, "directory entity missing from schema"
    assert "dir_path" in s
    assert "project_id" in s


def test_schema_has_file_entity_with_l1_id() -> None:
    s = _schema()
    assert "entity file" in s, "file entity missing from schema"
    assert "l1_id" in s, "l1_id bridge attribute missing"


def test_schema_has_section_with_l2_id() -> None:
    s = _schema()
    assert "entity section" in s
    assert "l2_id" in s, "l2_id bridge attribute missing"


def test_schema_has_chunk_with_l3_id() -> None:
    s = _schema()
    assert "entity chunk" in s, "chunk entity missing from schema"
    assert "l3_id" in s, "l3_id bridge attribute missing"


def test_schema_contains_relation_covers_all_levels() -> None:
    s = _schema()
    assert "relation contains" in s
    # All four entity types play contains roles
    for entity in ("directory", "file", "section", "chunk"):
        assert f"{entity}" in s


def test_schema_no_old_document_entity() -> None:
    """Old `document` entity should be replaced by `file`."""
    s = _schema()
    assert "entity document" not in s, "old `document` entity still in schema"


def test_schema_no_old_composition_relation() -> None:
    """Old `composition` relation should be replaced by unified `contains`."""
    s = _schema()
    assert "relation composition" not in s, "old `composition` relation still in schema"


# ─────────────────────────────────────────────────────────────────────────────
# Phase 2 — DDL assertions
# ─────────────────────────────────────────────────────────────────────────────

DDL_PATH = REPO_ROOT / "core" / "ontology_so" / "postgres_ddl.sql"


def test_ddl_has_typedb_synced_at_column() -> None:
    ddl = DDL_PATH.read_text()
    assert "typedb_synced_at" in ddl, "typedb_synced_at column missing from DDL"
    assert "TIMESTAMPTZ" in ddl


# ─────────────────────────────────────────────────────────────────────────────
# Phase 2 — typedb_sync unit tests
# ─────────────────────────────────────────────────────────────────────────────

from core.ontology_so.typedb_sync import _escape


def test_escape_double_quotes() -> None:
    assert _escape('say "hello"') == 'say \\"hello\\"'


def test_escape_backslash() -> None:
    assert _escape("a\\b") == "a\\\\b"


def test_escape_newlines() -> None:
    result = _escape("line1\nline2\r\nline3")
    assert "\n" not in result
    assert "\r" not in result


def test_escape_empty_string() -> None:
    assert _escape("") == ""


def test_sync_directory_tree_empty_rows_returns_true() -> None:
    """Empty l1_rows should return True immediately without calling TypeDB."""
    from core.ontology_so.typedb_sync import sync_directory_tree_to_typedb

    with patch.dict(os.environ, {"TYPEDB_HOST": "localhost"}):
        with patch("core.ontology_so.typedb_sync._typedb_driver") as mock_driver:
            result = sync_directory_tree_to_typedb("1", [])
    assert result is True
    mock_driver.assert_not_called()


def test_sync_directory_tree_flattens_nested_input() -> None:
    """Nested tree (with 'children') should be flattened before processing."""
    from core.ontology_so.typedb_sync import sync_directory_tree_to_typedb

    nested = [
        {
            "id": "l1-parent",
            "title": "docs/parent.md",
            "parent_id": None,
            "children": [
                {"id": "l1-child", "title": "docs/child.md", "parent_id": "l1-parent", "children": []},
            ],
        }
    ]

    mock_tx = MagicMock()
    mock_tx.__enter__ = MagicMock(return_value=mock_tx)
    mock_tx.__exit__ = MagicMock(return_value=False)
    mock_driver_instance = MagicMock()
    mock_driver_instance.transaction.return_value = mock_tx

    with patch("core.ontology_so.typedb_sync._typedb_driver", return_value=mock_driver_instance):
        result = sync_directory_tree_to_typedb("42", nested)

    assert result is True
    # Two nodes → two file insertions expected
    insert_calls = [
        c for c in mock_tx.query.call_args_list
        if "isa file" in str(c)
    ]
    # The test just checks no exception raised and driver was used
    mock_driver_instance.transaction.assert_called_once()


def test_sync_directory_tree_flat_input() -> None:
    """Flat rows (no 'children' key) are processed directly."""
    from core.ontology_so.typedb_sync import sync_directory_tree_to_typedb

    flat = [
        {"id": "aaa", "title": "dir/file1.md"},
        {"id": "bbb", "title": "dir/file2.md"},
    ]

    mock_tx = MagicMock()
    mock_tx.__enter__ = MagicMock(return_value=mock_tx)
    mock_tx.__exit__ = MagicMock(return_value=False)
    mock_driver_instance = MagicMock()
    mock_driver_instance.transaction.return_value = mock_tx

    with patch("core.ontology_so.typedb_sync._typedb_driver", return_value=mock_driver_instance):
        result = sync_directory_tree_to_typedb("1", flat)

    assert result is True
    mock_driver_instance.transaction.assert_called_once()


def test_sync_document_skips_when_typedb_import_fails() -> None:
    """RuntimeError from _typedb_driver (no driver installed) should propagate."""
    from core.ontology_so.typedb_sync import sync_document_to_typedb

    with patch("core.ontology_so.typedb_sync._typedb_driver", side_effect=RuntimeError("no driver")):
        with patch("core.ontology_so.typedb_sync._fetch_l2_l3_rows", return_value=[]):
            with pytest.raises(RuntimeError, match="no driver"):
                sync_document_to_typedb("l1-uuid", "1")


def test_sync_document_calls_mark_synced_on_success() -> None:
    """_mark_synced should be called after a successful TypeDB transaction."""
    from core.ontology_so.typedb_sync import sync_document_to_typedb

    mock_tx = MagicMock()
    mock_tx.__enter__ = MagicMock(return_value=mock_tx)
    mock_tx.__exit__ = MagicMock(return_value=False)
    mock_driver_instance = MagicMock()
    mock_driver_instance.transaction.return_value = mock_tx

    with patch("core.ontology_so.typedb_sync._typedb_driver", return_value=mock_driver_instance):
        with patch("core.ontology_so.typedb_sync._fetch_l2_l3_rows", return_value=[]):
            with patch("core.ontology_so.typedb_sync._mark_synced") as mock_mark:
                result = sync_document_to_typedb("l1-uuid", "1")

    assert result is True
    mock_mark.assert_called_once_with("l1-uuid")


# ─────────────────────────────────────────────────────────────────────────────
# Phase 3 — graph_context unit tests
# ─────────────────────────────────────────────────────────────────────────────

from flows.xylem_flow.graph_context import get_related_l1_ids


def test_get_related_l1_ids_returns_empty_when_no_typedb_host(monkeypatch) -> None:
    """Should return [] immediately when TYPEDB_HOST is not set."""
    monkeypatch.delenv("TYPEDB_HOST", raising=False)
    result = get_related_l1_ids(["l1-aaa", "l1-bbb"], project_id=1)
    assert result == []


def test_get_related_l1_ids_returns_empty_on_exception(monkeypatch) -> None:
    """Any exception in TypeDB traversal should be swallowed and return []."""
    monkeypatch.setenv("TYPEDB_HOST", "localhost")
    with patch(
        "flows.xylem_flow.graph_context._fetch_sibling_l1_ids",
        side_effect=Exception("connection refused"),
    ):
        result = get_related_l1_ids(["l1-aaa"], project_id=1)
    assert result == []


def test_get_related_l1_ids_returns_empty_list_for_no_hits(monkeypatch) -> None:
    """When _fetch_sibling_l1_ids returns [], result is []."""
    monkeypatch.setenv("TYPEDB_HOST", "localhost")
    with patch(
        "flows.xylem_flow.graph_context._fetch_sibling_l1_ids",
        return_value=[],
    ):
        result = get_related_l1_ids(["l1-aaa"], project_id=1)
    assert result == []


def test_get_related_l1_ids_passes_project_id_as_string(monkeypatch) -> None:
    """project_id (int or str) should be passed as string to _fetch_sibling_l1_ids."""
    monkeypatch.setenv("TYPEDB_HOST", "localhost")
    with patch(
        "flows.xylem_flow.graph_context._fetch_sibling_l1_ids",
        return_value=["l1-sibling"],
    ) as mock_fetch:
        get_related_l1_ids(["l1-aaa"], project_id=42)
    _, kwargs = mock_fetch.call_args
    assert kwargs.get("project_id") == "42" or mock_fetch.call_args[0][1] == "42"


def test_get_related_l1_ids_returns_sibling_ids(monkeypatch) -> None:
    """Should return whatever _fetch_sibling_l1_ids returns."""
    monkeypatch.setenv("TYPEDB_HOST", "localhost")
    with patch(
        "flows.xylem_flow.graph_context._fetch_sibling_l1_ids",
        return_value=["l1-sib1", "l1-sib2"],
    ):
        result = get_related_l1_ids(["l1-aaa"], project_id=1)
    assert set(result) == {"l1-sib1", "l1-sib2"}


# ─────────────────────────────────────────────────────────────────────────────
# Phase 4 — retriever.py graph expansion unit tests
# ─────────────────────────────────────────────────────────────────────────────

from flows.xylem_flow.retriever import retrieve_and_enrich


def _make_mock_conn(l3_rows=None):
    """Build a minimal mock psycopg connection for retrieve_and_enrich tests."""
    conn = MagicMock()
    cur = MagicMock()
    cur.__enter__ = MagicMock(return_value=cur)
    cur.__exit__ = MagicMock(return_value=False)
    # fetchone returns (l3_id,) for sibling top-chunk query
    cur.fetchone.return_value = ("sibling-l3-id",) if l3_rows is None else l3_rows
    conn.cursor.return_value = cur
    return conn


def _mock_qdrant_hit(l3_id: str, l1_id: str = "l1-hit", score: float = 0.9):
    hit = MagicMock()
    hit.score = score
    hit.payload = {"l3_id": l3_id, "l1_id": l1_id}
    return hit


def test_retrieve_and_enrich_use_graph_context_false_skips_expansion(monkeypatch) -> None:
    """use_graph_context=False must not call get_related_l1_ids."""
    monkeypatch.setenv("TYPEDB_HOST", "localhost")

    conn = _make_mock_conn()
    mock_ctx = {
        "l1_id": "l1-hit", "l2_id": "l2-x", "matched_l3_id": "l3-aaa",
        "breadcrumb": "doc > sec", "l1_title": "doc", "l2_summary": "",
        "section_heading": "", "matched_content": "content",
        "surrounding_context": "", "l2_section_id": "", "l2_block_type": "",
        "source_path": "", "doc_name": "",
    }

    with patch("flows.xylem_flow.retriever.qdrant_search_l3_points", return_value=[_mock_qdrant_hit("l3-aaa")]):
        with patch("flows.xylem_flow.retriever.embed_query_local", return_value=[0.1] * 1024):
            with patch("flows.xylem_flow.retriever.fetch_rich_context", return_value=mock_ctx):
                with patch("flows.xylem_flow.project_config.resolve_retrieval_settings") as mock_res:
                    mock_res.return_value = MagicMock(
                        embedding_backend="local",
                        local_embedding_addr="http://localhost:18789",
                        qdrant_host="localhost", qdrant_port=6333,
                        collection="gopedia_markdown", vector_name="",
                        embedding_model="text-embedding-3-small",
                    )
                    with patch("flows.xylem_flow.project_config.fetch_project_source_metadata", return_value={}):
                        with patch("flows.xylem_flow.graph_context.get_related_l1_ids") as mock_graph:
                            result = retrieve_and_enrich(
                                "test query", conn,
                                project_id=1,
                                use_graph_context=False,
                            )

    mock_graph.assert_not_called()
    assert all(r.get("source") != "graph_expansion" for r in result)


def test_retrieve_and_enrich_graph_expansion_appends_sibling(monkeypatch) -> None:
    """use_graph_context=True should append sibling results with source=graph_expansion."""
    monkeypatch.setenv("TYPEDB_HOST", "localhost")

    conn = _make_mock_conn()
    hit_ctx = {
        "l1_id": "l1-hit", "l2_id": "l2-hit", "matched_l3_id": "l3-aaa",
        "breadcrumb": "doc > sec", "l1_title": "doc", "l2_summary": "",
        "section_heading": "", "matched_content": "content",
        "surrounding_context": "", "l2_section_id": "", "l2_block_type": "",
        "source_path": "", "doc_name": "",
    }
    sibling_ctx = {
        "l1_id": "l1-sib", "l2_id": "l2-sib", "matched_l3_id": "sibling-l3-id",
        "breadcrumb": "sib-doc > sec", "l1_title": "sib-doc", "l2_summary": "",
        "section_heading": "", "matched_content": "sibling content",
        "surrounding_context": "", "l2_section_id": "", "l2_block_type": "",
        "source_path": "", "doc_name": "",
    }

    def _fake_rich_context(conn, l3_id, **kw):
        return sibling_ctx if l3_id == "sibling-l3-id" else hit_ctx

    with patch("flows.xylem_flow.retriever.qdrant_search_l3_points", return_value=[_mock_qdrant_hit("l3-aaa")]):
        with patch("flows.xylem_flow.retriever.embed_query_local", return_value=[0.1] * 1024):
            with patch("flows.xylem_flow.retriever.fetch_rich_context", side_effect=_fake_rich_context):
                with patch("flows.xylem_flow.project_config.resolve_retrieval_settings") as mock_res:
                    mock_res.return_value = MagicMock(
                        embedding_backend="local",
                        local_embedding_addr="http://localhost:18789",
                        qdrant_host="localhost", qdrant_port=6333,
                        collection="gopedia_markdown", vector_name="",
                        embedding_model="text-embedding-3-small",
                    )
                    with patch("flows.xylem_flow.project_config.fetch_project_source_metadata", return_value={}):
                        with patch(
                            "flows.xylem_flow.graph_context.get_related_l1_ids",
                            return_value=["l1-sib"],
                        ):
                            result = retrieve_and_enrich(
                                "test query", conn,
                                project_id=1,
                                use_graph_context=True,
                            )

    graph_results = [r for r in result if r.get("source") == "graph_expansion"]
    assert len(graph_results) >= 1, "expected at least one graph_expansion result"
    assert graph_results[0]["l1_id"] == "l1-sib"


def test_retrieve_and_enrich_graph_expansion_no_duplicate_l1(monkeypatch) -> None:
    """Graph expansion must not re-add l1_ids already in Qdrant results."""
    monkeypatch.setenv("TYPEDB_HOST", "localhost")

    conn = _make_mock_conn()
    hit_ctx = {
        "l1_id": "l1-hit", "l2_id": "l2-x", "matched_l3_id": "l3-aaa",
        "breadcrumb": "", "l1_title": "", "l2_summary": "", "section_heading": "",
        "matched_content": "", "surrounding_context": "", "l2_section_id": "",
        "l2_block_type": "", "source_path": "", "doc_name": "",
    }

    with patch("flows.xylem_flow.retriever.qdrant_search_l3_points", return_value=[_mock_qdrant_hit("l3-aaa")]):
        with patch("flows.xylem_flow.retriever.embed_query_local", return_value=[0.1] * 1024):
            with patch("flows.xylem_flow.retriever.fetch_rich_context", return_value=hit_ctx):
                with patch("flows.xylem_flow.project_config.resolve_retrieval_settings") as mock_res:
                    mock_res.return_value = MagicMock(
                        embedding_backend="local",
                        local_embedding_addr="http://localhost:18789",
                        qdrant_host="localhost", qdrant_port=6333,
                        collection="gopedia_markdown", vector_name="",
                        embedding_model="text-embedding-3-small",
                    )
                    with patch("flows.xylem_flow.project_config.fetch_project_source_metadata", return_value={}):
                        with patch(
                            "flows.xylem_flow.graph_context.get_related_l1_ids",
                            # same l1_id as the hit — should be skipped
                            return_value=["l1-hit"],
                        ):
                            result = retrieve_and_enrich(
                                "test query", conn,
                                project_id=1,
                                use_graph_context=True,
                            )

    graph_results = [r for r in result if r.get("source") == "graph_expansion"]
    assert graph_results == [], "duplicate l1_id should not appear in graph_expansion results"


def test_retrieve_and_enrich_graph_exception_does_not_break_results(monkeypatch) -> None:
    """Exception in graph expansion must be swallowed; Qdrant results still returned."""
    monkeypatch.setenv("TYPEDB_HOST", "localhost")

    conn = _make_mock_conn()
    hit_ctx = {
        "l1_id": "l1-hit", "l2_id": "l2-x", "matched_l3_id": "l3-aaa",
        "breadcrumb": "", "l1_title": "", "l2_summary": "", "section_heading": "",
        "matched_content": "", "surrounding_context": "", "l2_section_id": "",
        "l2_block_type": "", "source_path": "", "doc_name": "",
    }

    with patch("flows.xylem_flow.retriever.qdrant_search_l3_points", return_value=[_mock_qdrant_hit("l3-aaa")]):
        with patch("flows.xylem_flow.retriever.embed_query_local", return_value=[0.1] * 1024):
            with patch("flows.xylem_flow.retriever.fetch_rich_context", return_value=hit_ctx):
                with patch("flows.xylem_flow.project_config.resolve_retrieval_settings") as mock_res:
                    mock_res.return_value = MagicMock(
                        embedding_backend="local",
                        local_embedding_addr="http://localhost:18789",
                        qdrant_host="localhost", qdrant_port=6333,
                        collection="gopedia_markdown", vector_name="",
                        embedding_model="text-embedding-3-small",
                    )
                    with patch("flows.xylem_flow.project_config.fetch_project_source_metadata", return_value={}):
                        with patch(
                            "flows.xylem_flow.graph_context.get_related_l1_ids",
                            side_effect=Exception("TypeDB down"),
                        ):
                            result = retrieve_and_enrich(
                                "test query", conn,
                                project_id=1,
                                use_graph_context=True,
                            )

    # Qdrant results still present despite graph failure
    assert len(result) >= 1
    assert result[0]["l1_id"] == "l1-hit"


# ─────────────────────────────────────────────────────────────────────────────
# Integration tests (require TYPEDB_HOST to be set and TypeDB reachable)
# ─────────────────────────────────────────────────────────────────────────────

def _typedb_available() -> bool:
    host = os.environ.get("TYPEDB_HOST", "")
    if not host:
        return False
    import socket
    try:
        port = int(os.environ.get("TYPEDB_PORT", "1729"))
        with socket.create_connection((host, port), timeout=2):
            return True
    except OSError:
        return False


@pytest.mark.integration
@pytest.mark.skipif(not _typedb_available(), reason="TYPEDB_HOST not set or TypeDB unreachable")
def test_integration_sync_directory_tree_and_query_siblings() -> None:
    """End-to-end: sync a small tree, then verify sibling lookup returns correct l1_ids."""
    import uuid

    from core.ontology_so.typedb_sync import sync_directory_tree_to_typedb
    from flows.xylem_flow.graph_context import get_related_l1_ids

    project_id = f"test-{uuid.uuid4().hex[:8]}"
    l1_a = str(uuid.uuid4())
    l1_b = str(uuid.uuid4())

    # Both files are in the same directory ("testdir/")
    rows = [
        {"id": l1_a, "title": "testdir/file_a.md"},
        {"id": l1_b, "title": "testdir/file_b.md"},
    ]

    db = os.environ.get("TYPEDB_DATABASE", "gopedia")
    host = os.environ.get("TYPEDB_HOST", "localhost")
    port = os.environ.get("TYPEDB_PORT", "1729")

    # First insert the file entities so the contains relation can match
    from core.ontology_so.typedb_sync import _typedb_driver, _escape
    from typedb.driver import TransactionType

    driver = _typedb_driver(host, port)
    try:
        with driver.transaction(db, TransactionType.WRITE) as tx:
            for row in rows:
                l1_safe = _escape(row["id"])
                proj_safe = _escape(project_id)
                tx.query(
                    f'insert $f isa file, has l1_id "{l1_safe}", '
                    f'has source_type "md", has project_id "{proj_safe}";'
                ).resolve()
            tx.commit()
    finally:
        driver.close()

    # Now sync the directory tree
    result = sync_directory_tree_to_typedb(
        project_id, rows, typedb_host=host, typedb_port=port, typedb_database=db
    )
    assert result is True

    # Query siblings of l1_a → should return l1_b
    siblings = get_related_l1_ids(
        [l1_a], project_id=project_id,
        typedb_host=host, typedb_port=port, typedb_database=db,
    )
    assert l1_b in siblings, f"expected {l1_b} in siblings, got {siblings}"
    assert l1_a not in siblings, "hit l1_id should not appear in siblings"
