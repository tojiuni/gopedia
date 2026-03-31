#!/usr/bin/env python3
"""
Code file ingest entrypoint: read source code from path and send to Phloem
via gRPC with domain=code so the CodePipeline is selected.

Usage:
    python -m property.root_props.run_code /path/to/file.py
    python -m property.root_props.run_code /path/to/file.go

Env:
    GOPEDIA_PHLOEM_GRPC_ADDR  (default localhost:50051)
    GOPEDIA_PROJECT_ID        (optional)
"""
from __future__ import annotations

import os
import sys
from pathlib import Path

import grpc

repo_root = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(repo_root))

from core.proto.gen.python import rhizome_pb2, rhizome_pb2_grpc
from property.root_props.markdown_loader import register_project
from property.root_props.project_metadata import build_register_project_metadata

_CODE_EXTENSIONS = {".py", ".go", ".ts", ".tsx", ".js", ".jsx", ".java", ".rs", ".cpp", ".c", ".h"}

_LANG_MAP = {
    ".py": "python",
    ".go": "go",
    ".ts": "typescript",
    ".tsx": "typescript",
    ".js": "typescript",
    ".jsx": "typescript",
}


def _detect_lang(path: Path) -> str:
    return _LANG_MAP.get(path.suffix.lower(), "python")


def collect_code_paths(root: Path) -> list[Path]:
    if root.is_file():
        if root.suffix.lower() in _CODE_EXTENSIONS:
            return [root]
        return []
    paths = []
    for p in sorted(root.rglob("*")):
        if p.is_file() and p.suffix.lower() in _CODE_EXTENSIONS:
            paths.append(p)
    return paths


def ingest_code_file(
    channel: grpc.Channel,
    file_path: Path,
    project_id_str: str = "",
    project_machine_id_str: str = "",
) -> tuple[bool, str, int]:
    content = file_path.read_text(encoding="utf-8", errors="replace")
    title = str(file_path)
    lang = _detect_lang(file_path)

    meta: dict[str, str] = {
        "domain":    "code",
        "language":  lang,
        "source_type": "code",
    }
    if project_id_str:
        meta["project_id"] = project_id_str
    if project_machine_id_str:
        meta["project_machine_id"] = project_machine_id_str
    if env_pid := os.environ.get("GOPEDIA_PROJECT_ID", ""):
        meta.setdefault("project_id", env_pid)

    stub = rhizome_pb2_grpc.PhloemStub(channel)
    req = rhizome_pb2.IngestRequest(
        title=title,
        content=content,
        source_metadata=meta,
    )
    resp = stub.IngestMarkdown(req)
    if not resp.ok:
        msg = (resp.error_message or "").strip() or "unknown error"
        print(f"  Phloem error: {msg}", file=sys.stderr)
    return resp.ok, resp.doc_id, resp.machine_id


def main() -> None:
    if len(sys.argv) < 2:
        print(
            "Usage: python -m property.root_props.run_code <file.py|file.go|dir>",
            file=sys.stderr,
        )
        sys.exit(1)

    path = Path(sys.argv[1]).resolve()
    if not path.exists():
        print(f"Path not found: {path}", file=sys.stderr)
        sys.exit(1)

    addr = os.environ.get("GOPEDIA_PHLOEM_GRPC_ADDR", "localhost:50051")
    project_root = path if path.is_dir() else path.parent

    with grpc.insecure_channel(addr) as channel:
        project_id, project_machine_id = register_project(
            channel,
            str(project_root),
            name=project_root.name,
            metadata=build_register_project_metadata(),
        )
        project_id_str = str(project_id) if project_id else ""
        project_mid_str = str(project_machine_id) if project_machine_id else ""

        code_paths = collect_code_paths(path)
        if not code_paths:
            print(f"No code files found under: {path}", file=sys.stderr)
            sys.exit(1)

        for code_path in code_paths:
            lang = _detect_lang(code_path)
            print(f"Ingesting [{lang}] {code_path} ...")
            ok, doc_id, machine_id = ingest_code_file(
                channel, code_path, project_id_str, project_mid_str
            )
            if ok:
                print(f"  OK -> doc_id={doc_id}  machine_id={machine_id}")
            else:
                print(f"  FAIL {code_path}", file=sys.stderr)
                sys.exit(2)


if __name__ == "__main__":
    main()
