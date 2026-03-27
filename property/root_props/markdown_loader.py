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


def register_project(
    channel: grpc.Channel,
    root_path: str,
    *,
    name: str = "",
    metadata: dict[str, str] | None = None,
    machine_id: int = 0,
) -> tuple[int | None, int | None]:
    """
    Upsert a projects row by root_path via Phloem.
    Returns (project_id, project_machine_id); (None, None) if the call fails.
    """
    stub = rhizome_pb2_grpc.PhloemStub(channel)
    req = rhizome_pb2.RegisterProjectRequest(
        name=name or "",
        root_path=root_path,
        metadata=metadata or {},
        machine_id=machine_id,
    )
    try:
        resp = stub.RegisterProject(req)
        if resp.project_id:
            return int(resp.project_id), int(resp.machine_id)
    except grpc.RpcError as e:
        print(
            f"RegisterProject failed (ingest continues; use GOPEDIA_PROJECT_ID if needed): {e}",
            file=sys.stderr,
        )
    return None, None


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
    if not resp.ok:
        msg = (resp.error_message or "").strip()
        if msg:
            print(f"Phloem IngestMarkdown: {msg}", file=sys.stderr)
        else:
            print(
                "Phloem IngestMarkdown failed (ok=false, empty error_message)",
                file=sys.stderr,
            )
    return resp.ok, resp.doc_id, resp.machine_id
