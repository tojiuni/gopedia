#!/usr/bin/env python3
"""
Transpiration test: keyword query -> Qdrant search -> TypeDB section context.
Verifies that Qdrant returns relevant TOC/sections and TypeDB can be queried for parent/child.
Exit codes: 0 = success (>=1 Qdrant hit; if TYPEDB_HOST set, >=1 TypeDB result), 1 = usage, 2 = OpenAI, 3 = Qdrant, 4 = no hits, 5 = TypeDB empty/fail.
Usage: python scripts/verify_transpiration.py "keyword"
Env: QDRANT_HOST, QDRANT_PORT, OPENAI_API_KEY, TYPEDB_HOST, TYPEDB_PORT, TYPEDB_DATABASE
"""
from __future__ import annotations

import os
import sys
from pathlib import Path

repo_root = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(repo_root))


def main() -> int:
    if len(sys.argv) < 2:
        print('Usage: python scripts/verify_transpiration.py "keyword"', file=sys.stderr)
        return 1
    query = sys.argv[1]

    # 1) Embed query via OpenAI
    try:
        from openai import OpenAI
        client = OpenAI(api_key=os.environ.get("OPENAI_API_KEY"))
        r = client.embeddings.create(model=os.environ.get("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"), input=query)
        query_vector = r.data[0].embedding
    except Exception as e:
        print(f"OpenAI embedding failed: {e}", file=sys.stderr)
        return 2

    # 2) Qdrant search
    qdrant_host = os.environ.get("QDRANT_HOST", "localhost")
    qdrant_port = int(os.environ.get("QDRANT_PORT", "6333"))
    collection = os.environ.get("QDRANT_COLLECTION", "gopedia_markdown")

    try:
        from qdrant_client import QdrantClient
        qc = QdrantClient(host=qdrant_host, port=qdrant_port)
        hits = qc.search(
            collection_name=collection,
            query_vector=query_vector,
            limit=5,
        )
    except Exception as e:
        print(f"Qdrant search failed: {e}", file=sys.stderr)
        return 3

    if not hits:
        print("No Qdrant hits for query:", query, file=sys.stderr)
        return 4

    print("Qdrant hits (score, doc_id, section_id, toc_path):")
    for h in hits:
        payload = h.payload or {}
        doc_id = payload.get("doc_id", "")
        section_id = payload.get("section_id", "")
        toc_path = payload.get("toc_path", "")
        print(f"  score={h.score:.4f} doc_id={doc_id} section_id={section_id} toc_path={toc_path}")

    # 3) TypeDB: if available, query section context (parent/child)
    typedb_host = os.environ.get("TYPEDB_HOST")
    if not typedb_host:
        print("TYPEDB_HOST not set; skipping TypeDB context check.")
        return 0

    try:
        from typedb.driver import TypeDB, SessionType, TransactionType
        addr = f"{typedb_host}:{os.environ.get('TYPEDB_PORT', '1729')}"
        db = os.environ.get("TYPEDB_DATABASE", "gopedia")
        typedb_results: list = []
        with TypeDB.core_driver(addr) as driver:
            with driver.session(db, SessionType.DATA) as session:
                with session.transaction(TransactionType.READ) as tx:
                    result = tx.query.match(
                        'match $d isa document, has doc_id $doc_id, has title $title;'
                        ' $c (parent: $d, child: $s) isa composition;'
                        ' $s isa section, has section_id $sid, has toc_level $lvl, has body $body;'
                        ' get $doc_id, $title, $sid, $lvl, $body; limit 10;'
                    )
                    for ans in result:
                        typedb_results.append(ans)
                        print("TypeDB:", ans)
        if not typedb_results:
            print("TypeDB: no document/section composition results (expected >=1 for E2E).", file=sys.stderr)
            return 5
    except ImportError:
        print("typedb-driver not installed; skipping TypeDB.", file=sys.stderr)
    except Exception as e:
        print(f"TypeDB query failed: {e}", file=sys.stderr)
        return 5

    print("Transpiration check done.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
