package main

import (
	"path/filepath"
	"regexp"
	"strings"
)

type releaseInfo struct {
	Title string
	Year  string
}

var (
	releaseYearRe  = regexp.MustCompile(`(?i)(?:^|[.\s_\-\(\[\{])((?:19|20)\d{2})(?:[.\s_\-)\)\]\}]|$)`)
	releaseTagRe   = regexp.MustCompile(`(?i)[.\s_\-]+(UHD|BluRay|Blu-?Ray|BDRip|WEB-?DL|WEBRip|Remux|2160p|1080p|720p|576p|HEVC|H\.?265|x265|H\.?264|x264|AVC|DTS[- ]?HD(?:[ .]?MA)?|DTS|TrueHD|Atmos|AAC|AC3|EAC3|DD\+?|FLAC|10bit|8bit|HDR10\+?|HDR|DV|DoVi|SDR).*$`)
	leadingGroupRe = regexp.MustCompile(`^\[[^\]]+\][\s._-]*`)
)

func parseReleaseName(path string) releaseInfo {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// Strip site prefixes such as www.example.com@
	if i := strings.LastIndex(name, "@"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)

	title := name
	year := ""
	if matches := releaseYearRe.FindAllStringSubmatchIndex(name, -1); len(matches) > 0 {
		loc := matches[len(matches)-1]
		year = name[loc[2]:loc[3]]
		title = name[:loc[0]]
	} else {
		title = releaseTagRe.ReplaceAllString(title, "")
	}

	title = leadingGroupRe.ReplaceAllString(title, "")
	title = strings.NewReplacer(".", " ", "_", " ").Replace(title)
	title = whitespaceRe.ReplaceAllString(title, " ")
	title = strings.Trim(title, " -[](){}")
	title = strings.TrimSpace(title)

	return releaseInfo{Title: title, Year: year}
}
