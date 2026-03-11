# ontology-so

TypeDB and Qdrant schema/init for Gopedia 0.0.1.

## TypeDB

- **Schema**: `typedb_schema.typeql` — `document`, `section`, `composition` (parent/child).
- **Init**: `python typedb_init.py` (requires `typedb-driver`). Uses `TYPEDB_HOST`, `TYPEDB_PORT`, `TYPEDB_DATABASE`.

## Qdrant

- **Vector size**: 1536 (OpenAI).
- **Payload**: `doc_id`, `machine_id`, `toc_path`, `section_id` (set by Phloem sink).
- **Init**: Call `ontologyso.EnsureQdrantCollection(ctx, client, collectionName, 1536)` from Phloem on startup.
