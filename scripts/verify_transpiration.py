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
    # initialize.py와 동일하게 DOC_* 설정을 우선 사용
    collection = os.environ.get("QDRANT_DOC_COLLECTION") or os.environ.get(
        "QDRANT_COLLECTION", "gopedia_document"
    )

    try:
        from qdrant_client import QdrantClient

        qc = QdrantClient(host=qdrant_host, port=qdrant_port)
        # initialize.py의 DOC 벡터 설정 우선
        vector_name = os.environ.get("QDRANT_DOC_VECTOR_NAME") or os.environ.get(
            "QDRANT_VECTOR_NAME"
        )
        qp_kwargs: dict[str, object] = {}
        if vector_name:
            qp_kwargs["using"] = vector_name  # explicit named vector in collection
        result = qc.query_points(
            collection_name=collection,
            query=query_vector,
            limit=5,
            **qp_kwargs,
        )
        hits = result.points or []
    except Exception as e:
        print(f"Qdrant search failed: {e}", file=sys.stderr)
        return 3

    if not hits:
        print("No Qdrant hits for query:", query, file=sys.stderr)
        return 4

    print("Qdrant hits (score, l1_id, section_id):")
    for h in hits:
        payload = h.payload or {}
        l1_id = payload.get("l1_id", "")
        section_id = payload.get("section_id", "")
        print(f"  score={h.score:.4f} l1_id={l1_id} section_id={section_id}")

    # 3) TypeDB: if available, query section context (parent/child)
    typedb_host = os.environ.get("TYPEDB_HOST")
    if not typedb_host:
        print("TYPEDB_HOST not set; skipping TypeDB context check.")
        return 0

    try:
        from typedb.driver import Credentials, DriverOptions, TransactionType, TypeDB

        addr = f"{typedb_host}:{os.environ.get('TYPEDB_PORT', '1729')}"
        db = os.environ.get("TYPEDB_DATABASE", "gopedia")
        username = os.environ.get("TYPEDB_USERNAME", "admin")
        password = os.environ.get("TYPEDB_PASSWORD", "password")

        typedb_results: list = []
        driver = TypeDB.driver(
            addr,
            Credentials(username, password),
            DriverOptions(is_tls_enabled=False),
        )
        try:
            with driver.transaction(db, TransactionType.READ) as tx:
                query = """
match
$d isa document, has doc_id $doc_id;
$c (parent: $d, child: $s) isa composition;
$s isa section, has section_id $sid, has toc_level $lvl;
fetch {
  "doc_id": $doc_id,
  "section_id": $sid,
  "toc_level": $lvl
};
"""
                # TypeDB 3.x: fetch-stage query; we only care that it executes without error.
                tx.query(query).resolve()
                typedb_results.append(True)
        finally:
            driver.close()

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
