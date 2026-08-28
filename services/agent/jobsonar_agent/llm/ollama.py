from __future__ import annotations

import json
import urllib.error
import urllib.request

from jobsonar_agent import config


class OllamaLLM:
    def __init__(self, host: str | None = None, model: str | None = None):
        self.host = (host or config.OLLAMA_HOST).rstrip("/")
        self.model = model or config.LLM_MODEL

    def complete(self, prompt: str, **kw) -> str:
        payload = json.dumps({
            "model": self.model,
            "prompt": prompt,
            "stream": False,
        }).encode()
        req = urllib.request.Request(
            self.host + "/api/generate",
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=120) as resp:
                body = json.loads(resp.read().decode())
        except urllib.error.HTTPError:
            return self._chat(prompt)
        text = body.get("response") or ""
        if not text:
            raise RuntimeError("ollama generate returned empty response")
        return text

    def _chat(self, prompt: str) -> str:
        payload = json.dumps({
            "model": self.model,
            "messages": [{"role": "user", "content": prompt}],
            "stream": False,
        }).encode()
        req = urllib.request.Request(
            self.host + "/api/chat",
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=120) as resp:
            body = json.loads(resp.read().decode())
        msg = body.get("message") or {}
        text = msg.get("content") or body.get("response") or ""
        if not text:
            raise RuntimeError("ollama chat returned empty response")
        return text
