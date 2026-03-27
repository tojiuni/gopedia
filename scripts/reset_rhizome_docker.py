#!/usr/bin/env python3
"""
Reset/clean Rhizome stores (PostgreSQL, Qdrant, TypeDB) from inside Docker.

Intended use:
  - run inside a container on the same Docker network as postgres_db/typedb/qdrant
  - uses environment variables from .env (or overrides passed by docker -e)

What it does:
  - PostgreSQL: DROP knowledge_l1/2/3, keyword_so, documents, projects, pipeline_version (if exists)
  - Qdrant: delete QDRANT_COLLECTION and QDRANT_DOC_COLLECTION (if exists)
  - TypeDB: delete TYPEDB_DATABASE (if exists)

It prints "BEFORE" and "AFTER" existence checks so you can confirm deletion worked.
"""

from __future__ import annotations

import os
import sys


def _env(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


def reset_postgres() -> None:
    import psycopg

    host = _env("POSTGRES_HOST", "postgres_db")
    port = _env("POSTGRES_PORT", "5432")
    user = _env("POSTGRES_USER", "")
    password = _env("POSTGRES_PASSWORD", "")
    db = _env("POSTGRES_DB", "gopedia")

    if not user:
        print("Postgres reset skipped: POSTGRES_USER not set", file=sys.stderr)
        return

    conninfo = (
        f"host={host} port={port} user={user} password={password} "
        f"dbname={db} sslmode=disable"
    )
    print(f"[postgres] connect: {host}:{port} db={db}")

    tables = [
        "keyword_so",
        "knowledge_l3",
        "knowledge_l2",
        "knowledge_l1",
        "documents",
        "projects",
        "pipeline_version",
    ]

    with psycopg.connect(conninfo) as conn:
        cur = conn.cursor()
        print("[postgres] BEFORE existence:")
        for t in tables:
            cur.execute("SELECT to_regclass(%s)", (t,))
            print(f"  {t}: {cur.fetchone()[0]}")

        for t in tables:
            cur.execute(f"DROP TABLE IF EXISTS {t} CASCADE")
        conn.commit()

        print("[postgres] AFTER existence:")
        for t in tables:
            cur.execute("SELECT to_regclass(%s)", (t,))
            print(f"  {t}: {cur.fetchone()[0]}")

        cur.close()


def reset_qdrant() -> None:
    from qdrant_client import QdrantClient

    host = _env("QDRANT_HOST", "qdrant")
    port = int(_env("QDRANT_PORT", "6333"))
    collections = [
        _env("QDRANT_COLLECTION", "gopedia_markdown"),
        _env("QDRANT_DOC_COLLECTION", "gopedia_document"),
    ]

    qc = QdrantClient(host=host, port=port)
    print(f"[qdrant] host={host}:{port}")

    for c in collections:
        if not c:
            continue
        print(f"[qdrant] BEFORE collection_exists({c}) ...", flush=True)
        exists = False
        try:
            exists = qc.collection_exists(c)
        except Exception:
            # Older qdrant-client: collection_exists may not exist; fall back to get_collection.
            try:
                qc.get_collection(c)
                exists = True
            except Exception:
                exists = False
        print(f"  exists={exists} ({c})")

        if exists:
            qc.delete_collection(c)

        # after
        try:
            exists2 = qc.collection_exists(c)
        except Exception:
            try:
                qc.get_collection(c)
                exists2 = True
            except Exception:
                exists2 = False
        print(f"  exists_after={exists2} ({c})")


def reset_typedb() -> None:
    from typedb.driver import Credentials, DriverOptions, TransactionType, TypeDB

    host = _env("TYPEDB_HOST", "typedb")
    port = _env("TYPEDB_PORT", "1729")
    database = _env("TYPEDB_DATABASE", "gopedia")
    username = _env("TYPEDB_USERNAME", "admin")
    password = _env("TYPEDB_PASSWORD", "password")

    addr = f"{host}:{port}"
    driver = TypeDB.driver(
        addr,
        Credentials(username, password),
        DriverOptions(is_tls_enabled=False),
    )

    try:
        db_names = [db.name for db in driver.databases.all()]
        print(f"[typedb] databases BEFORE: {db_names}")

        if database not in db_names:
            print(f"[typedb] database not found (skip clear): {database}")
            return

        def _has_any_entity(tx) -> bool:
            # Any of these types existing indicates data.
            q = "match $d isa document; limit 1; fetch $d;"
            try:
                res = tx.query(q).resolve()
                # Typedb result iterator supports next()
                return next(res, None) is not None
            except TypeError:
                # Some driver variants return an object with `iterator()`
                res = tx.query(q).resolve()
                try:
                    it = res.iterator()
                    return next(it, None) is not None
                except Exception:
                    return False
            except Exception:
                return False

        # Clear by deleting all instances (schema is kept).
        with driver.transaction(database, TransactionType.WRITE) as tx:
            before = _has_any_entity(tx)
            print(f"[typedb] has_document_before={before}")

            # Delete relations first (when possible), then entities.
            # If deletes cascade, redundant deletes are harmless.
            for del_q in [
                "match $r isa composition; delete $r;",
                "match $r isa contains; delete $r;",
                "match $r isa mentions; delete $r;",
                "match $s isa section; delete $s;",
                "match $x isa sentence; delete $x;",
                "match $k isa keyword; delete $k;",
                "match $d isa document; delete $d;",
            ]:
                try:
                    tx.query(del_q).resolve()
                except Exception:
                    # Best-effort: if some type doesn't exist yet, ignore.
                    pass
            tx.commit()

        with driver.transaction(database, TransactionType.READ) as tx:
            after = _has_any_entity(tx)
            print(f"[typedb] has_document_after={after}")

    finally:
        driver.close()


def main() -> int:
    # These calls are intentionally independent so one subsystem's failure is visible.
    try:
        reset_postgres()
    except Exception as e:
        print(f"[postgres] reset failed: {e}", file=sys.stderr)
        return 2

    try:
        reset_qdrant()
    except Exception as e:
        print(f"[qdrant] reset failed: {e}", file=sys.stderr)
        return 3

    try:
        reset_typedb()
    except Exception as e:
        print(f"[typedb] reset failed: {e}", file=sys.stderr)
        return 4

    return 0


if __name__ == "__main__":
    raise SystemExit(main())

