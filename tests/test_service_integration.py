"""
Gopedia 서비스 연동 통합 테스트. 연결 실패 시 skip 없이 fail.
"""
from __future__ import annotations

import os
import socket
from pathlib import Path

import pytest

_REPO_ROOT = Path(__file__).resolve().parents[1]
if str(_REPO_ROOT) not in __import__("sys").path:
    __import__("sys").path.insert(0, str(_REPO_ROOT))


def _typedb_configured() -> bool:
    return bool(os.environ.get("TYPEDB_HOST"))


@pytest.mark.integration
@pytest.mark.skipif(not _typedb_configured(), reason="TYPEDB_HOST not set")
def test_typedb_connect(typedb_host: str, typedb_port: str, typedb_database: str) -> None:
    try:
        from typedb.driver import TypeDB, SessionType, TransactionType
    except ImportError:
        pytest.skip("typedb-driver not installed")

    addr = f"{typedb_host}:{typedb_port}"
    with TypeDB.core_driver(addr) as driver:
        dbs = [db.name for db in driver.databases.all()]
        assert typedb_database in dbs, f"Database '{typedb_database}' not found (run core/ontology-so/typedb_init.py)"
        with driver.session(typedb_database, SessionType.DATA) as session:
            with session.transaction(TransactionType.READ) as tx:
                list(tx.query.match("match $x isa document; get $x; limit 1;"))


def _qdrant_configured() -> bool:
    return bool(os.environ.get("QDRANT_HOST"))


@pytest.mark.integration
@pytest.mark.skipif(not _qdrant_configured(), reason="QDRANT_HOST not set")
def test_qdrant_http_connect(qdrant_host: str, qdrant_port: int) -> None:
    try:
        from qdrant_client import QdrantClient
    except ImportError:
        pytest.skip("qdrant-client not installed (pip install qdrant-client)")

    client = QdrantClient(host=qdrant_host, port=qdrant_port)
    collections = client.get_collections().collections
    assert isinstance(collections, list)


@pytest.mark.integration
@pytest.mark.skipif(not _qdrant_configured(), reason="QDRANT_HOST not set")
def test_qdrant_collection_optional(qdrant_host: str, qdrant_port: int) -> None:
    try:
        from qdrant_client import QdrantClient
    except ImportError:
        pytest.skip("qdrant-client not installed")

    collection = os.environ.get("QDRANT_COLLECTION", "gopedia_markdown")
    client = QdrantClient(host=qdrant_host, port=qdrant_port)
    try:
        info = client.get_collection(collection)
        assert info is not None
    except Exception as e:
        if "Not found" in str(e) or "does not exist" in str(e).lower():
            pytest.skip(f"Collection {collection} not created yet")
        raise


def _postgres_configured() -> bool:
    return bool(os.environ.get("POSTGRES_HOST") and os.environ.get("POSTGRES_USER"))


@pytest.mark.integration
@pytest.mark.skipif(not _postgres_configured(), reason="POSTGRES_HOST/POSTGRES_USER not set")
def test_postgres_connect(
    postgres_host: str,
    postgres_port: str,
    postgres_user: str,
    postgres_password: str,
    postgres_db: str,
) -> None:
    try:
        import psycopg
    except ImportError:
        pytest.skip("psycopg not installed")

    conninfo = (
        f"host={postgres_host} port={postgres_port} user={postgres_user} "
        f"password={postgres_password} dbname={postgres_db} sslmode=disable"
    )
    with psycopg.connect(conninfo) as conn:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'documents')"
            )
            exists = cur.fetchone()[0]
    assert exists, "documents table not found (run core/ontology-so/postgres_ddl.sql)"


def _phloem_addr() -> str:
    addr = os.environ.get("GOPEDIA_PHLOEM_GRPC_ADDR", "localhost:50051")
    return addr or "localhost:50051"


def _phloem_reachable(addr: str) -> bool:
    try:
        host, port_str = addr.rsplit(":", 1)
        port = int(port_str)
    except (ValueError, TypeError):
        return False
    try:
        with socket.create_connection((host, port), timeout=2):
            return True
    except (socket.error, OSError):
        return False


@pytest.mark.integration
def test_phloem_grpc_reachable(phloem_grpc_addr: str) -> None:
    if not _phloem_reachable(phloem_grpc_addr):
        pytest.fail(f"Phloem gRPC server not reachable at {phloem_grpc_addr} (start with: go run ./cmd/phloem)")


@pytest.mark.integration
def test_phloem_ingest_markdown(phloem_grpc_addr: str) -> None:
    if not _phloem_reachable(phloem_grpc_addr):
        pytest.fail("Phloem gRPC server not reachable")

    sys_path = __import__("sys").path
    gen_python = _REPO_ROOT / "core" / "proto" / "gen" / "python"
    if str(gen_python) not in sys_path:
        sys_path.insert(0, str(gen_python))

    try:
        import rhizome_pb2
        import rhizome_pb2_grpc
    except ImportError:
        pytest.skip("Proto Python stubs not generated (cd core/proto && buf generate)")

    import grpc

    channel = grpc.insecure_channel(phloem_grpc_addr)
    stub = rhizome_pb2_grpc.PhloemStub(channel)
    req = rhizome_pb2.IngestRequest(
        title="integration_test_doc",
        content="# Test\nMinimal content for integration test.",
    )
    try:
        resp = stub.IngestMarkdown(req, timeout=10)
        assert resp is not None
        assert hasattr(resp, "machine_id") and hasattr(resp, "doc_id")
    except grpc.RpcError as e:
        if e.code() == grpc.StatusCode.UNAVAILABLE:
            pytest.fail("Phloem unavailable")
        raise
    finally:
        channel.close()
