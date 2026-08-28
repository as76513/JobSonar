import os

import pytest

from jobsonar_agent.llm import LLM
from jobsonar_agent.llm.factory import resolve_llm
from jobsonar_agent.llm.fake import FakeLLM


def test_fake_llm_protocol():
    llm: LLM = FakeLLM()
    text = llm.complete("hello")
    assert "justification_md" in text


def test_resolve_fake_default(monkeypatch):
    monkeypatch.setattr("jobsonar_agent.config.DEEP_DIVE_BACKEND", "fake")
    assert isinstance(resolve_llm(), FakeLLM)


def test_bedrock_fail_closed_without_opt_in(monkeypatch):
    monkeypatch.setattr("jobsonar_agent.config.DEEP_DIVE_BACKEND", "bedrock")
    monkeypatch.setattr("jobsonar_agent.config.DEEP_DIVE_OPT_IN", False)
    assert resolve_llm() is None


@pytest.mark.skipif(not os.environ.get("RUN_LLM_TEST"), reason="live ollama")
def test_ollama_complete_smoke():
    from jobsonar_agent.llm.ollama import OllamaLLM

    text = OllamaLLM().complete("Reply with the word ok.")
    assert isinstance(text, str) and text.strip()
