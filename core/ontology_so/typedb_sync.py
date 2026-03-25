"""
Sync a single document from (doc_id, machine_id, title, content) into TypeDB.
Inserts document + sections + composition hierarchy per core/ontology_so/typedb_schema.typeql.
Used by Root after Phloem IngestMarkdown, or by a batch script reading from PG.
"""
from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Optional

HEADING_RE = re.compile(r"^(#{1,6})\s+(.+)$")
KEYWORD_RE = re.compile(r"[A-Za-z0-9][A-Za-z0-9_-]{2,}")


@dataclass
class SectionRow:
    section_id: str
    toc_level: int
    heading: str
    body: str
    parent_section_id: Optional[str]  # None => parent is document


def parse_toc_and_sections(content: str) -> list[SectionRow]:
    """
    Parse markdown content into a flat list of sections with bodies.
    Section IDs match Go FlattenTOC: s0, s1, s2, ... (depth-first).
    Body is content from the line after the heading until the next heading of same or higher level.
    """
    lines = content.split("\n")
    rows: list[SectionRow] = []
    heading_stack: list[tuple[int, str]] = []  # (level, section_id)
    idx = 0
    i = 0
    while i < len(lines):
        line = lines[i]
        stripped = line.strip()
        m = HEADING_RE.match(stripped)
        if m:
            level = len(m.group(1))
            heading_text = m.group(2).strip()
            # Pop stack until we're under the right parent
            while heading_stack and heading_stack[-1][0] >= level:
                heading_stack.pop()
            parent_sid = heading_stack[-1][1] if heading_stack else None
            section_id = f"s{idx}"
            idx += 1
            heading_stack.append((level, section_id))
            # Body: from next line until next same-or-higher-level heading
            body_lines: list[str] = []
            j = i + 1
            while j < len(lines):
                next_line = lines[j]
                next_stripped = next_line.strip()
                next_m = HEADING_RE.match(next_stripped)
                if next_m:
                    next_level = len(next_m.group(1))
                    if next_level <= level:
                        break
                body_lines.append(next_line)
                j += 1
            body = "\n".join(body_lines).strip() if body_lines else ""
            rows.append(
                SectionRow(
                    section_id=section_id,
                    toc_level=level,
                    heading=heading_text,
                    body=body[:10000] if len(body) > 10000 else body,  # cap for TypeDB
                    parent_section_id=parent_sid,
                )
            )
            i = j
            continue
        i += 1
    return rows


def _escape_typeql_string(s: str) -> str:
    """Escape string for TypeQL attribute value (double quotes, backslash, newlines)."""
    return s.replace("\\", "\\\\").replace('"', '\\"').replace("\n", " ").replace("\r", " ")


def sync_document_to_typedb(
    doc_id: str,
    machine_id: int,
    title: str,
    content: str,
    *,
    typedb_host: str = "",
    typedb_port: str = "1729",
    typedb_database: str = "gopedia",
) -> bool:
    """
    Insert one document and its sections + composition into TypeDB.
    Uses typedb_schema.typeql: document, section, composition (parent/child).
    Returns True on success.
    """
    import os

    if not typedb_host:
        typedb_host = os.environ.get("TYPEDB_HOST", "localhost")
    if not typedb_port:
        typedb_port = os.environ.get("TYPEDB_PORT", "1729")
    if not typedb_database:
        typedb_database = os.environ.get("TYPEDB_DATABASE", "gopedia")

    try:
        from typedb.driver import (
            Credentials,
            DriverOptions,
            TransactionType,
            TypeDB,
        )
    except ImportError:
        raise RuntimeError("typedb-driver not installed") from None

    sections = parse_toc_and_sections(content)
    l3_rows = _fetch_l3_sentences_from_postgres(doc_id)
    addr = f"{typedb_host}:{typedb_port}"

    # Normalize title for TypeQL (cap length, escape)
    doc_id_safe = _escape_typeql_string(str(doc_id)[:256])

    username = os.environ.get("TYPEDB_USERNAME", "admin")
    password = os.environ.get("TYPEDB_PASSWORD", "password")
    source_type = _escape_typeql_string(
        (os.environ.get("GOPEDIA_SOURCE_TYPE") or "md")[:64]
    )
    project_id = _escape_typeql_string(
        (os.environ.get("GOPEDIA_PROJECT_ID") or "0")[:64]
    )

    driver = TypeDB.driver(
        addr,
        Credentials(username, password),
        DriverOptions(is_tls_enabled=False),
    )
    try:
        with driver.transaction(typedb_database, TransactionType.WRITE) as tx:
            # Insert document (align with doc/design: l1_id + source_type + project_id)
            tx.query(
                f'insert $d isa document, has doc_id "{doc_id_safe}", '
                f'has source_type "{source_type}", has project_id "{project_id}";'
            ).resolve()
            # Insert sections and composition
            for row in sections:
                sid_safe = _escape_typeql_string(row.section_id[:256])
                tx.query(
                    f'insert $s isa section, has section_id "{sid_safe}", has toc_level {row.toc_level};'
                ).resolve()
                if row.parent_section_id is None:
                    tx.query(
                        f'match $d isa document, has doc_id "{doc_id_safe}"; '
                        f'$s isa section, has section_id "{sid_safe}"; '
                        "insert (parent: $d, child: $s) isa composition;"
                    ).resolve()
                else:
                    parent_safe = _escape_typeql_string(row.parent_section_id[:256])
                    tx.query(
                        f'match $p isa section, has section_id "{parent_safe}"; '
                        f'$s isa section, has section_id "{sid_safe}"; '
                        "insert (parent: $p, child: $s) isa composition;"
                    ).resolve()

            # Insert sentences (L3) and connect section -> sentence, sentence -> keyword.
            for l3 in l3_rows:
                sid_safe = _escape_typeql_string(l3["section_id"][:256])
                body_safe = _escape_typeql_string((l3["content"] or "")[:10000])
                l3_uuid = str(l3["l3_id"])
                l3_safe = _escape_typeql_string(l3_uuid[:256])

                tx.query(
                    f'insert $x isa sentence, has l3_id "{l3_safe}";'
                ).resolve()
                tx.query(
                    f'match $s isa section, has section_id "{sid_safe}"; '
                    f'$x isa sentence, has l3_id "{l3_safe}"; '
                    "insert (container: $s, contained: $x) isa contains;"
                ).resolve()

                for kw in _extract_keywords(body_safe):
                    kw_id = _keyword_machine_id(kw)
                    tx.query(
                        f"insert $k isa keyword, has keyword_machine_id {kw_id};"
                    ).resolve()
                    tx.query(
                        f'match $x isa sentence, has l3_id "{l3_safe}"; '
                        f"$k isa keyword, has keyword_machine_id {kw_id}; "
                        "insert (source: $x, target: $k) isa mentions;"
                    ).resolve()
            tx.commit()
    finally:
        driver.close()
    return True


