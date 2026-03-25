from __future__ import annotations

import os
import re
from concurrent import futures

import grpc

from core.proto.gen.python import nlp_worker_pb2, nlp_worker_pb2_grpc


_SPLIT_RE = re.compile(r"[.!?]+")


def _split_sentences(text: str) -> list[str]:
    parts = _SPLIT_RE.split(text.replace("\r\n", "\n").replace("\r", "\n"))
    out: list[str] = []
    for p in parts:
        p = p.strip()
        if p:
            out.append(p)
    return out


class NLPWorkerServicer(nlp_worker_pb2_grpc.NLPWorkerServicer):
    def ProcessL2(self, request: nlp_worker_pb2.NLPRequest, context: grpc.ServicerContext):
        # Minimal v1: English-only sentence split; entity extraction added later.
        sents = _split_sentences(request.text or "")
        return nlp_worker_pb2.NLPResponse(sentences=sents, entities=[])


def serve() -> None:
    addr = os.environ.get("GOPEDIA_NLP_WORKER_ADDR", "0.0.0.0:50052")
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    nlp_worker_pb2_grpc.add_NLPWorkerServicer_to_server(NLPWorkerServicer(), server)
    server.add_insecure_port(addr)
    server.start()
    server.wait_for_termination()


if __name__ == "__main__":
    serve()

