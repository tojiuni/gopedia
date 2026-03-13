"""
DB 초기화: PostgreSQL, TypeDB, Qdrant 테이블/DB/컬렉션이 없으면 생성.
연동 테스트 전 또는 docker compose up 후 한 번 실행할 때 사용.
"""
from __future__ import annotations

import logging
import os
from pathlib import Path
from typing import Optional

_REPO_ROOT = Path(__file__).resolve().parents[1]
_ONTOLOGY_DIR = _REPO_ROOT / "core" / "ontology_so"

logger = logging.getLogger(__name__)


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
        qdrant_doc_collection: Optional[str] = None,
        qdrant_doc_vector_name: Optional[str] = None,
        qdrant_doc_vector_size: Optional[int] = None,
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
        self.qdrant_doc_collection = qdrant_doc_collection or _env(
            "QDRANT_DOC_COLLECTION", "gopedia_document"
        )
        self.qdrant_doc_vector_name = qdrant_doc_vector_name or _env(
            "QDRANT_DOC_VECTOR_NAME", "wiki"
        )
        self.qdrant_doc_vector_size = int(
            qdrant_doc_vector_size or _env("QDRANT_DOC_VECTOR_SIZE", "1536")
        )

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
        """TypeDB: DB가 없으면 생성하고 스키마 적용. 성공 시 True.

        typedb-driver 3.x API를 사용한다.
        """
        if not self.typedb_host:
            logger.error("TypeDB host가 설정되지 않았습니다.")
            return False

        try:
            from typedb.driver import (
                Credentials,
                DriverOptions,
                TransactionType,
                TypeDB,
                TypeDBDriverException,
            )
        except ImportError:
            logger.error("typedb-driver 가 설치되어 있지 않습니다.")
            return False

        schema_path = _ONTOLOGY_DIR / "typedb_schema.typeql"
        if not schema_path.exists():
            logger.error("TypeDB 스키마 파일을 찾을 수 없습니다: %s", schema_path)
            return False

        addr = f"{self.typedb_host}:{self.typedb_port}"
        credentials = Credentials(
            _env("TYPEDB_USERNAME", "admin"),
            _env("TYPEDB_PASSWORD", "password"),
        )
        options = DriverOptions(is_tls_enabled=False)

        try:
            with TypeDB.driver(addr, credentials, options) as driver:
                # 데이터베이스 존재 여부 확인 및 생성
                try:
                    exists = (
                        driver.databases.contains(self.typedb_database)
                        if hasattr(driver.databases, "contains")
                        else self.typedb_database
                        in [db.name for db in driver.databases.all()]
                    )
                except Exception as e:
                    logger.error("TypeDB 데이터베이스 조회 중 오류: %s", e)
                    return False

                if not exists:
                    logger.info("TypeDB 데이터베이스 생성 중: %s", self.typedb_database)
                    driver.databases.create(self.typedb_database)

                # 메인 스키마 적용
                schema_text = schema_path.read_text().strip()
                if schema_text:
                    logger.info("TypeDB 메인 스키마를 적용합니다.")
                    with driver.transaction(
                        self.typedb_database, TransactionType.SCHEMA
                    ) as tx:
                        tx.query(schema_text).resolve()
                        tx.commit()

                # 최소 document 스키마 보장
                self._ensure_minimal_typedb_schema(driver)

            logger.info("TypeDB 초기화 성공")
            return True
        except TypeDBDriverException as e:
            logger.error("TypeDB 드라이버 오류 발생: %s", e)
        except Exception as e:
            logger.exception("TypeDB 초기화 중 예상치 못한 오류 발생: %s", e)

        return False

    def _ensure_minimal_typedb_schema(self, driver) -> None:
        """최소한 document 스키마가 존재하도록 보장한다."""
        try:
            from typedb.driver import TransactionType
        except ImportError:
            return

        minimal_doc_schema = """
define
  attribute doc_id, value string;
  attribute title, value string;

  entity document,
    owns doc_id,
    owns title;
"""
        try:
            with driver.transaction(
                self.typedb_database, TransactionType.SCHEMA
            ) as tx:
                tx.query(minimal_doc_schema).resolve()
                tx.commit()
        except Exception:
            # 이미 정의되어 있는 경우 등은 무시
            return

    def init_qdrant(self) -> bool:
        """Qdrant: 컬렉션이 없으면 생성. 성공 시 True.

        - 기본 컬렉션(self.qdrant_collection): 단일 벡터 설정.
        - 문서용 컬렉션(self.qdrant_doc_collection): config 기반(named vector) + 초기 vector upsert.
        """
        if not self.qdrant_host:
            return False
        try:
            from qdrant_client import QdrantClient
            from qdrant_client.models import Distance, PointStruct, VectorParams
        except ImportError:
            return False
        client = QdrantClient(host=self.qdrant_host, port=self.qdrant_port)

        # 1) 기존 markdown 컬렉션 보장
        try:
            client.get_collection(self.qdrant_collection)
        except Exception:
            client.create_collection(
                collection_name=self.qdrant_collection,
                vectors_config=VectorParams(
                    size=self.qdrant_vector_size,
                    distance=Distance.COSINE,
                ),
            )

        # 2) gopedia_document 컬렉션(named vector "wiki") 생성 + 초기 벡터 upsert
        if self.qdrant_doc_collection:
            try:
                client.get_collection(self.qdrant_doc_collection)
            except Exception:
                vectors_config = {
                    self.qdrant_doc_vector_name: VectorParams(
                        size=self.qdrant_doc_vector_size,
                        distance=Distance.COSINE,
                    )
                }
                client.create_collection(
                    collection_name=self.qdrant_doc_collection,
                    vectors_config=vectors_config,
                )

                # 최초 생성 시, config 기반 차원(self.qdrant_doc_vector_size)으로 seed 벡터 upsert
                seed_vector = [0.0] * self.qdrant_doc_vector_size
                points = [
                    PointStruct(
                        id=1,
                        vector={self.qdrant_doc_vector_name: seed_vector},
                        payload={
                            "source": "tests.initialize",
                            "kind": "seed",
                        },
                    )
                ]
                client.upsert(
                    collection_name=self.qdrant_doc_collection,
                    points=points,
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
