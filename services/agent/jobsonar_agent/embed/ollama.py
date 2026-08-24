from __future__ import annotations

import json
import urllib.error
import urllib.request

from jobsonar_agent import config


class OllamaEmbedder:
    def __init__(self, host: str | None = None, model: str | None = None, dim: int | None = None):
        self.host = (host or config.OLLAMA_HOST).rstrip("/")
        self.model = model or config.EMBED_MODEL
        self.dim = dim or config.EMBED_DIM

    def embed(self, texts: list[str]) -> list[list[float]]:
        if not texts:
            return []
        payload = json.dumps({"model": self.model, "input": texts}).encode()
        req = urllib.request.Request(
            self.host + "/api/embed",
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=120) as resp:
                body = json.loads(resp.read().decode())
        except urllib.error.HTTPError:
            return self._embed_legacy(texts)
        vectors = body.get("embeddings") or []
        if len(vectors) != len(texts):
            raise RuntimeError("ollama embed count mismatch")
        for v in vectors:
            if len(v) != self.dim:
                raise RuntimeError(f"expected dim {self.dim}, got {len(v)}")
        return vectors

    def _embed_legacy(self, texts: list[str]) -> list[list[float]]:
        out = []
        for text in texts:
            payload = json.dumps({"model": self.model, "prompt": text}).encode()
            req = urllib.request.Request(
                self.host + "/api/embeddings",
                data=payload,
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=120) as resp:
                body = json.loads(resp.read().decode())
            v = body.get("embedding") or []
            if len(v) != self.dim:
                raise RuntimeError(f"expected dim {self.dim}, got {len(v)}")
            out.append(v)
        return out
