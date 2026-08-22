package connector

import "strings"

// ParseList splits a comma-separated env/flag value and drops empties.
// "DevOps Lead, DevSecOps Lead" → ["DevOps Lead", "DevSecOps Lead"].
func ParseList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type region struct {
	Country string
	Where   string
}

// ExpandSearches turns comma-separated Query / Country / Where into one
// SearchParams per role×region. Aggregators (Adzuna, Jooble) call the API
// once per row; ATS connectors should keep the original Query string and
// OR-match phrases themselves so they do not refetch the board.
//
// Region pairing:
//   - N countries and N wheres → zip (in+Pune, nl+Amsterdam)
//   - 1 country and N wheres  → that country, each city
//   - N countries and 0–1 where → each country with that where (or none)
func ExpandSearches(q SearchParams) []SearchParams {
	roles := ParseList(q.Query)
	if len(roles) == 0 {
		roles = []string{strings.TrimSpace(q.Query)}
	}
	regions := expandRegions(ParseList(q.Country), ParseList(q.Where), q.Country, q.Where)
	out := make([]SearchParams, 0, len(roles)*len(regions))
	for _, role := range roles {
		for _, r := range regions {
			out = append(out, SearchParams{
				Query:   role,
				Where:   r.Where,
				Country: r.Country,
				Page:    q.Page,
				PerPage: q.PerPage,
			})
		}
	}
	return out
}

func expandRegions(countries, wheres []string, rawCountry, rawWhere string) []region {
	if len(countries) == 0 {
		c := strings.TrimSpace(rawCountry)
		if c != "" {
			countries = []string{c}
		} else {
			countries = []string{""}
		}
	}
	switch {
	case len(wheres) == 0:
		out := make([]region, 0, len(countries))
		for _, c := range countries {
			out = append(out, region{Country: c})
		}
		return out
	case len(countries) == 1:
		out := make([]region, 0, len(wheres))
		for _, w := range wheres {
			out = append(out, region{Country: countries[0], Where: w})
		}
		return out
	case len(wheres) == len(countries):
		out := make([]region, 0, len(countries))
		for i, c := range countries {
			out = append(out, region{Country: c, Where: wheres[i]})
		}
		return out
	default:
		w := strings.TrimSpace(rawWhere)
		out := make([]region, 0, len(countries))
		for _, c := range countries {
			out = append(out, region{Country: c, Where: w})
		}
		return out
	}
}
