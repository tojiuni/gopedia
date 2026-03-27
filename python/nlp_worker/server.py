from __future__ import annotations

import os
import pathlib
import sys
from concurrent import futures

# Repo root (for core.proto) and this dir (for mask/split_text) when launched as a script.
_nlp_dir = pathlib.Path(__file__).resolve().parent
_repo_root = _nlp_dir.parent.parent
for p in (_repo_root, _nlp_dir):
    s = str(p)
    if s not in sys.path:
        sys.path.insert(0, s)

import grpc

from core.proto.gen.python import nlp_worker_pb2, nlp_worker_pb2_grpc

from mask import mask_for_sentence_split, unmask_sentences
from split_text import split_sentences_language_aware


class NLPWorkerServicer(nlp_worker_pb2_grpc.NLPWorkerServicer):
    def ProcessL2(self, request: nlp_worker_pb2.NLPRequest, context: grpc.ServicerContext):
        raw = request.text or ""
        masked, replacements = mask_for_sentence_split(raw)
        sents_masked = split_sentences_language_aware(masked)
        sents = unmask_sentences(sents_masked, replacements)
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
