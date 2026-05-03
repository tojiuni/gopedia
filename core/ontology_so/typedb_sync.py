"""
Sync documents and directory trees from PostgreSQL into TypeDB.

Schema: core/ontology_so/typedb_schema.typeql
  directory → file → section → chunk  (via `contains` relation)

Public API:
  sync_document_to_typedb(l1_id, project_id, source_type)
  sync_directory_tree_to_typedb(project_id, l1_rows)
"""
from __future__ import annotations

import os


def _escape(s: str) -> str:
    """Escape a string value for TypeQL (double quotes, backslash, newlines)."""
    return s.replace("\\", "\\\\").replace('"', '\\"').replace("\n", " ").replace("\r", " ")


def _typedb_driver(typedb_host: str = "", typedb_port: str = "1729"):
    """Return a connected TypeDB driver. Raises if typedb-driver not installed."""
    try:
        from typedb.driver import Credentials, DriverOptions, TypeDB
    except ImportError:
        raise RuntimeError("typedb-driver not installed") from None

    host = typedb_host or os.environ.get("TYPEDB_HOST", "localhost")
    port = typedb_port or os.environ.get("TYPEDB_PORT", "1729")
    username = os.environ.get("TYPEDB_USERNAME", "admin")
    password = os.environ.get("TYPEDB_PASSWORD", "password")
    return TypeDB.driver(
        f"{host}:{port}",
        Credentials(username, password),
        DriverOptions(is_tls_enabled=False),
    )


