#!/usr/bin/env python3
"""Index reset — truncate PostgreSQL tables and Qdrant points for gopedia.

Delete order respects FK constraints:
  keyword_so → knowledge_l3 → knowledge_l2 → knowledge_l1 → documents → projects

Supports:
  --dry-run   Print row counts, do nothing
  --project-id <int>   Limit deletion to one project (partial reset)
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

repo_root = Path(__file__).resolve().parents[2]
if str(repo_root) not in sys.path:
    sys.path.insert(0, str(repo_root))

_DELETE_ORDER = [
    "keyword_so",
    "knowledge_l3",
    "knowledge_l2",
    "knowledge_l1",
    "documents",
    "projects",
]


def _pg_connect():
    import psycopg
    return psycopg.connect(
        f"host={os.environ.get('POSTGRES_HOST', '')} "
        f"port={os.environ.get('POSTGRES_PORT', '5432')} "
        f"user={os.environ.get('POSTGRES_USER', '')} "
        f"password={os.environ.get('POSTGRES_PASSWORD', '')} "
        f"dbname={os.environ.get('POSTGRES_DB', 'gopedia')} "
        f"sslmode={os.environ.get('POSTGRES_SSLMODE', 'disable')}"
    )


def _count_rows(conn, table: str, project_id: int | None) -> int:
    if project_id is None:
        row = conn.execute(f"SELECT COUNT(*) FROM {table}").fetchone()
    elif table == "keyword_so":
        return 0  # no project FK — skipped for partial reset
    elif table == "projects":
        row = conn.execute("SELECT COUNT(*) FROM projects WHERE id = %s", (project_id,)).fetchone()
    elif table == "documents":
        row = conn.execute("SELECT COUNT(*) FROM documents WHERE project_id = %s", (project_id,)).fetchone()
    elif table == "knowledge_l1":
        row = conn.execute("SELECT COUNT(*) FROM knowledge_l1 WHERE project_id = %s", (project_id,)).fetchone()
    elif table == "knowledge_l2":
        row = conn.execute(
            "SELECT COUNT(*) FROM knowledge_l2 WHERE l1_id IN "
            "(SELECT id FROM knowledge_l1 WHERE project_id = %s)",
            (project_id,),
        ).fetchone()
    elif table == "knowledge_l3":
        row = conn.execute(
            "SELECT COUNT(*) FROM knowledge_l3 WHERE l2_id IN "
            "(SELECT id FROM knowledge_l2 WHERE l1_id IN "
            "(SELECT id FROM knowledge_l1 WHERE project_id = %s))",
            (project_id,),
        ).fetchone()
    else:
        row = conn.execute(f"SELECT COUNT(*) FROM {table}").fetchone()
    return int(row[0]) if row else 0


def reset_postgres(project_id: int | None, dry_run: bool) -> dict:
    result = {}
    with _pg_connect() as conn:
        for table in _DELETE_ORDER:
            count = _count_rows(conn, table, project_id)
            result[table] = {"rows": count, "deleted": 0}
            if dry_run or count == 0:
                continue
            if project_id is None:
                conn.execute(f"TRUNCATE {table} CASCADE")
            elif table == "keyword_so":
                pass  # no project FK — skip for partial reset
            elif table == "projects":
                conn.execute("DELETE FROM projects WHERE id = %s", (project_id,))
            elif table == "documents":
                conn.execute("DELETE FROM documents WHERE project_id = %s", (project_id,))
            elif table == "knowledge_l1":
                conn.execute("DELETE FROM knowledge_l1 WHERE project_id = %s", (project_id,))
            elif table == "knowledge_l2":
                conn.execute(
                    "DELETE FROM knowledge_l2 WHERE l1_id IN "
                    "(SELECT id FROM knowledge_l1 WHERE project_id = %s)",
                    (project_id,),
                )
            elif table == "knowledge_l3":
                conn.execute(
                    "DELETE FROM knowledge_l3 WHERE l2_id IN "
                    "(SELECT id FROM knowledge_l2 WHERE l1_id IN "
                    "(SELECT id FROM knowledge_l1 WHERE project_id = %s))",
                    (project_id,),
                )
            result[table]["deleted"] = count
        if not dry_run:
            conn.commit()
    return result


def reset_qdrant(project_id: int | None, dry_run: bool) -> dict:
    from qdrant_client import QdrantClient
    from qdrant_client.models import Filter, FieldCondition, MatchValue

    host = os.environ.get("QDRANT_HOST", "localhost")
    port = int(os.environ.get("QDRANT_PORT", "6333"))
    collection = os.environ.get("QDRANT_COLLECTION", "gopedia_markdown")

    client = QdrantClient(host=host, port=port)
    try:
        info = client.get_collection(collection)
        total = info.points_count
    except Exception:
        return {"collection": collection, "points": 0, "deleted": 0, "error": "collection not found"}

    if dry_run:
        return {"collection": collection, "points": total, "deleted": 0, "dry_run": True}

    if project_id is None:
        client.delete_collection(collection)
        client.recreate_collection(
            collection,
            vectors_config={"size": 1024, "distance": "Cosine"},
        )
        return {"collection": collection, "points": total, "deleted": total}
    else:
        result = client.delete(
            collection_name=collection,
            points_selector=Filter(
                must=[FieldCondition(key="project_id", match=MatchValue(value=project_id))]
            ),
        )
        return {"collection": collection, "deleted": getattr(result, "status", "ok")}


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description="Reset gopedia index (PostgreSQL + Qdrant)")
    parser.add_argument("--project-id", type=int, default=None, help="Limit to one project (partial reset)")
    parser.add_argument("--dry-run", action="store_true", help="Count rows without deleting")
    parser.add_argument("--skip-qdrant", action="store_true", help="Skip Qdrant reset")
    args = parser.parse_args(argv)

    output: dict = {"dry_run": args.dry_run, "project_id": args.project_id}

    try:
        output["postgres"] = reset_postgres(args.project_id, args.dry_run)
    except Exception as e:
        output["postgres"] = {"error": str(e)}

    if not args.skip_qdrant:
        try:
            output["qdrant"] = reset_qdrant(args.project_id, args.dry_run)
        except Exception as e:
            output["qdrant"] = {"error": str(e)}

    print(json.dumps(output, ensure_ascii=False, default=str))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
