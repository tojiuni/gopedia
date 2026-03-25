#!/usr/bin/env python3
"""
Print row counts for key Rhizome tables (used by fresh reset scripts).
"""

from __future__ import annotations

import os

import psycopg


def main() -> int:
    c = psycopg.connect(
        f"host={os.environ.get('POSTGRES_HOST','')} "
        f"port={os.environ.get('POSTGRES_PORT','5432')} "
        f"user={os.environ.get('POSTGRES_USER','')} "
        f"password={os.environ.get('POSTGRES_PASSWORD','')} "
        f"dbname={os.environ.get('POSTGRES_DB','gopedia')} sslmode=disable"
    )
    cur = c.cursor()
    for t in ["pipeline_version", "documents", "knowledge_l1", "knowledge_l2", "knowledge_l3", "keyword_so"]:
        cur.execute("SELECT COUNT(*) FROM " + t)
        print(t, cur.fetchone()[0], flush=True)
    cur.close()
    c.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

