# Optional Pydantic schemas mirroring proto for Python clients (e.g. Root, Verify).
# For gRPC, use the generated gen/python/ stubs instead.
from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


class IngestRequestSchema(BaseModel):
    """Mirrors IngestRequest from rhizome.proto."""
    title: str = ""
    content: str = ""
    source_metadata: dict[str, str] = Field(default_factory=dict)


class IngestResponseSchema(BaseModel):
    """Mirrors IngestResponse from rhizome.proto."""
    machine_id: int = 0
    doc_id: str = ""
    ok: bool = False
    error_message: str = ""
