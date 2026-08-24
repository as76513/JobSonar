package score

import (
	"strings"
	"unicode"
)

// Keyword is skill overlap of the job posting against the profile.
// Missing = asked by the job, not on the resume. Extra profile skills
// that this posting does not mention are not treated as gaps.
type Keyword struct {
	Coverage      float64  `json:"coverage"`
	Semantic      *float64 `json:"semantic,omitempty"`
	MatchedSkills []string `json:"matched_skills"`
	MissingSkills []string `json:"missing_skills"`
}

func Extract(text string) []string {
	hay := " " + normalize(text) + " "
	var found []string
	for _, skill := range Lexicon {
		needle := " " + normalize(skill) + " "
		if needle == "  " {
			continue
		}
		if strings.Contains(hay, needle) {
			found = append(found, skill)
		}
	}
	return found
}

func Overlap(profile []string, title, description string) Keyword {
	have := map[string]string{}
	for _, raw := range profile {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		have[strings.ToLower(s)] = s
	}
	jobSkills := Extract(title + " " + description)
	var matched, missing []string
	for _, js := range jobSkills {
		if orig, ok := have[strings.ToLower(js)]; ok {
			matched = append(matched, orig)
		} else {
			missing = append(missing, js)
		}
	}
	out := Keyword{MatchedSkills: matched, MissingSkills: missing}
	if n := len(jobSkills); n > 0 {
		out.Coverage = float64(len(matched)) / float64(n)
	}
	if out.MatchedSkills == nil {
		out.MatchedSkills = []string{}
	}
	if out.MissingSkills == nil {
		out.MissingSkills = []string{}
	}
	return out
}

func normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "ci / cd", "ci/cd")
	s = strings.ReplaceAll(s, "ci-cd", "ci/cd")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '#' || r == '/' {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
