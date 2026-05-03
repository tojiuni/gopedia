#!/usr/bin/env python3
"""
Root entrypoint: load Markdown or source code from path and send to Phloem via gRPC.

Markdown files (.md, .markdown) → WikiPipeline (domain=wiki).
Code files (.py, .go, .ts, .tsx, .js, .jsx, .java, .rs, .cpp, .c, .h) → CodePipeline (domain=code).

Usage: python -m property.root_props.run /path/to/file.md
       python -m property.root_props.run /path/to/dir/   (mixed markdown + code)
Env: GOPEDIA_PHLOEM_GRPC_ADDR (default localhost:50051)
"""
from __future__ import annotations

import os
import sys
from pathlib import Path

import grpc

# Ensure repo root on path
repo_root = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(repo_root))

from property.root_props.markdown_loader import (
    call_phloem_ingest,
    collect_markdown_paths,
    finalize_project,
    load_markdown,
    register_project,
)
from property.root_props.project_metadata import build_register_project_metadata
from property.root_props.run_code import _CODE_EXTENSIONS, ingest_code_file
try:
    from core.ontology_so.typedb_sync import (
        sync_directory_tree_to_typedb,
        sync_document_to_typedb,
    )
except ImportError:
    sync_document_to_typedb = None  # TypeDB sync optional if driver missing
    sync_directory_tree_to_typedb = None

try:
    from flows.xylem_flow.tree import fetch_project_l1_nodes
except ImportError:
    fetch_project_l1_nodes = None

# Env: TYPEDB_HOST (optional) — when set, syncs each ingested doc to TypeDB.

_MD_EXTENSIONS = {".md", ".markdown"}


def _collect_all_paths(root: Path) -> list[Path]:
    """Collect markdown and code files from root, skipping hidden dirs."""
    if root.is_file():
        return [root]
    paths = []
    for p in sorted(root.rglob("*")):
        if any(part.startswith(".") for part in p.parts):
            continue
        if p.is_file() and p.suffix.lower() in (_MD_EXTENSIONS | _CODE_EXTENSIONS):
            paths.append(p)
    return paths


def main() -> None:
    if len(sys.argv) < 2:
        print("Usage: python -m property.root_props.run <path-to.md-or-dir>", file=sys.stderr)
        sys.exit(1)

    path = Path(sys.argv[1]).resolve()
    if not path.exists():
        print(f"Path not found: {path}", file=sys.stderr)
        sys.exit(1)

    addr = os.environ.get("GOPEDIA_PHLOEM_GRPC_ADDR", "localhost:50051")
    project_root = path if path.is_dir() else path.parent

    with grpc.insecure_channel(addr) as channel:
        project_id, project_machine_id, already_up_to_date = register_project(
            channel,
            str(project_root),
            name=project_root.name,
            metadata=build_register_project_metadata(),
        )
        project_id_str = str(project_id) if project_id else ""
        project_mid_str = str(project_machine_id) if project_machine_id else ""

        if already_up_to_date:
            print(f"[skip] project already up to date (project_id={project_id})")
            return

        # Single-file: keep existing behaviour (markdown or code)
        if path.is_file():
            all_paths = [path]
        else:
            all_paths = _collect_all_paths(path)

        for file_path in all_paths:
            ext = file_path.suffix.lower()

            if ext in _CODE_EXTENSIONS:
                # ── Code domain ──────────────────────────────────────────
                print(f"[code] {file_path}")
                ok, doc_id, machine_id = ingest_code_file(
                    channel, file_path, project_id_str, project_mid_str
                )
                if ok:
                    print(f"  OK -> doc_id={doc_id}  machine_id={machine_id}")
                else:
                    print(f"  FAIL {file_path}", file=sys.stderr)
                    sys.exit(2)

            elif ext in _MD_EXTENSIONS:
                # ── Markdown domain ───────────────────────────────────────
                content, title, source_metadata = load_markdown(file_path)
                meta = dict(source_metadata)
                if project_id_str:
                    meta["project_id"] = project_id_str
                if project_mid_str:
                    meta["project_machine_id"] = project_mid_str
                print(f"[md]   {file_path}")
                ok, doc_id, machine_id = call_phloem_ingest(
                    channel, title, content, meta
                )
                if ok:
                    print(f"  OK -> doc_id={doc_id}  machine_id={machine_id}")
                    if sync_document_to_typedb is not None and os.environ.get("TYPEDB_HOST"):
                        try:
                            sync_document_to_typedb(doc_id, int(machine_id), title, content)
                        except Exception as e:
                            print(f"  TypeDB sync failed: {e}", file=sys.stderr)
                else:
                    print(f"  FAIL {file_path} doc_id={doc_id} machine_id={machine_id}", file=sys.stderr)
                    sys.exit(2)

        # Store Merkle hash for next-run deduplication.
        if project_id:
            ok = finalize_project(channel, project_id)
            if ok:
                print(f"[finalize] project content_hash stored (project_id={project_id})")

        # Sync directory tree to TypeDB after all files ingested (TYPEDB_HOST gated).
        if (
            sync_directory_tree_to_typedb is not None
            and fetch_project_l1_nodes is not None
            and os.environ.get("TYPEDB_HOST")
            and project_id
        ):
            try:
                import psycopg
                pg_conninfo = (
                    f"host={os.environ.get('POSTGRES_HOST', '')} "
                    f"port={os.environ.get('POSTGRES_PORT', '5432')} "
                    f"user={os.environ.get('POSTGRES_USER', '')} "
                    f"password={os.environ.get('POSTGRES_PASSWORD', '')} "
                    f"dbname={os.environ.get('POSTGRES_DB', 'gopedia')} sslmode=disable"
                )
                with psycopg.connect(pg_conninfo) as pg_conn:
                    l1_rows = fetch_project_l1_nodes(pg_conn, project_id)
                sync_directory_tree_to_typedb(project_id, l1_rows)
                print(f"[typedb] directory tree synced ({len(l1_rows)} nodes, project_id={project_id})")
            except Exception as e:
                print(f"[typedb] directory tree sync failed: {e}", file=sys.stderr)


if __name__ == "__main__":
    main()
