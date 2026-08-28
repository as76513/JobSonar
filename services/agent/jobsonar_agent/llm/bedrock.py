"""Thin Bedrock adapter. AWS SDK stays in this file (NFR-2 portability).
Never import this from the graph unless resolve_llm() selected bedrock
*and* DEEP_DIVE_OPT_IN=1. No keys in the repo.
"""

from __future__ import annotations

from jobsonar_agent import config


class BedrockLLM:
    def complete(self, prompt: str, **kw) -> str:
        import boto3  # optional; only when opted in

        client = boto3.client("bedrock-runtime", region_name=config.AWS_REGION)
        resp = client.converse(
            modelId=config.BEDROCK_MODEL,
            messages=[{"role": "user", "content": [{"text": prompt}]}],
        )
        blocks = ((resp.get("output") or {}).get("message") or {}).get("content") or []
        texts = [b.get("text") or "" for b in blocks if isinstance(b, dict)]
        text = "\n".join(t for t in texts if t).strip()
        if not text:
            raise RuntimeError("bedrock converse returned empty text")
        return text
