"""
DBInitializer 클래스 테스트: init_postgres, init_typedb, init_qdrant, init_all.
env가 설정된 경우에만 실행되며, 없으면 skip.
"""
from __future__ import annotations

import pytest

from tests.initialize import DBInitializer


def _env_set(*keys: str) -> bool:
    import os
    return any(os.environ.get(k) for k in keys)


@pytest.mark.integration
def test_db_initializer_init_all_skip_when_no_env() -> None:
    """env가 하나도 없으면 init_all은 빈 dict 반환 (skip_missing=True)."""
    init = DBInitializer()
    init.postgres_host = ""
    init.typedb_host = ""
    init.qdrant_host = ""
    result = init.init_all(skip_missing=True)
    assert result == {}


@pytest.mark.integration
@pytest.mark.skipif(
    not _env_set("POSTGRES_HOST", "QDRANT_HOST", "TYPEDB_HOST"),
    reason="POSTGRES_HOST and/or QDRANT_HOST and/or TYPEDB_HOST not set",
)
def test_db_initializer_init_all() -> None:
    """env가 있으면 init_all 실행 후 각 DB별 성공 여부 반환."""
    init = DBInitializer()
    result = init.init_all(skip_missing=True)
    assert isinstance(result, dict)
    for k in result:
        assert k in ("postgres", "typedb", "qdrant")
        assert isinstance(result[k], bool)


@pytest.mark.integration
@pytest.mark.skipif(not _env_set("POSTGRES_HOST", "POSTGRES_USER"), reason="POSTGRES_* not set")
def test_db_initializer_init_postgres() -> None:
    """init_postgres 호출 시 documents 테이블 생성 또는 이미 존재."""
    init = DBInitializer()
    ok = init.init_postgres()
    assert ok is True


@pytest.mark.integration
@pytest.mark.skipif(not _env_set("TYPEDB_HOST"), reason="TYPEDB_HOST not set")
def test_db_initializer_init_typedb() -> None:
    """init_typedb 호출 시 DB 및 스키마 생성 또는 이미 존재."""
    try:
        from typedb.driver import TypeDB  # noqa: F401
    except ImportError:
        pytest.skip("typedb-driver not installed")
    init = DBInitializer()
    ok = init.init_typedb()
    assert ok is True


@pytest.mark.integration
@pytest.mark.skipif(not _env_set("QDRANT_HOST"), reason="QDRANT_HOST not set")
def test_db_initializer_init_qdrant() -> None:
    """init_qdrant 호출 시 컬렉션 생성 또는 이미 존재."""
    try:
        from qdrant_client import QdrantClient  # noqa: F401
    except ImportError:
        pytest.skip("qdrant-client not installed")
    init = DBInitializer()
    ok = init.init_qdrant()
    assert ok is True
