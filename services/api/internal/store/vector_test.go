package store

import "testing"

func TestFormatParseVector(t *testing.T) {
	in := []float64{0.1, -2, 3.5}
	got, err := ParseVector(FormatVector(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != 0.1 || got[2] != 3.5 {
		t.Fatalf("%v", got)
	}
	if FormatVector(nil) != "[]" {
		t.Fatal(FormatVector(nil))
	}
}
