"""Local embedding service — intfloat/multilingual-e5-large.

Passages (ingestion): prefix "passage"
Queries  (retrieval): prefix "query"

POST /embed
  Body: {"texts": ["..."], "prefix": "passage"}
  Response: {"embeddings": [[...], ...], "vector_size": 1024}

GET /health
  Response: {"status": "ok", "model": "...", "vector_size": 1024}
"""

from __future__ import annotations

import os
import logging
from typing import List

from fastapi import FastAPI
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

MODEL_NAME = os.environ.get("EMBEDDING_MODEL", "intfloat/multilingual-e5-large")

logger.info("Loading embedding model: %s", MODEL_NAME)
_model = SentenceTransformer(MODEL_NAME)
_vector_size: int = _model.get_sentence_embedding_dimension()
logger.info("Model loaded. vector_size=%d", _vector_size)

app = FastAPI(title="gopedia-embedding-service")


class EmbedRequest(BaseModel):
    texts: List[str]
    prefix: str = "passage"  # "passage" for documents, "query" for queries


class EmbedResponse(BaseModel):
    embeddings: List[List[float]]
    vector_size: int


@app.post("/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest) -> EmbedResponse:
    prefixed = [f"{req.prefix}: {t}" for t in req.texts]
    vecs = _model.encode(prefixed, normalize_embeddings=True).tolist()
    return EmbedResponse(embeddings=vecs, vector_size=_vector_size)


@app.get("/health")
def health() -> dict:
    return {"status": "ok", "model": MODEL_NAME, "vector_size": _vector_size}
