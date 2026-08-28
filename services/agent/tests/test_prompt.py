from jobsonar_agent.graph.prompt import build_prompt, parse_analysis
from jobsonar_agent.llm.fake import FakeLLM


def test_fake_llm_is_parseable():
    raw = FakeLLM().complete("ignored")
    parsed = parse_analysis(raw)
    assert parsed is not None
    just, tail = parsed
    assert "matched skills" in just.lower() or "fit" in just.lower()
    assert tail


def test_parse_json_fence():
    raw = """Here you go
```json
{"justification_md": "fit", "tailoring_md": "close the kafka gap"}
```
"""
    assert parse_analysis(raw) == ("fit", "close the kafka gap")


def test_parse_heading_fallback():
    raw = """## Why you fit
Kubernetes on the resume.

## What to close
Add Helm examples.
"""
    just, tail = parse_analysis(raw)
    assert "Kubernetes" in just
    assert "Helm" in tail


def test_parse_empty():
    assert parse_analysis("") is None
    assert parse_analysis("not json and no headings") is None


def test_prompt_has_no_resume_blob_and_caps_description():
    profile = {
        "skills": ["kubernetes"],
        "seniority": "senior",
        "location": "Pune",
        "remote_pref": "hybrid",
    }
    job = {
        "title": "DevOps",
        "company": "Acme",
        "matched_skills": ["kubernetes"],
        "missing_skills": ["kafka"],
        "description_md": "x" * 8000,
    }
    prompt = build_prompt(profile, job)
    assert "raw resume" not in prompt.lower()
    assert "kubernetes" in prompt
    assert "kafka" in prompt
    assert "x" * 4000 in prompt
    assert "x" * 4001 not in prompt
    assert "Do not apply" in prompt
