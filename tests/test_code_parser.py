"""Unit tests for flows/code_parser/parser.py — tree-sitter based code parser."""
import json
import subprocess
import sys
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).parent.parent
FIXTURE = REPO_ROOT / "tests/fixtures/sample.py"


# ---------------------------------------------------------------------------
# Helper: run the CLI
# ---------------------------------------------------------------------------

def parse_via_cli(source: str, lang: str = "python") -> dict:
    result = subprocess.run(
        [sys.executable, "-m", "flows.code_parser.cli", "parse", "--lang", lang],
        input=source,
        capture_output=True,
        text=True,
        cwd=str(REPO_ROOT),
    )
    assert result.returncode == 0, f"CLI failed: {result.stderr}"
    return json.loads(result.stdout)


def parse_fixture() -> dict:
    return parse_via_cli(FIXTURE.read_text())


# ---------------------------------------------------------------------------
# TOC tests
# ---------------------------------------------------------------------------

class TestTOC:
    def test_toc_has_function_and_class(self):
        out = parse_fixture()
        names = [n["text"] for n in out["toc"]]
        assert "_pg_connect" in names, f"Expected _pg_connect in TOC, got: {names}"
        assert "DataStore" in names, f"Expected DataStore in TOC, got: {names}"

    def test_toc_entries_have_required_fields(self):
        out = parse_fixture()
        for entry in out["toc"]:
            assert "text" in entry
            assert "level" in entry
            assert "node_type" in entry
            assert "start_line" in entry
            assert "end_line" in entry

    def test_toc_level_is_2(self):
        out = parse_fixture()
        for entry in out["toc"]:
            assert entry["level"] == 2, f"Expected level 2, got {entry['level']}"


# ---------------------------------------------------------------------------
# Lines tests
# ---------------------------------------------------------------------------

class TestLines:
    def test_line_count_matches_source(self):
        source = FIXTURE.read_text()
        expected = len(source.splitlines())
        out = parse_via_cli(source)
        assert len(out["lines"]) == expected, (
            f"Expected {expected} lines, got {len(out['lines'])}"
        )

    def test_function_def_is_anchor(self):
        out = parse_fixture()
        pg_line = next(
            (l for l in out["lines"] if "def _pg_connect" in l["content"]), None
        )
        assert pg_line is not None, "def _pg_connect() line not found"
        assert pg_line["is_anchor"] is True, f"Expected is_anchor=True, got {pg_line}"
        assert pg_line["parent_idx"] == -1, (
            f"Expected parent_idx=-1 for top-level function, got {pg_line['parent_idx']}"
        )

    def test_class_def_is_anchor(self):
        out = parse_fixture()
        class_line = next(
            (l for l in out["lines"] if "class DataStore" in l["content"]), None
        )
        assert class_line is not None, "class DataStore line not found"
        assert class_line["is_anchor"] is True

    def test_empty_lines_preserved(self):
        out = parse_fixture()
        empty_lines = [l for l in out["lines"] if l["content"].strip() == ""]
        assert len(empty_lines) >= 1, "Expected at least one empty line to be preserved"
        for el in empty_lines:
            assert el["node_type"] == "empty_line"

    def test_inner_lines_have_parent_idx(self):
        out = parse_fixture()
        import_line = next(
            (l for l in out["lines"] if "import psycopg" in l["content"]), None
        )
        assert import_line is not None
        assert import_line["parent_idx"] >= 0, (
            f"inner line should have parent_idx >= 0, got {import_line['parent_idx']}"
        )

    def test_line_numbers_are_sequential(self):
        out = parse_fixture()
        nums = [l["line_num"] for l in out["lines"]]
        assert nums == list(range(1, len(nums) + 1)), "line_num should be 1-based sequential"

    def test_source_reconstruction(self):
        """Joining lines by newline must reproduce the original source exactly."""
        source = FIXTURE.read_text()
        out = parse_via_cli(source)
        reconstructed = "\n".join(l["content"] for l in out["lines"])
        assert reconstructed == source.rstrip("\n"), (
            "Source reconstruction failed — parser dropped or mangled lines"
        )


# ---------------------------------------------------------------------------
# CLI smoke tests
# ---------------------------------------------------------------------------

class TestCLI:
    def test_cli_smoke_python(self):
        out = parse_via_cli("def foo():\n    return 1\n")
        assert "toc" in out
        assert "lines" in out
        assert any(l["is_anchor"] for l in out["lines"])

    def test_cli_empty_source(self):
        out = parse_via_cli("")
        assert out["toc"] == []
        assert out["lines"] == []

    def test_cli_go_lang(self):
        go_src = 'package main\n\nfunc Hello() string {\n\treturn "hi"\n}\n'
        out = parse_via_cli(go_src, lang="go")
        assert "toc" in out
        assert "lines" in out
