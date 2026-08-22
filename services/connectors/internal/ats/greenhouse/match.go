package greenhouse

import (
	"encoding/json"
	"strings"

	"github.com/as76513/JobSonar/services/connectors/internal/connector"
)

// ops/infra family: a "DevOps" query should also keep SRE / platform /
// infrastructure titles. Generic words (engineer, lead) never match alone.
var opsInfraSynonyms = []string{
	"devsecops", "site reliability", "dev-ops", "dev ops",
	"devops", "sre", "platform", "infrastructure", "infra", "reliability",
}

var securitySynonyms = []string{
	"devsecops", "appsec", "application security", "security",
}

func filterJobs(jobs []connector.RawJob, query, where string) []connector.RawJob {
	wheres := connector.ParseList(where)
	roleActive := strings.TrimSpace(query) != ""
	if !roleActive && len(wheres) == 0 {
		return jobs
	}
	kept := jobs[:0]
	for _, raw := range jobs {
		ad, ok := envelopePost(raw.Payload)
		if !ok {
			continue
		}
		if roleActive && !roleMatches(ad, query) {
			continue
		}
		if !locationMatches(ad, wheres) {
			continue
		}
		kept = append(kept, raw)
	}
	return kept
}

func roleMatches(ad post, query string) bool {
	hay := roleHaystack(ad)
	for _, phrase := range connector.ParseList(query) {
		if familyMatch(hay, phrase) {
			return true
		}
		if !phraseHasFamily(phrase) && titleMatches(hay, roleTokens(phrase)) {
			return true
		}
	}
	return false
}

func roleHaystack(ad post) string {
	parts := []string{ad.Title}
	for _, d := range ad.Departments {
		parts = append(parts, d.Name)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func phraseHasFamily(phrase string) bool {
	return len(synonymsForPhrase(phrase)) > 0
}

func familyMatch(hay, phrase string) bool {
	for _, syn := range synonymsForPhrase(phrase) {
		if strings.Contains(hay, syn) {
			return true
		}
	}
	return false
}

func synonymsForPhrase(phrase string) []string {
	p := strings.ToLower(phrase)
	var out []string
	if hasAny(p, "devops", "devsecops", "sre", "site reliability", "platform", "infrastructure", "infra") {
		out = append(out, opsInfraSynonyms...)
	}
	if hasAny(p, "security", "appsec", "devsecops") {
		out = append(out, securitySynonyms...)
	}
	return out
}

func hasAny(hay string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(hay, n) {
			return true
		}
	}
	return false
}

func locationMatches(ad post, wheres []string) bool {
	if len(wheres) == 0 {
		return true
	}
	blob := strings.ToLower(ad.Location.Name + " " + ad.Title)
	for _, o := range ad.Offices {
		blob += " " + strings.ToLower(o.Name+" "+o.Location)
	}
	for _, w := range wheres {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		if strings.Contains(blob, w) {
			return true
		}
	}
	return false
}

func envelopePost(payload json.RawMessage) (post, bool) {
	var env rawJob
	if err := json.Unmarshal(payload, &env); err != nil {
		return post{}, false
	}
	var ad post
	if err := json.Unmarshal(env.Job, &ad); err != nil {
		return post{}, false
	}
	return ad, true
}

func roleTokens(query string) []string {
	var out []string
	for _, t := range strings.Fields(strings.ToLower(query)) {
		t = strings.Trim(t, ",.;:()[]/\\\"'")
		if len(t) < 2 {
			continue
		}
		out = append(out, t)
	}
	return out
}

func titleMatches(hay string, tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	hay = strings.ToLower(hay)
	for _, t := range tokens {
		if !strings.Contains(hay, t) {
			return false
		}
	}
	return true
}
