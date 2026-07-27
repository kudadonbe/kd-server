package main

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Acme Co", "acme-co"},
		{"  Leading/Trailing  ", "leadingtrailing"},
		{"Already-Slug", "already-slug"},
		{"Numbers 123", "numbers-123"},
		{"", ""},
	}

	for _, c := range cases {
		if got := slugify(c.input); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
