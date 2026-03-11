#!/usr/bin/env python3
"""
Sync one document to TypeDB (document + sections + composition).
Use after Phloem ingest when TYPEDB_HOST was not set, or for testing.
Usage: python scripts/sync_doc_to_typedb.py <doc_id> <machine_id> <title> <path-to.md>
Env: TYPEDB_HOST, TYPEDB_PORT, TYPEDB_DATABASE (optional)
"""
from __future__ import annotations

import os
import sys
from pathlib import Path

repo_root = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(repo_root))

from core.ontology_so import sync_document_to_typedb


def main() -> int:
    if len(sys.argv) < 5:
        print(
            "Usage: python scripts/sync_doc_to_typedb.py <doc_id> <machine_id> <title> <path-to.md>",
            file=sys.stderr,
        )
        return 1
    doc_id = sys.argv[1]
    try:
        machine_id = int(sys.argv[2])
    except ValueError:
        print("machine_id must be an integer", file=sys.stderr)
        return 1
    title = sys.argv[3]
    md_path = Path(sys.argv[4])
    if not md_path.exists():
        print(f"File not found: {md_path}", file=sys.stderr)
        return 1
    content = md_path.read_text(encoding="utf-8", errors="replace")
    if not os.environ.get("TYPEDB_HOST"):
        print("TYPEDB_HOST not set; skipping TypeDB sync.", file=sys.stderr)
        return 0
    try:
        sync_document_to_typedb(doc_id, machine_id, title, content)
        print(f"Synced doc_id={doc_id} to TypeDB.")
        return 0
    except Exception as e:
        print(f"TypeDB sync failed: {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
