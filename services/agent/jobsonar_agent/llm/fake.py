from __future__ import annotations

import json

from jobsonar_agent.llm import LLM


class FakeLLM:
    """Deterministic deep-dive text. No network. Used by tests and when
    Ollama has no chat model (same role as FakeEmbedder)."""

    def complete(self, prompt: str, **kw) -> str:
        return json.dumps({
            "justification_md": (
                "You already cover the matched skills on this posting. "
                "The composite score is a strong fit on the deterministic first pass; "
                "this note does not change the rank."
            ),
            "tailoring_md": (
                "Lead the summary with the matched skills. "
                "Close gaps listed as job-asks-not-on-profile with one concrete example each. "
                "Do not submit an application from this tool."
            ),
        })


class CountingLLM:
    """Wraps an LLM and counts complete() calls for the NFR-1 cost guard."""

    def __init__(self, inner: LLM):
        self.inner = inner
        self.calls = 0
        self.prompts: list[str] = []

    def complete(self, prompt: str, **kw) -> str:
        self.calls += 1
        self.prompts.append(prompt)
        return self.inner.complete(prompt, **kw)
