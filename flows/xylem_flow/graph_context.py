"""Graph context expansion via TypeDB.

Entry point:
  get_related_l1_ids(hit_l1_ids, project_id, depth=1) -> list[str]

For each hit file (l1_id), traverses the TypeDB `contains` relation to find
the parent directory, then returns l1_ids of all sibling files in that
directory. This expands Qdrant retrieval results with structurally-related
documents.

Graceful degradation: returns [] on any error or when TYPEDB_HOST is unset.
"""
from __future__ import annotations

import os
from typing import Optional


def get_related_l1_ids(
    hit_l1_ids: list[str],
    project_id: int | str,
    depth: int = 1,
    *,
    typedb_host: Optional[str] = None,
    typedb_port: Optional[str] = None,
    typedb_database: Optional[str] = None,
) -> list[str]:
    """Return l1_ids of files in the same directories as the hit files.

    Traversal (depth=1):
      hit file (l1_id) → parent directory → sibling files (l1_ids)

    The hit l1_ids themselves are excluded from the result.
    Returns [] when TypeDB is unreachable or TYPEDB_HOST is not configured.
    """
    host = typedb_host or os.environ.get("TYPEDB_HOST", "")
    if not host:
        return []

    try:
        return _fetch_sibling_l1_ids(
            hit_l1_ids=hit_l1_ids,
            project_id=str(project_id),
            host=host,
            port=typedb_port or os.environ.get("TYPEDB_PORT", "1729"),
            database=typedb_database or os.environ.get("TYPEDB_DATABASE", "gopedia"),
        )
    except Exception:
        return []


def _fetch_sibling_l1_ids(
    hit_l1_ids: list[str],
    project_id: str,
    host: str,
    port: str,
    database: str,
) -> list[str]:
    """Inner implementation — may raise; callers must catch."""
    from typedb.driver import Credentials, DriverOptions, TransactionType, TypeDB

    username = os.environ.get("TYPEDB_USERNAME", "admin")
    password = os.environ.get("TYPEDB_PASSWORD", "password")

    driver = TypeDB.driver(
        f"{host}:{port}",
        Credentials(username, password),
        DriverOptions(is_tls_enabled=False),
    )

    hit_set = set(hit_l1_ids)
    related: set[str] = set()

    try:
        with driver.transaction(database, TransactionType.READ) as tx:
            for l1_id in hit_l1_ids:
                l1_safe = l1_id.replace('"', '\\"')
                proj_safe = project_id.replace('"', '\\"')

                # Step 1: find parent directories of this file
                dir_query = (
                    f'match $f isa file, has l1_id "{l1_safe}"; '
                    f'$d isa directory, has project_id "{proj_safe}"; '
                    "(container: $d, contained: $f) isa contains; "
                    "fetch $d: dir_path;"
                )
                try:
                    dir_result = list(tx.query(dir_query).resolve())
                except Exception:
                    continue

                for item in dir_result:
                    try:
                        dir_path = item.get("d", {}).get("dir_path", [{}])[0].get("value", "")
                    except Exception:
                        continue
                    if not dir_path:
                        continue

                    dir_safe = dir_path.replace('"', '\\"')

                    # Step 2: find all sibling files in this directory
                    sibling_query = (
                        f'match $d isa directory, has dir_path "{dir_safe}", '
                        f'has project_id "{proj_safe}"; '
                        "$sib isa file; "
                        "(container: $d, contained: $sib) isa contains; "
                        "fetch $sib: l1_id;"
                    )
                    try:
                        sib_result = list(tx.query(sibling_query).resolve())
                    except Exception:
                        continue

                    for sib_item in sib_result:
                        try:
                            sib_l1_id = (
                                sib_item.get("sib", {}).get("l1_id", [{}])[0].get("value", "")
                            )
                        except Exception:
                            continue
                        if sib_l1_id and sib_l1_id not in hit_set:
                            related.add(sib_l1_id)
    finally:
        driver.close()

    return list(related)
