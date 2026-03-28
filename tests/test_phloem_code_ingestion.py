"""Verify markdown code fences ingest as L2 c* and split to multiple L3 via codesplitter (go test)."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

import pytest


def _repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


def test_phloem_flow_sample_code_l2_and_multiple_l3() -> None:
    """Runs Go test on phloem-flow.md sample: mermaid block -> c*, SplitToL3 -> 3+ fragments."""
    root = _repo_root()
    cmd = [
        "go",
        "test",
        "-count=1",
        "-run",
        "^TestPhloemFlowSampleHasCodeL2AndMultipleL3$",
        "./internal/phloem/chunker",
    ]
    proc = subprocess.run(
        cmd,
        cwd=root,
        capture_output=True,
        text=True,
        timeout=120,
    )
    if proc.returncode != 0:
        sys.stderr.write(proc.stdout or "")
        sys.stderr.write(proc.stderr or "")
        pytest.fail(
            f"go test failed ({proc.returncode}): {' '.join(cmd)}\n"
            f"stdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
        )


def test_codesplitter_unit() -> None:
    """codesplitter package unit tests (fences + line split)."""
    root = _repo_root()
    cmd = ["go", "test", "-count=1", "./internal/phloem/codesplitter"]
    proc = subprocess.run(
        cmd,
        cwd=root,
        capture_output=True,
        text=True,
        timeout=120,
    )
    if proc.returncode != 0:
        sys.stderr.write(proc.stdout or "")
        sys.stderr.write(proc.stderr or "")
        pytest.fail(
            f"go test failed ({proc.returncode}): {' '.join(cmd)}\n"
            f"stdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
        )
