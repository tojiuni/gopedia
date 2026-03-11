"""
DB 초기화: PostgreSQL, TypeDB, Qdrant 테이블/DB/컬렉션이 없으면 생성.
연동 테스트 전 또는 docker compose up 후 한 번 실행할 때 사용.
"""
from __future__ import annotations

import os
from pathlib import Path
from typing import Optional

_REPO_ROOT = Path(__file__).resolve().parents[1]
_ONTOLOGY_DIR = _REPO_ROOT / "core" / "ontology-so"


def _env(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


class DBInitializer:
    """각 DB(PostgreSQL, TypeDB, Qdrant)의 테이블/데이터베이스/컬렉션이 없으면 생성하는 초기화 클래스."""

    def __init__(
        self,
        *,
        postgres_host: Optional[str] = None,
        postgres_port: Optional[str] = None,
        postgres_user: Optional[str] = None,
        postgres_password: Optional[str] = None,
        postgres_db: Optional[str] = None,
        typedb_host: Optional[str] = None,
        typedb_port: Optional[str] = None,
        typedb_database: Optional[str] = None,
        qdrant_host: Optional[str] = None,
        qdrant_port: Optional[int] = None,
        qdrant_collection: Optional[str] = None,
        qdrant_vector_size: int = 1536,
    ) -> None:
        self.postgres_host = postgres_host or _env("POSTGRES_HOST", "")
        self.postgres_port = postgres_port or _env("POSTGRES_PORT", "5432")
        self.postgres_user = postgres_user or _env("POSTGRES_USER", "")
        self.postgres_password = postgres_password or _env("POSTGRES_PASSWORD", "")
        self.postgres_db = postgres_db or _env("POSTGRES_DB", "gopedia")
        self.typedb_host = typedb_host or _env("TYPEDB_HOST", "localhost")
        self.typedb_port = typedb_port or _env("TYPEDB_PORT", "1729")
        self.typedb_database = typedb_database or _env("TYPEDB_DATABASE", "gopedia")
        self.qdrant_host = qdrant_host or _env("QDRANT_HOST", "localhost")
        self.qdrant_port = qdrant_port or int(_env("QDRANT_PORT", "6333"))
        self.qdrant_collection = qdrant_collection or _env("QDRANT_COLLECTION", "gopedia_markdown")
        self.qdrant_vector_size = qdrant_vector_size

    def init_postgres(self) -> bool:
        """PostgreSQL: documents 테이블이 없으면 생성. 성공 시 True."""
        if not self.postgres_host or not self.postgres_user:
            return False
        try:
            import psycopg
        except ImportError:
            return False
        conninfo = (
            f"host={self.postgres_host} port={self.postgres_port} user={self.postgres_user} "
            f"password={self.postgres_password} dbname={self.postgres_db} sslmode=disable"
        )
        ddl_path = _ONTOLOGY_DIR / "postgres_ddl.sql"
        if not ddl_path.exists():
            return False
        ddl = ddl_path.read_text()
        with psycopg.connect(conninfo) as conn:
            with conn.cursor() as cur:
                cur.execute(ddl)
                conn.commit()
        return True

    def init_typedb(self) -> bool:
        """TypeDB: DB가 없으면 생성하고 스키마 적용. 성공 시 True."""
        if not self.typedb_host:
            return False
        try:
            from typedb.driver import TypeDB, SessionType, TransactionType
        except ImportError:
            return False
        schema_path = _ONTOLOGY_DIR / "typedb_schema.typeql"
        if not schema_path.exists():
            return False
        schema_text = schema_path.read_text()
        addr = f"{self.typedb_host}:{self.typedb_port}"
        with TypeDB.core_driver(addr) as driver:
            dbs = [db.name for db in driver.databases.all()]
            if self.typedb_database not in dbs:
                driver.databases.create(self.typedb_database)
            with driver.session(self.typedb_database, SessionType.SCHEMA) as session:
                with session.transaction(TransactionType.WRITE) as tx:
                    tx.query.define(schema_text)
                    tx.commit()
        return True

    def init_qdrant(self) -> bool:
        """Qdrant: 컬렉션이 없으면 생성. 성공 시 True."""
        if not self.qdrant_host:
            return False
        try:
            from qdrant_client import QdrantClient
            from qdrant_client.models import VectorParams, Distance
        except ImportError:
            return False
        client = QdrantClient(host=self.qdrant_host, port=self.qdrant_port)
        try:
            client.get_collection(self.qdrant_collection)
            return True
        except Exception:
            pass
        client.create_collection(
            collection_name=self.qdrant_collection,
            vectors_config=VectorParams(
                size=self.qdrant_vector_size,
                distance=Distance.COSINE,
            ),
        )
        return True

    def init_all(self, *, skip_missing: bool = True) -> dict[str, bool]:
        """PostgreSQL, TypeDB, Qdrant 순서로 초기화. skip_missing=True면 env 없으면 스킵.
        Returns: {"postgres": bool, "typedb": bool, "qdrant": bool}
        """
        out: dict[str, bool] = {}
        if self.postgres_host and self.postgres_user:
            try:
                out["postgres"] = self.init_postgres()
            except Exception:
                out["postgres"] = False
        elif not skip_missing:
            out["postgres"] = False

        if self.typedb_host:
            try:
                out["typedb"] = self.init_typedb()
            except Exception:
                out["typedb"] = False
        elif not skip_missing:
            out["typedb"] = False

        if self.qdrant_host:
            try:
                out["qdrant"] = self.init_qdrant()
            except Exception:
                out["qdrant"] = False
        elif not skip_missing:
            out["qdrant"] = False

        return out
