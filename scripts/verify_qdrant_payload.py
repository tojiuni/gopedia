#!/usr/bin/env python3
"""
Direct Qdrant verification:
- prints point counts for both the document collection and the markdown (Phloem) collection
- prints sample payload keys and selected fields from a few points

Exit codes:
  0 = success (queries executed; doesn't enforce strict payload schema)
  3 = Qdrant connection/query failed
"""

from __future__ import annotations

import os


def main() -> int:
    try:
        from qdrant_client import QdrantClient
    except ImportError as e:
        print(f"qdrant-client not installed: {e}")
        return 3

    qdrant_host = os.environ.get("QDRANT_HOST", "qdrant")
    qdrant_port = int(os.environ.get("QDRANT_PORT", "6333"))

    collection = os.environ.get("QDRANT_COLLECTION", "gopedia_markdown")
    doc_collection = os.environ.get("QDRANT_DOC_COLLECTION", "gopedia_document")

    qc = QdrantClient(host=qdrant_host, port=qdrant_port)

    print(f"Qdrant host={qdrant_host}:{qdrant_port}")
    print(f"Collections: collection={collection}, doc_collection={doc_collection}")

    for c in (doc_collection, collection):
        cnt = qc.count(collection_name=c, exact=True).count
        print(f"  count[{c}] = {cnt}")
        res = qc.scroll(collection_name=c, limit=3, with_payload=True, with_vectors=False)
        points = res[0] if res else []
        for i, p in enumerate(points):
            payload = p.payload or {}
            keys = sorted(payload.keys())
            print(f"    sample[{c}#{i}] keys={keys}")
            # likely fields for Phloem vectors
            interesting = {
                "l1_id": payload.get("l1_id"),
                "l2_id": payload.get("l2_id"),
                "l3_id": payload.get("l3_id"),
                "section_id": payload.get("section_id"),
                "section_type": payload.get("section_type"),
                "keyword_ids": payload.get("keyword_ids"),
                "version": payload.get("version"),
                "version_id": payload.get("version_id"),
                "source_type": payload.get("source_type"),
                "project_id": payload.get("project_id"),
            }
            print(f"    sample[{c}#{i}] interesting={interesting}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())

