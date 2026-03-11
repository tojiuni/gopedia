#!/usr/bin/env python3
"""Apply TypeDB schema for Gopedia 0.0.1 (document, section, composition)."""
from __future__ import annotations

import os
import sys
from pathlib import Path

# Add repo root for imports if running standalone
sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

try:
    from typedb.driver import TypeDB, SessionType, TransactionType
except ImportError:
    print("Install TypeDB Python driver: pip install typedb-driver", file=sys.stderr)
    sys.exit(1)


def main() -> None:
    host = os.environ.get("TYPEDB_HOST", "localhost")
    port = os.environ.get("TYPEDB_PORT", "1729")
    database = os.environ.get("TYPEDB_DATABASE", "gopedia")
    addr = f"{host}:{port}"

    schema_path = Path(__file__).parent / "typedb_schema.typeql"
    schema_text = schema_path.read_text()

    with TypeDB.core_driver(addr) as driver:
        if database not in [db.name for db in driver.databases.all()]:
            driver.databases.create(database)
        with driver.session(database, SessionType.SCHEMA) as session:
            with session.transaction(TransactionType.WRITE) as tx:
                tx.query.define(schema_text)
                tx.commit()
    print(f"Schema applied to database '{database}' at {addr}")


if __name__ == "__main__":
    main()
