package connector

import (
	"reflect"
	"testing"
)

func TestParseList(t *testing.T) {
	got := ParseList("DevOps Lead, DevOps Architect, DevSecOps Lead")
	want := []string{"DevOps Lead", "DevOps Architect", "DevSecOps Lead"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
	if ParseList("  ") != nil {
		t.Fatal("blank should be empty")
	}
}

func TestExpandSearchesZipRegions(t *testing.T) {
	got := ExpandSearches(SearchParams{
		Query:   "DevOps Lead,DevSecOps Lead",
		Country: "in,nl",
		Where:   "Pune,Amsterdam",
		Page:    1,
		PerPage: 20,
	})
	if len(got) != 4 {
		t.Fatalf("len=%d want 4: %#v", len(got), got)
	}
	want := []SearchParams{
		{Query: "DevOps Lead", Country: "in", Where: "Pune", Page: 1, PerPage: 20},
		{Query: "DevOps Lead", Country: "nl", Where: "Amsterdam", Page: 1, PerPage: 20},
		{Query: "DevSecOps Lead", Country: "in", Where: "Pune", Page: 1, PerPage: 20},
		{Query: "DevSecOps Lead", Country: "nl", Where: "Amsterdam", Page: 1, PerPage: 20},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestExpandSearchesOneCountryManyCities(t *testing.T) {
	got := ExpandSearches(SearchParams{Query: "DevOps", Country: "in", Where: "Pune,Mumbai"})
	if len(got) != 2 || got[0].Where != "Pune" || got[1].Where != "Mumbai" {
		t.Fatalf("%#v", got)
	}
	if got[0].Country != "in" || got[1].Country != "in" {
		t.Fatalf("country %#v", got)
	}
}

func TestExpandSearchesCountriesOnly(t *testing.T) {
	got := ExpandSearches(SearchParams{Query: "DevOps", Country: "in,nl"})
	if len(got) != 2 || got[0].Country != "in" || got[1].Country != "nl" {
		t.Fatalf("%#v", got)
	}
	if got[0].Query != "DevOps" || got[0].Where != "" {
		t.Fatalf("%#v", got)
	}
}
