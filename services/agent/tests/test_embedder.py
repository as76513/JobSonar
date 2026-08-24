import os

import pytest

from jobsonar_agent.embed.fake import FakeEmbedder
from jobsonar_agent.llm import Embedder


def test_fake_embedder_is_protocol_and_768d():
    emb: Embedder = FakeEmbedder()
    vecs = emb.embed(["kubernetes terraform aws", "sales quota crm"])
    assert len(vecs) == 2
    assert len(vecs[0]) == 768
    assert len(vecs[1]) == 768
    # similar text should be closer to itself than to an unrelated string
    a, b = vecs
    self_dot = sum(x * x for x in a)
    cross = sum(x * y for x, y in zip(a, b))
    assert self_dot > cross
    devops, sales = FakeEmbedder().embed([
        "kubernetes terraform aws docker devops",
        "sales quota crm pipeline",
    ])
    profile = FakeEmbedder().embed(["kubernetes terraform aws docker devops"])[0]
    close = sum(x * y for x, y in zip(profile, devops))
    far = sum(x * y for x, y in zip(profile, sales))
    assert close > far


@pytest.mark.skipif(not os.environ.get("RUN_OLLAMA_EMBED_TEST"), reason="live ollama")
def test_ollama_nomic_dim():
    from jobsonar_agent.embed.ollama import OllamaEmbedder

    vecs = OllamaEmbedder().embed(["kubernetes platform engineer"])
    assert len(vecs) == 1
    assert len(vecs[0]) == 768
