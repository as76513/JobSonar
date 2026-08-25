from jobsonar_agent.score.skill_coverage import coverage, extract_job_skills

# Mirrors services/api/internal/score/keyword_test.go -- same semantics,
# same fixtures, so the Go implementation being deleted (Day 6) and the
# Python one replacing it agree on behavior, not just intent.


def test_overlap_ranks_devops_job():
    skills = ["kubernetes", "terraform", "aws", "go", "devops"]
    job_skills = extract_job_skills("Senior DevOps Engineer", "Own kubernetes and terraform on AWS.")
    cov, matched, missing = coverage(skills, job_skills)
    assert len(matched) == 4
    assert missing == []  # go is extra, not a job ask
    assert cov >= 0.99


def test_overlap_job_ask_not_on_resume():
    job_skills = extract_job_skills("Engineer", "kubernetes and kafka")
    cov, matched, missing = coverage(["kubernetes"], job_skills)
    assert matched == ["kubernetes"]
    assert missing == ["kafka"]
    assert 0.49 <= cov <= 0.51


def test_overlap_extra_profile_skills_are_not_missing():
    job_skills = extract_job_skills("Engineer", "Run kubernetes clusters.")
    _, matched, missing = coverage(["kubernetes", "helm", "prometheus", "kafka"], job_skills)
    assert not ({"helm", "prometheus", "kafka"} & set(missing))
    assert len(matched) == 1


def test_overlap_empty_skills():
    job_skills = extract_job_skills("DevOps", "nothing listed")
    cov, matched, _ = coverage([], job_skills)
    assert cov == 0
    assert matched == []


def test_overlap_cicd_phrase():
    job_skills = extract_job_skills("Platform", "CI/CD with docker compose")
    _, matched, _ = coverage(["ci/cd", "docker"], job_skills)
    assert len(matched) == 2


def test_go_does_not_match_golang():
    job_skills = extract_job_skills("SWE", "golang services")
    _, matched, missing = coverage(["go"], job_skills)
    assert matched == []
    assert missing == ["golang"]
