#!/usr/bin/env python3
"""
After root ingest (run.py): assert projects table has rows and documents.project_id FK resolves.

Used by run_fresh_ingestion_docker.sh after sample ingest. Exits 1 if Postgres is empty or FK broken.
"""

from __future__ import annotations

import os
import sys

import psycopg


def main() -> int:
    host = os.environ.get("POSTGRES_HOST", "")
    user = os.environ.get("POSTGRES_USER", "")
    if not host or not user:
        print("POSTGRES_HOST/POSTGRES_USER required", file=sys.stderr)
        return 1

    conn = psycopg.connect(
        f"host={host} "
        f"port={os.environ.get('POSTGRES_PORT', '5432')} "
        f"user={user} "
        f"password={os.environ.get('POSTGRES_PASSWORD', '')} "
        f"dbname={os.environ.get('POSTGRES_DB', 'gopedia')} sslmode=disable"
    )
    try:
        cur = conn.cursor()
        cur.execute("SELECT COUNT(*) FROM projects")
        n_proj = cur.fetchone()[0]
        if n_proj < 1:
            print("verify_projects: expected >= 1 row in projects, got", n_proj, file=sys.stderr)
            return 1

        cur.execute(
            """
            SELECT d.id::text, d.project_id, p.id, p.root_path, p.machine_id
            FROM documents d
            JOIN projects p ON p.id = d.project_id
            ORDER BY d.created_at DESC NULLS LAST
            LIMIT 10
            """
        )
        linked = cur.fetchall()
        if not linked:
            print(
                "verify_projects: no documents with project_id pointing at projects (ingest with run.py?)",
                file=sys.stderr,
            )
            return 1

        for doc_id, pid, proj_id, root, pmid in linked:
            if pid != proj_id:
                print(
                    f"verify_projects: mismatch doc={doc_id} project_id={pid} projects.id={proj_id}",
                    file=sys.stderr,
                )
                return 1
            if pmid is None or pmid == 0:
                print(
                    f"verify_projects: project row missing machine_id (root={root})",
                    file=sys.stderr,
                )
                return 1

        print(f"verify_projects: ok ({n_proj} project(s), {len(linked)} recent doc(s) linked, machine_id set)")
        cur.close()
    finally:
        conn.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
