import argparse
import os
from typing import Any, List, Optional, Sequence, Union

import torch
import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field


def _device() -> str:
    forced = os.getenv("MODEL_DEVICE", "").strip()
    if forced:
        return forced
    return "cuda" if torch.cuda.is_available() else "cpu"


def _as_texts(value: Union[str, List[str]]) -> List[str]:
    if isinstance(value, str):
        return [value]
    return value


def _normalize_rows(rows: Any) -> List[List[float]]:
    if hasattr(rows, "tolist"):
        rows = rows.tolist()
    return [[float(x) for x in row] for row in rows]


class EmbeddingRequest(BaseModel):
    model: str
    input: Union[str, List[str]]
    encoding_format: Optional[str] = "float"
    dimensions: Optional[int] = None
    truncate_prompt_tokens: Optional[int] = None


class RerankRequest(BaseModel):
    model: Optional[str] = None
    query: str
    documents: List[str] = Field(default_factory=list)
    top_n: Optional[int] = None


class ModelService:
    def __init__(self, task: str, model_name: str):
        self.task = task
        self.model_name = model_name
        self.device = _device()
        self.embedder = None
        self.reranker = None

    def load(self) -> None:
        if self.task == "embedding":
            from sentence_transformers import SentenceTransformer

            self.embedder = SentenceTransformer(
                self.model_name,
                device=self.device,
                trust_remote_code=True,
            )
            if os.getenv("MODEL_HALF", "true").lower() in {"1", "true", "yes"}:
                try:
                    self.embedder.half()
                except Exception:
                    pass
            return

        if self.task == "rerank":
            from FlagEmbedding import FlagReranker

            use_fp16 = self.device.startswith("cuda") and os.getenv("MODEL_HALF", "true").lower() in {
                "1",
                "true",
                "yes",
            }
            self.reranker = FlagReranker(self.model_name, use_fp16=use_fp16)
            return

        raise ValueError(f"unsupported task: {self.task}")

    def embed(self, texts: Sequence[str]) -> List[List[float]]:
        if self.embedder is None:
            raise RuntimeError("embedding model is not loaded")

        batch_size = int(os.getenv("EMBEDDING_BATCH_SIZE", "8"))
        normalize = os.getenv("EMBEDDING_NORMALIZE", "true").lower() in {"1", "true", "yes"}
        prompt_name = os.getenv("EMBEDDING_PROMPT_NAME", "").strip() or None
        kwargs = {
            "batch_size": batch_size,
            "normalize_embeddings": normalize,
            "show_progress_bar": False,
        }
        if prompt_name:
            kwargs["prompt_name"] = prompt_name

        rows = self.embedder.encode(list(texts), **kwargs)
        return _normalize_rows(rows)

    def rerank(self, query: str, documents: Sequence[str]) -> List[float]:
        if self.reranker is None:
            raise RuntimeError("rerank model is not loaded")
        if not documents:
            return []

        pairs = [[query, doc] for doc in documents]
        batch_size = int(os.getenv("RERANK_BATCH_SIZE", "8"))
        normalize = os.getenv("RERANK_NORMALIZE", "true").lower() in {"1", "true", "yes"}
        scores = self.reranker.compute_score(pairs, batch_size=batch_size, normalize=normalize)
        if isinstance(scores, (float, int)):
            return [float(scores)]
        if hasattr(scores, "tolist"):
            scores = scores.tolist()
        return [float(score) for score in scores]


def create_app(service: ModelService) -> FastAPI:
    app = FastAPI(title=f"EggKidNotebook {service.task} model service")

    @app.get("/health")
    def health() -> dict:
        return {
            "status": "ok",
            "task": service.task,
            "model": service.model_name,
            "device": service.device,
            "cuda_available": torch.cuda.is_available(),
        }

    @app.get("/v1/models")
    def models() -> dict:
        return {"data": [{"id": service.model_name, "object": "model"}]}

    @app.post("/v1/embeddings")
    @app.post("/embeddings")
    def embeddings(req: EmbeddingRequest) -> dict:
        if service.task != "embedding":
            raise HTTPException(status_code=404, detail="embedding endpoint is disabled")

        texts = _as_texts(req.input)
        vectors = service.embed(texts)
        return {
            "object": "list",
            "model": req.model or service.model_name,
            "data": [
                {"object": "embedding", "index": i, "embedding": vector}
                for i, vector in enumerate(vectors)
            ],
            "usage": {"prompt_tokens": 0, "total_tokens": 0},
        }

    @app.post("/v1/rerank")
    @app.post("/rerank")
    def rerank(req: RerankRequest) -> dict:
        if service.task != "rerank":
            raise HTTPException(status_code=404, detail="rerank endpoint is disabled")

        scores = service.rerank(req.query, req.documents)
        results = [
            {
                "index": i,
                "document": {"text": req.documents[i]},
                "relevance_score": score,
            }
            for i, score in sorted(enumerate(scores), key=lambda item: item[1], reverse=True)
        ]
        if req.top_n is not None and req.top_n > 0:
            results = results[: req.top_n]
        return {
            "id": "eggnb-rerank",
            "model": req.model or service.model_name,
            "results": results,
            "usage": {"total_tokens": 0},
        }

    return app


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--task", choices=["embedding", "rerank"], required=True)
    parser.add_argument("--model", required=True)
    parser.add_argument("--host", default=os.getenv("MODEL_HOST", "127.0.0.1"))
    parser.add_argument("--port", type=int, default=int(os.getenv("MODEL_PORT", "8000")))
    args = parser.parse_args()

    service = ModelService(task=args.task, model_name=args.model)
    service.load()
    app = create_app(service)
    uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
