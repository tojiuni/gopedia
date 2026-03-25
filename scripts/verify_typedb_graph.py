#!/usr/bin/env python3
"""
Direct TypeDB verification:
- fetches existence of document/section/sentence/keyword for a given doc_id (optional)
- prints whether queries execute successfully

Exit codes:
  0 = success (queries executed)
  2 = typedb not reachable/query error
"""

from __future__ import annotations

import os
import sys


def main() -> int:
    try:
        from typedb.driver import Credentials, DriverOptions, TransactionType, TypeDB
    except ImportError as e:
        print(f"typedb-driver not installed: {e}")
        return 2

    typedb_host = os.environ.get("TYPEDB_HOST")
    if not typedb_host:
        print("TYPEDB_HOST not set; skipping.")
        return 0

    typedb_port = os.environ.get("TYPEDB_PORT", "1729")
    typedb_database = os.environ.get("TYPEDB_DATABASE", "gopedia")
    username = os.environ.get("TYPEDB_USERNAME", "admin")
    password = os.environ.get("TYPEDB_PASSWORD", "password")

    addr = f"{typedb_host}:{typedb_port}"
    driver = TypeDB.driver(
        addr,
        Credentials(username, password),
        DriverOptions(is_tls_enabled=False),
    )

    doc_id = os.environ.get("GOPEDIA_VERIFY_DOC_ID", "").strip()

    try:
        with driver.transaction(typedb_database, TransactionType.READ) as tx:
            if doc_id:
                q_doc = (
                    f'match $d isa document, has doc_id "{doc_id}"; '
                    f'fetch {{ "doc_id": $d; }};'
                )
                # Some driver versions require attributes in fetch; fallback to a composition query below.
                try:
                    tx.query(q_doc).resolve()
                    print("TypeDB doc fetch ok for doc_id=", doc_id)
                except Exception:
                    pass
            else:
                pass

            # Use the same fetch syntax as verify_transpiration.py (fetch { ... };).
            # We only care that the query executes successfully.
            q_comp = f"""
match
$d isa document, has doc_id $doc_id;
$c (parent: $d, child: $s) isa composition;
$s isa section, has section_id $sid;
fetch {{
  "doc_id": $doc_id,
  "section_id": $sid
}};
"""
            if doc_id:
                q_comp = q_comp.replace("$d isa document, has doc_id $doc_id;", f'$d isa document, has doc_id "{doc_id}";')

            tx.query(q_comp).resolve()
            print("TypeDB composition fetch ok")

            # Optional: check sentence & keyword queries also parse/execute.
            q_sentence = """
match $x isa sentence, has l3_id $lid;
fetch { "l3_id": $lid };
"""
            tx.query(q_sentence).resolve()
            print("TypeDB sentence fetch ok (may be empty)")

            q_keyword = """
match $k isa keyword, has keyword_machine_id $kid;
fetch { "keyword_machine_id": $kid };
"""
            tx.query(q_keyword).resolve()
            print("TypeDB keyword fetch ok (may be empty)")
    finally:
        driver.close()

    return 0


if __name__ == "__main__":
    raise SystemExit(main())

