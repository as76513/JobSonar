from pathlib import Path

from jobsonar_agent.resume.parse import extract_skills, parse_resume

FIXTURE = Path(__file__).parent / "fixtures" / "resume.txt"


def test_fixture_extracts_expected_skills():
    parsed = parse_resume(FIXTURE)
    skills = {s.lower() for s in parsed["skills"]}
    for need in ("kubernetes", "terraform", "aws", "docker", "python", "go"):
        assert need in skills
    assert parsed["char_count"] > 0


def test_go_does_not_match_golang_only_when_absent():
    assert "go" not in extract_skills("I write golang services on kubernetes")
    assert "go" in extract_skills("I write go and kubernetes")


def test_cicd_phrase():
    skills = extract_skills("Built ci/cd pipelines on GitHub Actions")
    assert "ci/cd" in skills
    assert "github actions" in skills
