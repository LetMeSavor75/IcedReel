package main

import (
	"strings"
	"testing"
)

func TestBuildNameFull(t *testing.T) {
	meta := &BilingualTitle{
		ImdbID:   "tt0111161",
		TmdbID:   278,
		TitleEn:  "The Shawshank Redemption",
		TitleZh:  "肖申克的救赎",
		Year:     "1994",
		DoubanID: "1292052",
	}
	want := "肖申克的救赎 (The Shawshank Redemption) (1994) {imdb-tt0111161} {tmdb-278} {douban-1292052}"
	if got := BuildName(meta); got != want {
		t.Fatalf("BuildName:\n got  %q\n want %q", got, want)
	}
}

func TestBuildNameEyesWideShut(t *testing.T) {
	meta := &BilingualTitle{
		ImdbID:   "tt0120663",
		TmdbID:   345,
		TitleEn:  "Eyes Wide Shut",
		TitleZh:  "大开眼戒",
		Year:     "1999",
		DoubanID: "1300275",
	}
	want := "大开眼戒 (Eyes Wide Shut) (1999) {imdb-tt0120663} {tmdb-345} {douban-1300275}"
	if got := BuildName(meta); got != want {
		t.Fatalf("BuildName:\n got  %q\n want %q", got, want)
	}
}

func TestBuildNameIgnoresRuntime(t *testing.T) {
	meta := &BilingualTitle{
		ImdbID:         "tt0111161",
		TmdbID:         278,
		TitleEn:        "The Shawshank Redemption",
		TitleZh:        "肖申克的救赎",
		Year:           "1994",
		DoubanID:       "1292052",
		RuntimeMinutes: 142,
	}
	want := "肖申克的救赎 (The Shawshank Redemption) (1994) {imdb-tt0111161} {tmdb-278} {douban-1292052}"
	if got := BuildName(meta); got != want {
		t.Fatalf("BuildName:\n got  %q\n want %q", got, want)
	}
}

func TestBuildNameMissingOptional(t *testing.T) {
	meta := &BilingualTitle{
		ImdbID:  "tt9",
		TitleEn: "Foo",
		Year:    "2020",
	}
	want := "Foo (2020) {imdb-tt9}"
	if got := BuildName(meta); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildNameEnglishOnly(t *testing.T) {
	meta := &BilingualTitle{
		ImdbID:   "tt123",
		TitleEn:  "Foo",
		Year:     "2021",
		DoubanID: "456",
	}
	want := "Foo (2021) {imdb-tt123} {douban-456}"
	if got := BuildName(meta); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatRuntime(t *testing.T) {
	cases := []struct {
		minutes int
		exact   string
		total   string
	}{
		{142, "2h22m", "142min"},
		{22, "22m", "22min"},
		{0, "", ""},
	}
	for _, c := range cases {
		exact, total := formatRuntime(c.minutes)
		if exact != c.exact || total != c.total {
			t.Errorf("formatRuntime(%d) = (%q,%q), want (%q,%q)", c.minutes, exact, total, c.exact, c.total)
		}
	}
}

func TestSanitizeSegment(t *testing.T) {
	cases := map[string]string{
		"A（B）":     "A (B)",
		"a:b/c?_x": "a b c x",
		"  foo  ":  "foo",
		"a_b":      "a b",
	}
	for in, want := range cases {
		if got := sanitizeSegment(in); got != want {
			t.Errorf("sanitizeSegment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeImdbID(t *testing.T) {
	cases := map[string]string{
		"tt0111161":                             "tt0111161",
		"0111161":                               "tt0111161",
		"https://www.imdb.com/title/tt0111161/": "tt0111161",
		"":                                      "",
		"abc":                                   "",
	}
	for in, want := range cases {
		if got := NormalizeImdbID(in); got != want {
			t.Errorf("NormalizeImdbID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLooksMostlyLatin(t *testing.T) {
	if !looksMostlyLatin("The Shawshank Redemption") {
		t.Error("English should be mostly Latin")
	}
	if looksMostlyLatin("肖申克的救赎") {
		t.Error("Chinese should not be mostly Latin")
	}
	if !looksMostlyLatin("") {
		t.Error("empty should be mostly Latin per IcedSub")
	}
}

func TestFormatTitle(t *testing.T) {
	cases := []struct{ zh, en, want string }{
		{"乙", "A", "乙 (A)"},
		{"", "A", "A"},
		{"乙", "", "乙"},
		{"Same", "Same", "Same"},
	}
	for _, c := range cases {
		if got := formatTitle(c.zh, c.en); got != c.want {
			t.Errorf("formatTitle(%q,%q) = %q, want %q", c.zh, c.en, got, c.want)
		}
	}
}

func TestBuildNameNoEmptySegments(t *testing.T) {
	meta := &BilingualTitle{TitleEn: "Foo"}
	got := BuildName(meta)
	if strings.Contains(got, "  ") || strings.HasSuffix(got, " ") {
		t.Errorf("unexpected separators: %q", got)
	}
}
