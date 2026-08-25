"""Skill coverage sub-score (Week 6). Mirrors the semantics of the Go
score.Overlap it replaces (services/api/internal/score/keyword.go, deleted
once the API reads from `scores`): missing = asked by the job, not on the
resume; extra profile skills the job doesn't mention are not gaps.

Job-side extraction reuses jobsonar_agent.resume.parse.extract_skills
(same lexicon Week 5 already uses for resumes) rather than a separate
lexicon -- one skill list, one extraction function, for both sides.
"""

from __future__ import annotations

from jobsonar_agent.resume.parse import extract_skills


def extract_job_skills(title: str, description_md: str) -> list[str]:
    return extract_skills(f"{title} {description_md}")


def coverage(profile_skills: list[str], job_skills: list[str]) -> tuple[float, list[str], list[str]]:
    """Returns (skill_cov, matched_skills, missing_skills)."""
    have = {s.strip().lower(): s.strip() for s in profile_skills if s.strip()}
    matched: list[str] = []
    missing: list[str] = []
    for js in job_skills:
        orig = have.get(js.lower())
        if orig:
            matched.append(orig)
        else:
            missing.append(js)
    skill_cov = len(matched) / len(job_skills) if job_skills else 0.0
    return skill_cov, matched, missing
