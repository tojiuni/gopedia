"""
Pytest configuration for Gopedia tests. Loads .env from repo root.
"""
from __future__ import annotations

import os
from pathlib import Path

import pytest

_REPO_ROOT = Path(__file__).resolve().parents[1]


def _load_dotenv() -> None:
    try:
        from dotenv import load_dotenv
        load_dotenv(_REPO_ROOT / ".env")
    except ImportError:
        pass


def pytest_configure(config: pytest.Config) -> None:
    _load_dotenv()
    config.addinivalue_line(
        "markers",
        "integration: mark test as integration (requires external services)",
    )


def _env(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


@pytest.fixture(scope="session")
def typedb_host() -> str:
    return _env("TYPEDB_HOST", "localhost")


@pytest.fixture(scope="session")
def typedb_port() -> str:
    return _env("TYPEDB_PORT", "1729")


@pytest.fixture(scope="session")
def typedb_database() -> str:
    return _env("TYPEDB_DATABASE", "gopedia")


@pytest.fixture(scope="session")
def qdrant_host() -> str:
    return _env("QDRANT_HOST", "localhost")


@pytest.fixture(scope="session")
def qdrant_port() -> int:
    return int(_env("QDRANT_PORT", "6333"))


@pytest.fixture(scope="session")
def postgres_host() -> str:
    return _env("POSTGRES_HOST", "")


@pytest.fixture(scope="session")
def postgres_port() -> str:
    return _env("POSTGRES_PORT", "5432")


@pytest.fixture(scope="session")
def postgres_user() -> str:
    return _env("POSTGRES_USER", "")


@pytest.fixture(scope="session")
def postgres_password() -> str:
    return _env("POSTGRES_PASSWORD", "")


@pytest.fixture(scope="session")
def postgres_db() -> str:
    return _env("POSTGRES_DB", "gopedia")


@pytest.fixture(scope="session")
def phloem_grpc_addr() -> str:
    return _env("GOPEDIA_PHLOEM_GRPC_ADDR", "localhost:50051")
