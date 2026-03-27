#!/usr/bin/env python3
"""
Xylem flow verification after Phloem ingest (docker E2E).

Modes:
  --restore-only   PostgreSQL에서 최신 L1 기준 전체 마크다운 복원만 (OpenAI/Qdrant 불필요)
  --keyword-only   시맨틱 검색 + PG 리치 컨텍스트만 (OPENAI_API_KEY 필요)
  (default)        스키마 확인 + 복원 미리보기 + keyword 검증

Test environment: docker (use .env with service hostnames).

Exit codes: 0 = success, 2 = OpenAI/embed or retrieve error, 3 = no enriched Qdrant hits,
            4 = Postgres/schema/assertion failure.
"""
from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

repo_root = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(repo_root))


def _pg_connect():
    import psycopg

    return psycopg.connect(
        f"host={os.environ.get('POSTGRES_HOST','')} "
        f"port={os.environ.get('POSTGRES_PORT','5432')} "
        f"user={os.environ.get('POSTGRES_USER','')} "
        f"password={os.environ.get('POSTGRES_PASSWORD','')} "
        f"dbname={os.environ.get('POSTGRES_DB','gopedia')} sslmode=disable"
    )


def _check_title_id_column(cur) -> bool:
    cur.execute(
        """
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'knowledge_l2'
          AND column_name = 'title_id'
        """
    )
    return cur.fetchone() is not None


def run_restore_only() -> int:
    try:
        from flows.xylem_flow.restorer import restore_markdown_for_l1
    except ImportError as e:
        print(f"Import error: {e}", file=sys.stderr)
        return 4

    try:
        with _pg_connect() as conn:
            with conn.cursor() as cur:
                if not _check_title_id_column(cur):
                    print(
                        "knowledge_l2.title_id column missing (apply postgres_ddl.sql)",
                        file=sys.stderr,
                    )
                    return 4
                cur.execute("SELECT COUNT(*) FROM knowledge_l2 WHERE title_id IS NOT NULL")
                titled = cur.fetchone()[0]
                cur.execute(
                    "SELECT id::text FROM knowledge_l1 ORDER BY created_at DESC NULLS LAST LIMIT 1"
                )
                row = cur.fetchone()
                if not row:
                    print("No knowledge_l1 rows; run ingest first.", file=sys.stderr)
                    return 4
                l1_id = row[0]

            md = restore_markdown_for_l1(conn, l1_id)
            if not md.strip():
                print("restore_markdown_for_l1 returned empty", file=sys.stderr)
                return 4
            if "#" not in md:
                print(
                    "restored markdown has no heading lines (unexpected)",
                    file=sys.stderr,
                )
                return 4
            print("--- Full markdown restore ---")
            print(md)
            print(f"--- title_id count on knowledge_l2: {titled} ---")
            print(f"--- l1_id: {l1_id} ---")
    except Exception as e:
        print(f"Postgres/verify error: {e}", file=sys.stderr)
        return 4

    print("Xylem restore-only OK.")
    return 0


def run_keyword_only(query: str) -> int:
    try:
        from flows.xylem_flow.retriever import retrieve_and_enrich
    except ImportError as e:
        print(f"Import error: {e}", file=sys.stderr)
        return 4

    if not os.environ.get("OPENAI_API_KEY"):
        print("OPENAI_API_KEY required for --keyword-only", file=sys.stderr)
        return 2

    try:
        with _pg_connect() as conn:
            with conn.cursor() as cur:
                if not _check_title_id_column(cur):
                    print(
                        "knowledge_l2.title_id column missing (apply postgres_ddl.sql)",
                        file=sys.stderr,
                    )
                    return 4
                cur.execute(
                    "SELECT id::text FROM knowledge_l1 ORDER BY created_at DESC NULLS LAST LIMIT 1"
                )
                if not cur.fetchone():
                    print("No knowledge_l1 rows; run ingest first.", file=sys.stderr)
                    return 4

            try:
                enriched = retrieve_and_enrich(query, conn, limit=5)
            except Exception as e:
                print(f"retrieve_and_enrich failed: {e}", file=sys.stderr)
                return 2

            if not enriched:
                print(
                    "No enriched Qdrant hits (check QDRANT_COLLECTION / QDRANT_VECTOR_NAME / ingest).",
                    file=sys.stderr,
                )
                return 3

            print(f"--- Keyword / semantic search query={query!r} (top {len(enriched)} hits) ---")
            for idx, top in enumerate(enriched):
                print(f"--- hit {idx} ---")
                for k in (
                    "qdrant_score",
                    "l1_title",
                    "section_heading",
                    "l2_summary",
                    "matched_l3_id",
                    "matched_content",
                ):
                    v = top.get(k)
                    s = str(v) if v is not None else ""
                    tail = "..." if len(s) > 200 else ""
                    print(f"  {k}: {s[:200]}{tail}")
                sc = top.get("surrounding_context") or ""
                print(f"  surrounding_context (len={len(sc)}):")
                print(f"  {sc[:800]}{'...' if len(sc) > 800 else ''}")

    except Exception as e:
        print(f"Postgres/verify error: {e}", file=sys.stderr)
        return 4

    print("Xylem keyword-only OK.")
    return 0