def _fetch_l2_l3_rows(l1_id: str) -> list[dict]:
    """Fetch L2+L3 rows from PostgreSQL for a given L1 UUID."""
    import psycopg

    host = os.environ.get("POSTGRES_HOST", "")
    user = os.environ.get("POSTGRES_USER", "")
    if not host or not user:
        return []

    conninfo = (
        f"host={host} port={os.environ.get('POSTGRES_PORT', '5432')} "
        f"user={user} password={os.environ.get('POSTGRES_PASSWORD', '')} "
        f"dbname={os.environ.get('POSTGRES_DB', 'gopedia')} sslmode=disable"
    )
    rows: list[dict] = []
    with psycopg.connect(conninfo) as conn:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT
                  l2.id::text   AS l2_id,
                  l2.section_id AS section_id,
                  l3.id::text   AS l3_id
                FROM knowledge_l2 l2
                JOIN knowledge_l3 l3 ON l3.l2_id = l2.id
                WHERE l2.l1_id = %s::uuid
                ORDER BY l2.sort_order, l3.sort_order
                """,
                (l1_id,),
            )
            for (l2_id, section_id, l3_id) in cur.fetchall():
                rows.append(
                    {
                        "l2_id": str(l2_id),
                        "section_id": str(section_id or ""),
                        "l3_id": str(l3_id),
                    }
                )
    return rows


def _mark_synced(l1_id: str) -> None:
    """Update knowledge_l1.typedb_synced_at to now(). Best-effort, never raises."""
    try:
        import psycopg

        host = os.environ.get("POSTGRES_HOST", "")
        user = os.environ.get("POSTGRES_USER", "")
        if not host or not user:
            return
        conninfo = (
            f"host={host} port={os.environ.get('POSTGRES_PORT', '5432')} "
            f"user={user} password={os.environ.get('POSTGRES_PASSWORD', '')} "
            f"dbname={os.environ.get('POSTGRES_DB', 'gopedia')} sslmode=disable"
        )
        with psycopg.connect(conninfo) as conn:
            conn.execute(
                "UPDATE knowledge_l1 SET typedb_synced_at = now() WHERE id = %s::uuid",
                (l1_id,),
            )
            conn.commit()
    except Exception:
        pass


def sync_document_to_typedb(
    l1_id: str,
    project_id: str,
    source_type: str = "md",
    *,
    typedb_host: str = "",
    typedb_port: str = "1729",
    typedb_database: str = "gopedia",
) -> bool:
    """Insert one file (L1) with its sections (L2) and chunks (L3) into TypeDB.

    Hierarchy inserted:  file → section → chunk  via `contains` relation.
    Bridge attributes:   file.l1_id, section.l2_id, chunk.l3_id  (PG UUIDs).

    Returns True on success.
    """
    if not typedb_database:
        typedb_database = os.environ.get("TYPEDB_DATABASE", "gopedia")

    l2_l3_rows = _fetch_l2_l3_rows(l1_id)
    l1_safe = _escape(str(l1_id)[:256])
    proj_safe = _escape(str(project_id)[:64])
    src_safe = _escape((source_type or "md")[:64])

    from typedb.driver import TransactionType  # imported here to allow mocking in tests
    driver = _typedb_driver(typedb_host, typedb_port)
    try:
        with driver.transaction(typedb_database, TransactionType.WRITE) as tx:
            # Insert file entity
            tx.query(
                f'insert $f isa file, has l1_id "{l1_safe}", '
                f'has source_type "{src_safe}", has project_id "{proj_safe}";'
            ).resolve()

            # Insert sections (L2) + file→section contains
            seen_l2: set[str] = set()
            for row in l2_l3_rows:
                l2_id = row["l2_id"]
                if l2_id in seen_l2:
                    continue
                seen_l2.add(l2_id)
                l2_safe = _escape(l2_id[:256])
                sid_safe = _escape(row["section_id"][:256])
                tx.query(
                    f'insert $s isa section, has l2_id "{l2_safe}", '
                    f'has section_id "{sid_safe}", has toc_level 1;'
                ).resolve()
                tx.query(
                    f'match $f isa file, has l1_id "{l1_safe}"; '
                    f'$s isa section, has l2_id "{l2_safe}"; '
                    "insert (container: $f, contained: $s) isa contains;"
                ).resolve()

            # Insert chunks (L3) + section→chunk contains
            for row in l2_l3_rows:
                l2_safe = _escape(row["l2_id"][:256])
                l3_safe = _escape(row["l3_id"][:256])
                tx.query(
                    f'insert $c isa chunk, has l3_id "{l3_safe}";'
                ).resolve()
                tx.query(
                    f'match $s isa section, has l2_id "{l2_safe}"; '
                    f'$c isa chunk, has l3_id "{l3_safe}"; '
                    "insert (container: $s, contained: $c) isa contains;"
                ).resolve()

            tx.commit()
    finally:
        driver.close()

    _mark_synced(l1_id)
    return True


def sync_directory_tree_to_typedb(
    project_id: int | str,
    l1_rows: list[dict],
    *,
    typedb_host: str = "",
    typedb_port: str = "1729",
    typedb_database: str = "gopedia",
) -> bool:
    """Build directory → file `contains` relations in TypeDB from L1 node rows.

    l1_rows: output of tree.build_project_l1_tree() (nested) or
             tree.fetch_project_l1_nodes() (flat).
    Each row must have: id (l1_id UUID str), title (file path / name),
    optionally parent_id and children.

    Directory path is derived from the title treated as a POSIX path
    (dirname). If title has no parent dir component, "/" is used.

    Returns True on success.
    """
    import posixpath

    if not typedb_database:
        typedb_database = os.environ.get("TYPEDB_DATABASE", "gopedia")

    proj_safe = _escape(str(project_id)[:64])

    # Flatten nested tree if needed
    flat: list[dict] = []

    def _flatten(nodes: list[dict]) -> None:
        for n in nodes:
            flat.append(n)
            if n.get("children"):
                _flatten(n["children"])

    if l1_rows and "children" in l1_rows[0]:
        _flatten(l1_rows)
    else:
        flat.extend(l1_rows)

    if not flat:
        return True

    from typedb.driver import TransactionType  # imported here to allow mocking in tests
    driver = _typedb_driver(typedb_host, typedb_port)
    try:
        with driver.transaction(typedb_database, TransactionType.WRITE) as tx:
            seen_dirs: set[str] = set()

            for node in flat:
                l1_id = str(node.get("id", ""))
                title = str(node.get("title", ""))
                if not l1_id:
                    continue

                dir_path = posixpath.dirname(title.replace("\\", "/")) or "/"
                l1_safe = _escape(l1_id[:256])
                dir_safe = _escape(dir_path[:512])

                # Upsert directory once per (project, dir_path) pair
                dir_key = f"{proj_safe}:{dir_safe}"
                if dir_key not in seen_dirs:
                    seen_dirs.add(dir_key)
                    tx.query(
                        f'insert $d isa directory, has dir_path "{dir_safe}", '
                        f'has project_id "{proj_safe}";'
                    ).resolve()

                # directory → file contains
                tx.query(
                    f'match $d isa directory, has dir_path "{dir_safe}", '
                    f'has project_id "{proj_safe}"; '
                    f'$f isa file, has l1_id "{l1_safe}"; '
                    "insert (container: $d, contained: $f) isa contains;"
                ).resolve()

            tx.commit()
    finally:
        driver.close()

    return True
