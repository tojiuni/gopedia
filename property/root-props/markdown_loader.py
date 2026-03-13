"""
Markdown loader and Phloem gRPC client for Gopedia 0.0.1.
Loads .md files, parses front-matter, and calls Phloem IngestMarkdown.
"""
from __future__ import annotations

import os
import sys
from pathlib import Path

# Repo root for proto gen
repo_root = Path(__file__).resolve().parents[2]
if str(repo_root) not in sys.path:
    sys.path.insert(0, str(repo_root))

from core.proto.gen.python import rhizome_pb2, rhizome_pb2_grpc
import grpc


def load_markdown(path: str | Path) -> tuple[str, str, dict[str, str]]:
    """
    Load a markdown file and parse front-matter.
    Returns (content, title, source_metadata).
    """
    path = Path(path)
    raw = path.read_text(encoding="utf-8", errors="replace")

    try:
        import frontmatter
    except ImportError:
        title = path.stem.replace("-", " ").replace("_", " ").title()
        return raw, title, {}
    if not hasattr(frontmatter, "loads"):
        title = path.stem.replace("-", " ").replace("_", " ").title()
        return raw, title, {}
    post = frontmatter.loads(raw)
    meta = dict(post.metadata) if post.metadata else {}
    title = meta.pop("title", path.stem.replace("-", " ").replace("_", " ").title())
    # Normalize metadata to string values for proto map<string,string>
    source_metadata = {}
    for k, v in meta.items():
        if v is not None:
            source_metadata[str(k)] = str(v)
    return post.content, title, source_metadata


def collect_markdown_paths(path: str | Path) -> list[Path]:
    """Collect all .md files under path (file or directory)."""
    path = Path(path)
    if path.is_file():
        return [path] if path.suffix.lower() == ".md" else []
    return list(path.rglob("*.md"))


def call_phloem_ingest(
    channel: grpc.Channel,
    title: str,
    content: str,
    source_metadata: dict[str, str] | None = None,
) -> tuple[bool, str, int]:
    """
    Call Phloem IngestMarkdown. Returns (ok, doc_id, machine_id).
    """
    stub = rhizome_pb2_grpc.PhloemStub(channel)
    req = rhizome_pb2.IngestRequest(
        title=title,
        content=content,
        source_metadata=source_metadata or {},
    )
    resp = stub.IngestMarkdown(req)
    return resp.ok, resp.doc_id, resp.machine_id
