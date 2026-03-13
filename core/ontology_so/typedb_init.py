#!/usr/bin/env python3
"""
Initialize TypeDB for Gopedia 0.0.1:
- Create database if missing
- Apply schema from core/ontology_so/typedb_schema.typeql

Env:
  TYPEDB_HOST (default: localhost)
  TYPEDB_PORT (default: 1729)
  TYPEDB_DATABASE (default: gopedia)
  TYPEDB_USERNAME (default: admin)
  TYPEDB_PASSWORD (default: password)
"""

from __future__ import annotations

import os
import sys
from pathlib import Path


def _env(name: str, default: str) -> str:
    return os.environ.get(name, default)


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    schema_path = repo_root / "core" / "ontology_so" / "typedb_schema.typeql"
    if not schema_path.exists():
        print(f"Schema file not found: {schema_path}", file=sys.stderr)
        return 2

    try:
        from typedb.driver import Credentials, DriverOptions, TransactionType, TypeDB
    except ImportError:
        print("typedb-driver not installed", file=sys.stderr)
        return 3

    host = _env("TYPEDB_HOST", "localhost")
    port = _env("TYPEDB_PORT", "1729")
    database = _env("TYPEDB_DATABASE", "gopedia")
    username = _env("TYPEDB_USERNAME", "admin")
    password = _env("TYPEDB_PASSWORD", "password")
    addr = f"{host}:{port}"

    schema_text = schema_path.read_text(encoding="utf-8", errors="replace")

    driver = TypeDB.driver(
        addr,
        Credentials(username, password),
        DriverOptions(is_tls_enabled=False),
    )
    try:
        dbs = [db.name for db in driver.databases.all()]
        if database not in dbs:
            driver.databases.create(database)
        with driver.transaction(database, TransactionType.SCHEMA) as tx:
            tx.query(schema_text).resolve()
            tx.commit()
    finally:
        driver.close()

    print(f"TypeDB initialized: {database} @ {addr}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