def _fetch_l3_sentences_from_postgres(doc_id: str) -> list[dict]:
    """
    Optional: fetch L3 rows from Postgres for TypeDB. Only rows under the revision head
    (documents.current_l1_id, or latest knowledge_l1 by created_at) are returned.
    doc_id is documents.id (UUID string) or a numeric machine_id string (legacy).
    """
    import os
    import uuid as uuid_mod

    host = os.environ.get("POSTGRES_HOST", "")
    user = os.environ.get("POSTGRES_USER", "")
    if not host or not user:
        return []
    try:
        import psycopg
    except Exception:
        return []

    port = os.environ.get("POSTGRES_PORT", "5432")
    password = os.environ.get("POSTGRES_PASSWORD", "")
    db = os.environ.get("POSTGRES_DB", "gopedia")
    conninfo = f"host={host} port={port} user={user} password={password} dbname={db} sslmode=disable"

    rows: list[dict] = []
    with psycopg.connect(conninfo) as conn:
        with conn.cursor() as cur:
            try:
                uuid_mod.UUID(doc_id)
                where_sql = "k1.document_id = %s::uuid"
                param = doc_id
            except ValueError:
                where_sql = "d.machine_id = %s"
                param = int(doc_id)

            cur.execute(
                f"""
                SELECT l3.id::text AS l3_id, l3.content AS content, l2.section_id AS section_id
                  FROM documents d
                  JOIN knowledge_l1 k1 ON k1.document_id = d.id
                    AND k1.id = COALESCE(
                      d.current_l1_id,
                      (
                        SELECT k2.id FROM knowledge_l1 k2
                         WHERE k2.document_id = d.id
                         ORDER BY k2.created_at DESC NULLS LAST
                         LIMIT 1
                      )
                    )
                  JOIN knowledge_l2 l2 ON l2.l1_id = k1.id
                  JOIN knowledge_l3 l3 ON l3.l2_id = l2.id
                 WHERE {where_sql}
                 ORDER BY l3.sort_order, l3.created_at
                """,
                (param,),
            )
            for (l3_id, content, section_id) in cur.fetchall():
                rows.append(
                    {
                        "l3_id": l3_id,
                        "content": content or "",
                        "section_id": section_id or "",
                    }
                )
    return rows


def _extract_keywords(text: str) -> list[str]:
    matches = KEYWORD_RE.findall((text or "").lower())
    out: list[str] = []
    seen: set[str] = set()
    for m in matches:
        if len(m) < 4:
            continue
        if m in seen:
            continue
        seen.add(m)
        out.append(m)
        if len(out) >= 8:
            break
    return out


def _keyword_machine_id(keyword: str) -> int:
    import hashlib

    kw = (keyword or "").strip().lower()
    h = hashlib.sha256(("kw:" + kw).encode("utf-8")).digest()
    # TypeDB `integer` is signed; keep within signed 64-bit range.
    u = int.from_bytes(h[:8], byteorder="big", signed=False)
    max_i64 = (2**63) - 1
    return int(u % max_i64)
