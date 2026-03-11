#!/usr/bin/env python3
"""
Root entrypoint: load Markdown from path and send to Phloem via gRPC.
Usage: python -m property.root_props.run /path/to/file.md
       python -m property.root_props.run /path/to/dir/
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
    load_markdown,
)


def main() -> None:
    if len(sys.argv) < 2:
        print("Usage: python -m property.root_props.run <path-to.md-or-dir>", file=sys.stderr)
        sys.exit(1)

    path = Path(sys.argv[1]).resolve()
    if not path.exists():
        print(f"Path not found: {path}", file=sys.stderr)
        sys.exit(1)

    addr = os.environ.get("GOPEDIA_PHLOEM_GRPC_ADDR", "localhost:50051")
    with grpc.insecure_channel(addr) as channel:
        for md_path in collect_markdown_paths(path):
            content, title, source_metadata = load_markdown(md_path)
            ok, doc_id, machine_id = call_phloem_ingest(
                channel, title, content, source_metadata
            )
            if ok:
                print(f"OK {md_path} -> doc_id={doc_id} machine_id={machine_id}")
            else:
                print(f"FAIL {md_path} doc_id={doc_id} machine_id={machine_id}", file=sys.stderr)
                sys.exit(2)


if __name__ == "__main__":
    main()
