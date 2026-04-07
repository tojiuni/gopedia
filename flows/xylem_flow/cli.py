#!/usr/bin/env python3
"""CLI for Xylem semantic search and optional L1 restore.

Run from repo root with PYTHONPATH set (the Go runner sets this):

    python -m flows.xylem_flow.cli search --query "Introduction"
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Any

repo_root = Path(__file__).resolve().parents[2]
if str(repo_root) not in sys.path:
    sys.path.insert(0, str(repo_root))


def _pg_connect() -> Any:
    import psycopg

    return psycopg.connect(
        f"host={os.environ.get('POSTGRES_HOST', '')} "
        f"port={os.environ.get('POSTGRES_PORT', '5432')} "
        f"user={os.environ.get('POSTGRES_USER', '')} "
        f"password={os.environ.get('POSTGRES_PASSWORD', '')} "
        f"dbname={os.environ.get('POSTGRES_DB', 'gopedia')} "
        f"sslmode={os.environ.get('POSTGRES_SSLMODE', 'disable')}"
    )


def _hit_to_markdown(hit: dict[str, Any], index: int) -> str:
    parts: list[str] = []
    title = hit.get("breadcrumb") or f"Hit {index + 1}"
    parts.append(f"## {title}\n")
    for key in (
        "qdrant_score",
        "l1_title",
        "section_heading",
        "l2_summary",
        "matched_l3_id",
        "matched_content",
    ):
        if key in hit and hit[key] is not None:
            parts.append(f"**{key}**: {hit[key]}\n")
    sc = hit.get("surrounding_context") or ""
    if sc.strip():
        parts.append("\n### Surrounding context\n\n")
        parts.append(str(sc).strip())
        parts.append("\n")
    return "".join(parts).strip()


def cmd_search(args: argparse.Namespace) -> int:
    from flows.xylem_flow.restorer import restore_markdown_for_l1
    from flows.xylem_flow.retriever import retrieve_and_enrich

    query = (args.query or "").strip()
    if not query:
        print("empty query", file=sys.stderr)
        return 2

    embedding_backend = os.environ.get("GOPEDIA_EMBEDDING_BACKEND", "openai").strip().lower()
    if embedding_backend != "local" and not os.environ.get("OPENAI_API_KEY"):
        print("OPENAI_API_KEY required for semantic search when backend is not local", file=sys.stderr)
        return 2

    try:
        with _pg_connect() as conn:
            kw: dict = dict(
                final_limit=args.limit,
                neighbor_window=args.neighbor_window,
                context_level=args.context_level,
                use_reranker=getattr(args, "reranker", False),
                reranker_model=getattr(args, "reranker_model", None),
            )
            if getattr(args, "project_id", None) is not None:
                kw["project_id"] = int(args.project_id)
            enriched = retrieve_and_enrich(query, conn, **kw)
    except Exception as e:
        print(f"search failed: {e}", file=sys.stderr)
        return 2

    if not enriched:
        print("No Qdrant hits or empty context (check ingest / QDRANT_COLLECTION).", file=sys.stderr)
        return 3

    if args.restore_l1:
        l1_id = enriched[0].get("l1_id")
        if not l1_id:
            print("top hit missing l1_id", file=sys.stderr)
            return 4
        try:
            with _pg_connect() as conn:
                md = restore_markdown_for_l1(conn, str(l1_id))
        except Exception as e:
            print(f"restore failed: {e}", file=sys.stderr)
            return 2
        if args.format == "json":
            payload = {
                "l1_id": l1_id,
                "markdown": md,
                "hits": enriched,
            }
            print(json.dumps(payload, ensure_ascii=False, default=str))
        else:
            print("# Restored L1 (top hit)\n")
            print(md.strip())
        return 0

    if args.format == "json":
        print(json.dumps(enriched, ensure_ascii=False, default=str))
        return 0

    blocks = [_hit_to_markdown(h, i) for i, h in enumerate(enriched)]
    print("\n\n---\n\n".join(blocks))
    return 0


def cmd_restore(args: argparse.Namespace) -> int:
    from flows.xylem_flow.restorer import restore_code_for_l2, restore_content_for_l1

    l1_id = (args.l1_id or "").strip()
    l2_id = (args.l2_id or "").strip()
    if (not l1_id and not l2_id) or (l1_id and l2_id):
        print("exactly one of --l1-id or --l2-id is required", file=sys.stderr)
        return 2

    try:
        with _pg_connect() as conn:
            if l1_id:
                restored = restore_content_for_l1(conn, l1_id)
                content = restored.get("content") or ""
            else:
                content = restore_code_for_l2(conn, l2_id)
                restored = {
                    "l2_id": l2_id,
                    "content": content,
                    "source_type": "code",
                }
    except Exception as e:
        print(f"restore failed: {e}", file=sys.stderr)
        return 2

    if args.format == "json":
        print(json.dumps(restored, ensure_ascii=False, default=str))
        return 0

    print(str(content).strip())
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Gopedia Xylem CLI")
    sub = parser.add_subparsers(dest="command", required=True)

    p_search = sub.add_parser("search", help="Semantic search + rich PG context")
    p_search.add_argument("--query", "-q", required=True, help="Search query text")
    p_search.add_argument(
        "--format",
        choices=("markdown", "json"),
        default="markdown",
        help="Output format (default: markdown)",
    )
    p_search.add_argument(
        "--limit",
        type=int,
        default=5,
        help="Max hits after retrieval (default: 5)",
    )
    p_search.add_argument(
        "--neighbor-window",
        type=int,
        default=2000,
        help="Neighbor sort_order span (default: 2000)",
    )
    p_search.add_argument(
        "--context-level",
        type=int,
        default=1,
        help="fetch_rich_context level (default: 1)",
    )
    p_search.add_argument(
        "--restore-l1",
        action="store_true",
        help="Also restore full markdown for L1 of the top hit",
    )
    p_search.add_argument(
        "--project-id",
        type=int,
        default=None,
        help="Load Qdrant/PG settings from projects.source_metadata and filter Qdrant by project_id",
    )
    p_search.add_argument(
        "--reranker",
        action="store_true",
        help="Enable cross-encoder reranking between Qdrant candidates and final cutoff",
    )
    p_search.add_argument(
        "--reranker-model",
        default=None,
        dest="reranker_model",
        help="Cross-encoder model name (default: BAAI/bge-reranker-v2-m3 or GOPEDIA_RERANKER_MODEL env)",
    )
    p_search.set_defaults(func=cmd_search)

    p_restore = sub.add_parser("restore", help="Restore content from PostgreSQL by l1_id or l2_id")
    p_restore.add_argument("--l1-id", default="", help="knowledge_l1.id UUID to restore full content")
    p_restore.add_argument("--l2-id", default="", help="knowledge_l2.id UUID to restore code section")
    p_restore.add_argument(
        "--format",
        choices=("markdown", "json"),
        default="markdown",
        help="Output format (default: markdown)",
    )
    p_restore.set_defaults(func=cmd_restore)

    args = parser.parse_args(argv)
    fn = getattr(args, "func", None)
    if fn is None:
        parser.print_help()
        return 1
    return int(fn(args))


if __name__ == "__main__":
    raise SystemExit(main())
