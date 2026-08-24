package score

import "testing"

func TestOverlapRanksDevOpsJob(t *testing.T) {
	skills := []string{"kubernetes", "terraform", "aws", "go", "devops"}
	got := Overlap(skills, "Senior DevOps Engineer",
		"Own kubernetes and terraform on AWS.")
	if len(got.MatchedSkills) != 4 {
		t.Fatalf("matched=%v", got.MatchedSkills)
	}
	if len(got.MissingSkills) != 0 {
		t.Fatalf("missing=%v want none (go is extra, not a job ask)", got.MissingSkills)
	}
	if got.Coverage < 0.99 {
		t.Fatalf("coverage=%v", got.Coverage)
	}
}

func TestOverlapJobAskNotOnResume(t *testing.T) {
	got := Overlap([]string{"kubernetes"}, "Engineer", "kubernetes and kafka")
	if len(got.MatchedSkills) != 1 || got.MatchedSkills[0] != "kubernetes" {
		t.Fatalf("matched=%v", got.MatchedSkills)
	}
	if len(got.MissingSkills) != 1 || got.MissingSkills[0] != "kafka" {
		t.Fatalf("missing=%v", got.MissingSkills)
	}
	if got.Coverage < 0.49 || got.Coverage > 0.51 {
		t.Fatalf("coverage=%v want 0.5", got.Coverage)
	}
}

func TestOverlapExtraProfileSkillsAreNotMissing(t *testing.T) {
	got := Overlap([]string{"kubernetes", "helm", "prometheus", "kafka"},
		"Engineer", "Run kubernetes clusters.")
	for _, s := range got.MissingSkills {
		if s == "helm" || s == "prometheus" || s == "kafka" {
			t.Fatalf("profile extras must not be missing: %v", got.MissingSkills)
		}
	}
	if len(got.MatchedSkills) != 1 {
		t.Fatalf("matched=%v", got.MatchedSkills)
	}
}

func TestOverlapEmptySkills(t *testing.T) {
	got := Overlap(nil, "DevOps", "nothing listed")
	if got.Coverage != 0 || len(got.MatchedSkills) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestOverlapCICDPhrase(t *testing.T) {
	got := Overlap([]string{"ci/cd", "docker"}, "Platform", "CI/CD with docker compose")
	if len(got.MatchedSkills) != 2 {
		t.Fatalf("matched=%v missing=%v", got.MatchedSkills, got.MissingSkills)
	}
}

func TestGoDoesNotMatchGolang(t *testing.T) {
	got := Overlap([]string{"go"}, "SWE", "golang services")
	if len(got.MatchedSkills) != 0 {
		t.Fatalf("go must not match golang: %v", got.MatchedSkills)
	}
	if len(got.MissingSkills) != 1 || got.MissingSkills[0] != "golang" {
		t.Fatalf("missing=%v", got.MissingSkills)
	}
}
