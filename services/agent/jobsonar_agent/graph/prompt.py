"""Deep-dive prompt + parser. Inputs are derived profile fields and the
job posting — never raw resume text (golden rule 5). Do not log the
prompt body.
"""

from __future__ import annotations

import json
import re

from jobsonar_agent import config

_FENCE = re.compile(r"```(?:json)?\s*(.*?)\s*```", re.DOTALL | re.IGNORECASE)


def build_prompt(profile: dict, job: dict) -> str:
    desc = (job.get("description_md") or "")[: config.DEEP_DIVE_DESC_CHARS]
    skills = ", ".join(profile.get("skills") or []) or "(none)"
    matched = ", ".join(job.get("matched_skills") or []) or "(none)"
    missing = ", ".join(job.get("missing_skills") or []) or "(none)"
    return (
        "You help a job seeker understand a posting. Do not apply for them.\n"
        "Never invent skills that are not in the profile list.\n\n"
        "Profile (derived fields only — not a resume):\n"
        f"- skills: {skills}\n"
        f"- seniority: {profile.get('seniority') or '(unset)'}\n"
        f"- location: {profile.get('location') or '(unset)'}\n"
        f"- remote preference: {profile.get('remote_pref') or '(unset)'}\n\n"
        "Job:\n"
        f"- title: {job.get('title') or ''}\n"
        f"- company: {job.get('company') or ''}\n"
        f"- matched skills: {matched}\n"
        f"- job asks, not on profile: {missing}\n"
        f"- description:\n{desc}\n\n"
        "Reply with JSON only, no other prose:\n"
        '{"justification_md": "markdown: why they fit",'
        ' "tailoring_md": "markdown: what to close / how to tailor"}\n'
    )


def parse_analysis(text: str) -> tuple[str, str] | None:
    if not text or not text.strip():
        return None
    raw = text.strip()
    m = _FENCE.search(raw)
    if m:
        raw = m.group(1).strip()
    data = _try_json(raw)
    if data is None:
        data = _try_json(_first_object(raw))
    if isinstance(data, dict):
        just = str(data.get("justification_md") or data.get("justification") or "").strip()
        tail = str(data.get("tailoring_md") or data.get("tailoring") or "").strip()
        if just or tail:
            return just, tail
    just = _section(text, ("why you fit", "justification"))
    tail = _section(text, ("what to close", "tailoring", "how to tailor"))
    if just or tail:
        return just, tail
    return None


def _try_json(s: str | None) -> dict | None:
    if not s:
        return None
    try:
        val = json.loads(s)
    except json.JSONDecodeError:
        return None
    return val if isinstance(val, dict) else None


def _first_object(s: str) -> str | None:
    start, end = s.find("{"), s.rfind("}")
    if start == -1 or end <= start:
        return None
    return s[start : end + 1]


def _section(text: str, titles: tuple[str, ...]) -> str:
    lower = text.lower()
    for title in titles:
        idx = lower.find(title)
        if idx == -1:
            continue
        start = text.find("\n", idx)
        if start == -1:
            continue
        rest = text[start + 1 :]
        nxt = re.search(r"\n#{1,3}\s+", rest)
        body = rest[: nxt.start()] if nxt else rest
        return body.strip()
    return ""
