package main

import "testing"

func TestParseReleaseName(t *testing.T) {
	cases := []struct {
		path  string
		title string
		year  string
	}{
		{
			`D:\Downloads\www.example.com@The.Shawshank.Redemption.1994.UHD.BluRay.REMUX.2160p.HEVC.DTS-HD.MA.5.1.mkv`,
			"The Shawshank Redemption", "1994",
		},
		{`Movie.2020.1080p.BluRay.x264.mkv`, "Movie", "2020"},
		{`www.site@Some Movie (2019) [2160p].mkv`, "Some Movie", "2019"},
		{`1917 (2019).1080p.mkv`, "1917", "2019"},
		{`2001 A Space Odyssey (1968).mkv`, "2001 A Space Odyssey", "1968"},
		{`No.Year.1080p.mkv`, "No Year", ""},
		{`[Group] Title (1999).mkv`, "Title", "1999"},
	}
	for _, c := range cases {
		got := parseReleaseName(c.path)
		if got.Title != c.title || got.Year != c.year {
			t.Errorf("parseReleaseName(%q) = %+v, want title=%q year=%q", c.path, got, c.title, c.year)
		}
	}
}
