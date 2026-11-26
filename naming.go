package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type BilingualTitle struct {
	ImdbID         string
	TmdbID         int
	WikidataID     string
	TitleEn        string
	TitleZh        string
	Year           string
	DoubanID       string
	RuntimeMinutes int
	IsTV           bool
	Overview       string
	Tagline        string
	VoteAverage    float64
	VoteCount      int
	PosterPath     string
	BackdropPath   string
	Genres         []string
	Studios        []string
	Countries      []string
	ReleaseDate    string
}

// BuildName builds the logical directory name:
// 中文 (English) (年份) {imdb-tt...} {tmdb-...} {douban-...}
// It describes the work, not a specific file. Duration/audio/subtitle belong to scrape.
func BuildName(meta *BilingualTitle) string {
	if meta == nil {
		return ""
	}

	var parts []string
	if title := formatTitle(meta.TitleZh, meta.TitleEn); title != "" {
		parts = append(parts, title)
	}
	if year := sanitizeSegment(meta.Year); len(year) == 4 {
		parts = append(parts, "("+year+")")
	}
	if id := cleanID(meta.ImdbID); id != "" {
		parts = append(parts, "{imdb-"+id+"}")
	}
	if meta.TmdbID > 0 {
		parts = append(parts, fmt.Sprintf("{tmdb-%d}", meta.TmdbID))
	}
	if id := cleanID(meta.DoubanID); id != "" {
		parts = append(parts, "{douban-"+id+"}")
	}
	return strings.Join(parts, " ")
}

func formatTitle(zh, en string) string {
	zh = sanitizeSegment(zh)
	en = sanitizeSegment(en)
	switch {
	case zh == "":
		return en
	case en == "":
		return zh
	case strings.EqualFold(zh, en):
		return zh
	default:
		return zh + " (" + en + ")"
	}
}

var whitespaceRe = regexp.MustCompile(`\s+`)

// sanitizeSegment follows IcedSub's BilingualNamePattern.SanitizeSegment:
// full-width parens to ASCII, Windows-illegal characters to spaces,
// underscores to spaces (they separate name segments), collapse whitespace.
func sanitizeSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var b strings.Builder
	last := rune(0)
	for _, r := range s {
		switch r {
		case '（':
			if b.Len() > 0 && last != ' ' && last != '(' {
				b.WriteRune(' ')
			}
			b.WriteRune('(')
			last = '('
		case '）':
			b.WriteRune(')')
			last = ')'
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\x00', '\r', '\n', '\t', '_':
			b.WriteRune(' ')
			last = ' '
		default:
			b.WriteRune(r)
			last = r
		}
	}
	return strings.Trim(whitespaceRe.ReplaceAllString(b.String(), " "), " .")
}

func cleanID(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// formatRuntime is kept for the future scrape layer; name no longer uses it.
func formatRuntime(minutes int) (string, string) {
	if minutes <= 0 {
		return "", ""
	}
	h := minutes / 60
	m := minutes % 60
	var exact string
	if h > 0 {
		exact = fmt.Sprintf("%dh%02dm", h, m)
	} else {
		exact = fmt.Sprintf("%dm", m)
	}
	return exact, fmt.Sprintf("%dmin", minutes)
}

var (
	imdbURLRe = regexp.MustCompile(`(?i)imdb\.com/title/(tt\d+)`)
	nonDigits = regexp.MustCompile(`\D`)
)

func NormalizeImdbID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if m := imdbURLRe.FindStringSubmatch(raw); m != nil {
		raw = m[1]
	}
	if strings.HasPrefix(strings.ToLower(raw), "tt") {
		raw = "tt" + nonDigits.ReplaceAllString(raw[2:], "")
	} else {
		raw = "tt" + nonDigits.ReplaceAllString(raw, "")
	}
	if raw == "tt" {
		return ""
	}
	return raw
}

func looksMostlyLatin(s string) bool {
	if strings.TrimSpace(s) == "" {
		return true
	}
	letters, latin := 0, 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				latin++
			}
		}
	}
	return letters > 0 && latin*2 >= letters
}

func hasCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}
