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
    addr = f"{typedb_host}:{typedb_port}"

    # Normalize title for TypeQL (cap length, escape)
    title_safe = _escape_typeql_string((title or "Untitled")[:2000])
    doc_id_safe = _escape_typeql_string(str(doc_id)[:256])

    username = os.environ.get("TYPEDB_USERNAME", "admin")
    password = os.environ.get("TYPEDB_PASSWORD", "password")

    driver = TypeDB.driver(
        addr,
        Credentials(username, password),
        DriverOptions(is_tls_enabled=False),
    )
    try:
        with driver.transaction(typedb_database, TransactionType.WRITE) as tx:
            # Insert document
            tx.query(
                f'insert $d isa document, has doc_id "{doc_id_safe}", has title "{title_safe}";'
            ).resolve()
            # Insert sections and composition
            for row in sections:
                sid_safe = _escape_typeql_string(row.section_id[:256])
                body_safe = _escape_typeql_string(row.body[:10000])
                tx.query(
                    f'insert $s isa section, has section_id "{sid_safe}", '
                    f'has toc_level {row.toc_level}, has body "{body_safe}";'
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
            tx.commit()
    finally:
        driver.close()
    return True