def run_all(query: str) -> int:
    try:
        from flows.xylem_flow.restorer import restore_markdown_for_l1
        from flows.xylem_flow.retriever import retrieve_and_enrich
    except ImportError as e:
        print(f"Import error: {e}", file=sys.stderr)
        return 4

    try:
        with _pg_connect() as conn:
            with conn.cursor() as cur:
                if not _check_title_id_column(cur):
                    print(
                        "knowledge_l2.title_id column missing (apply postgres_ddl.sql)",
                        file=sys.stderr,
                    )
                    return 4

                cur.execute("SELECT COUNT(*) FROM knowledge_l2 WHERE title_id IS NOT NULL")
                titled = cur.fetchone()[0]
                cur.execute(
                    "SELECT id::text FROM knowledge_l1 ORDER BY created_at DESC NULLS LAST LIMIT 1"
                )
                row = cur.fetchone()
                if not row:
                    print("No knowledge_l1 rows; run ingest first.", file=sys.stderr)
                    return 4
                l1_id = row[0]

            md = restore_markdown_for_l1(conn, l1_id)
            if not md.strip():
                print("restore_markdown_for_l1 returned empty", file=sys.stderr)
                return 4
            if "#" not in md:
                print(
                    "restored markdown has no heading lines (unexpected)",
                    file=sys.stderr,
                )
                return 4
            print("--- Markdown restore (first 800 chars) ---")
            print(md[:800] + ("..." if len(md) > 800 else ""))
            print(f"--- title_id count on knowledge_l2: {titled} ---")

            if not os.environ.get("OPENAI_API_KEY"):
                print("OPENAI_API_KEY not set; skipping Qdrant enrich step.")
                return 0

            try:
                enriched = retrieve_and_enrich(query, conn, limit=5)
            except Exception as e:
                print(f"retrieve_and_enrich failed: {e}", file=sys.stderr)
                return 2

            if not enriched:
                print(
                    "No enriched Qdrant hits (check QDRANT_COLLECTION / ingest).",
                    file=sys.stderr,
                )
                return 3

            print("--- Top enriched context ---")
            top = enriched[0]
            for k in (
                "l1_title",
                "section_heading",
                "l2_summary",
                "matched_content",
                "qdrant_score",
            ):
                v = top.get(k)
                print(
                    f"  {k}: {str(v)[:200]}{'...' if v is not None and len(str(v)) > 200 else ''}"
                )
            sc = top.get("surrounding_context") or ""
            if len(sc) < 10:
                print("surrounding_context too short", file=sys.stderr)
                return 4
            print(
                f"  surrounding_context (len={len(sc)}): preview ... {sc[:300]}..."
            )

    except Exception as e:
        print(f"Postgres/verify error: {e}", file=sys.stderr)
        return 4

    print("Xylem flow verify OK.")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Xylem flow verification")
    parser.add_argument(
        "query",
        nargs="?",
        default=os.environ.get("XYLEM_VERIFY_QUERY", "Introduction"),
        help="Semantic search query (keyword / full modes)",
    )
    g = parser.add_mutually_exclusive_group()
    g.add_argument(
        "--restore-only",
        action="store_true",
        help="Only restore full markdown from PG (no OpenAI/Qdrant)",
    )
    g.add_argument(
        "--keyword-only",
        action="store_true",
        help="Only run embedding search + enriched context (no full-doc restore preview)",
    )
    args = parser.parse_args()

    if args.restore_only:
        return run_restore_only()
    if args.keyword_only:
        return run_keyword_only(args.query)
    return run_all(args.query)


if __name__ == "__main__":
    raise SystemExit(main())
