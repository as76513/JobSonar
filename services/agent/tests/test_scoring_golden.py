"""Golden scoring test (Week 6), per docs/TRD.md §8 and the scoring-model
skill's mandatory rule: a fixed resume + fixed job -> expected sub-scores
within tolerance. Exercises the full pipeline as pure functions (no DB --
that wiring is run.score_jobs, covered by test_store_gates.py); update the
expected numbers here whenever a sub-score's math changes.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

from jobsonar_agent.score.composite import band, composite
from jobsonar_agent.score.location import fit as location_fit
from jobsonar_agent.score.recency import fit as recency_fit
from jobsonar_agent.score.seniority import fit as seniority_fit
from jobsonar_agent.score.skill_coverage import coverage, extract_job_skills

# Fixed resume: parsed skill list a real upload would produce.
PROFILE_SKILLS = ["kubernetes", "terraform", "aws", "docker", "linux"]
PROFILE_SENIORITY = "senior"
PROFILE_LOCATION = "Pune"
PROFILE_REMOTE_PREF = None

# Fixed job posting.
JOB_TITLE = "Senior DevOps Engineer"
JOB_DESCRIPTION = "Own kubernetes and terraform on AWS. Docker and linux experience required. Kafka is a plus."
JOB_LOCATION = "Pune, Maharashtra"
JOB_REMOTE_TYPE = ""

NOW = datetime(2026, 1, 1, tzinfo=timezone.utc)
JOB_POSTED_AT = NOW - timedelta(days=30)  # half the 60-day recency window
FIXED_SEMANTIC = 0.8  # stands in for a real embedding cosine


def test_golden_resume_job_pair_within_tolerance():
    job_skills = extract_job_skills(JOB_TITLE, JOB_DESCRIPTION)
    assert job_skills == ["kubernetes", "terraform", "aws", "docker", "devops", "linux", "kafka"]

    skill_cov, matched, missing = coverage(PROFILE_SKILLS, job_skills)
    assert matched == ["kubernetes", "terraform", "aws", "docker", "linux"]
    assert missing == ["devops", "kafka"]
    assert abs(skill_cov - 5 / 7) < 0.01

    sen_fit = seniority_fit(PROFILE_SENIORITY, JOB_TITLE)
    assert sen_fit == 1.0  # "senior" profile, "Senior ..." title -> exact band match

    loc_fit = location_fit(PROFILE_LOCATION, PROFILE_REMOTE_PREF, JOB_LOCATION, JOB_REMOTE_TYPE)
    assert loc_fit == 1.0  # "Pune" is a substring of "Pune, Maharashtra"

    rec = recency_fit(JOB_POSTED_AT, None, now=NOW)
    assert abs(rec - 0.5) < 0.01  # halfway through the 60-day window

    comp = composite(skill_cov, FIXED_SEMANTIC, sen_fit, loc_fit, rec)
    # 0.40*0.7143 + 0.20*0.8 + 0.15*1.0 + 0.15*1.0 + 0.10*0.5 = 0.7957
    assert abs(comp - 0.7957) < 0.01
    assert band(comp) == "strong"
