package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrintNFOReport(t *testing.T) {
	doc := &nfoMovie{
		Title:         "肖申克的救赎",
		OriginalTitle: "The Shawshank Redemption",
		Year:          "1994",
		Premiered:     "1994-10-14",
		Runtime:       142,
		Plot:          "剧情简介",
		Rating:        6,
		Votes:         134,
		UniqueIDs: []nfoUniqueID{
			{Type: "imdb", Default: true, Value: "tt0111161"},
			{Type: "tmdb", Value: "278"},
		},
		Genres:     []string{"剧情"},
		Studios:    []string{"Studio"},
		Countries:  []string{"United States"},
		WikidataID: "Q172241",
		StreamDetails: &nfoStreamDetails{
			Video: &nfoVideo{Codec: "hevc", Width: 3840, Height: 2160, DurationSeconds: 8520, HDRType: "hdr10plus"},
			Audio: []nfoAudio{{Codec: "dtshd_ma", Language: "en", Channels: 6, Title: "英语 DTS-HD 5.1", SampleRate: 48000, BitDepth: 24, Default: true}},
			Subtitles: []nfoSubtitle{
				{Language: "zh", Title: "简英双语"},
				{Language: "zh", Title: "简体中文"},
			},
		},
	}

	var buf bytes.Buffer
	printNFOReport(&buf, "movie.nfo", doc)
	out := buf.String()
	for _, want := range []string{
		"标题:   肖申克的救赎",
		"原名:   The Shawshank Redemption",
		"链接:\n  imdb https://www.imdb.com/title/tt0111161/\n  tmdb https://www.themoviedb.org/movie/278\n  wikidata https://www.wikidata.org/wiki/Q172241",
		"评分:   6.0 (134)",
		"视频:   3840x2160 HEVC HDR10+ 2:22:00 (142 min)",
		"音轨:\n  English, 英语 DTS-HD 5.1 (Audio 1) [Default] (DTS-HD MA, 48.0 kHz, 5.1 chn, 24 bit)",
		"字幕:\n  Chinese, 简英双语\n  Chinese, 简体中文",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "时长:   142min") {
		t.Errorf("runtime 行不应与视频秒数重复:\n%s", out)
	}
	if strings.Contains(out, "上映:") {
		t.Errorf("不应显示上映日期:\n%s", out)
	}
	if strings.Contains(out, "识别:") {
		t.Errorf("不应显示识别行:\n%s", out)
	}
}

func TestFormatNFOAudioLine(t *testing.T) {
	cases := []struct {
		audio  nfoAudio
		number int
		want   string
	}{
		{
			nfoAudio{Codec: "dtshd_ma", Language: "en", Channels: 6, Title: "英语", SampleRate: 48000, BitDepth: 24, Default: true},
			1,
			"English, 英语 (Audio 1) [Default] (DTS-HD MA, 48.0 kHz, 5.1 chn, 24 bit)",
		},
		{
			nfoAudio{Codec: "dtshd_ma", Language: "zh", Channels: 6, Title: "国语(六区)", SampleRate: 48000, BitDepth: 16},
			2,
			"Chinese, 国语(六区) (Audio 2) (DTS-HD MA, 48.0 kHz, 5.1 chn, 16 bit)",
		},
		{
			nfoAudio{Codec: "aac", Language: "und", Channels: 1, SampleRate: 48000},
			3,
			"未知 (Audio 3) (AAC, 48.0 kHz, 1.0 chn)",
		},
	}
	for _, c := range cases {
		if got := formatNFOAudioLine(c.audio, c.number); got != c.want {
			t.Errorf("formatNFOAudioLine(%+v, %d) = %q, want %q", c.audio, c.number, got, c.want)
		}
	}
}

func TestPrintNFOReportRuntimeFallback(t *testing.T) {
	doc := &nfoMovie{Title: "Movie", Runtime: 93}
	var buf bytes.Buffer
	printNFOReport(&buf, "movie.nfo", doc)
	if !strings.Contains(buf.String(), "时长:   93min") {
		t.Errorf("缺少 runtime 兜底:\n%s", buf.String())
	}
}

func TestPrintNFOReportFallbackIDs(t *testing.T) {
	doc := &nfoMovie{
		Title:    "Movie",
		ImdbID:   "tt1234567",
		TmdbID:   42,
		DoubanID: "100",
	}
	var buf bytes.Buffer
	printNFOReport(&buf, "movie.nfo", doc)
	out := buf.String()
	if strings.Contains(out, "识别:") {
		t.Errorf("不应显示识别行 = %q", out)
	}
	if !strings.Contains(out, "douban https://movie.douban.com/subject/100/") {
		t.Errorf("fallback link = %q", out)
	}
}

func TestPrintNFOReportUsesTVLink(t *testing.T) {
	doc := &nfoMovie{
		Title:    "青春共体",
		TmdbID:   291507,
		TmdbType: "tv",
		ImdbID:   "tt8014982",
	}
	var buf bytes.Buffer
	printNFOReport(&buf, "test.nfo", doc)
	if !strings.Contains(buf.String(), "https://www.themoviedb.org/tv/291507") {
		t.Errorf("missing tv link:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "https://www.themoviedb.org/movie/291507") {
		t.Errorf("unexpected movie link:\n%s", buf.String())
	}
}

func TestMetadataLinks(t *testing.T) {
	links := metadataLinks("tt0111161", 278, false, "1292052")
	want := []metadataLink{
		{Label: "imdb", URL: "https://www.imdb.com/title/tt0111161/"},
		{Label: "tmdb", URL: "https://www.themoviedb.org/movie/278"},
		{Label: "douban", URL: "https://movie.douban.com/subject/1292052/"},
	}
	if len(links) != len(want) {
		t.Fatalf("links = %v", links)
	}
	for i := range want {
		if links[i] != want[i] {
			t.Errorf("links[%d] = %+v, want %+v", i, links[i], want[i])
		}
	}

	tv := metadataLinks("", 1399, true, "")
	if len(tv) != 1 || tv[0].URL != "https://www.themoviedb.org/tv/1399" {
		t.Errorf("tv links = %v", tv)
	}
}

func TestOSC8Link(t *testing.T) {
	got := osc8Link("https://example.com", "https://example.com")
	want := "\x1b]8;;https://example.com\x1b\\https://example.com\x1b]8;;\x1b\\"
	if got != want {
		t.Errorf("osc8Link = %q, want %q", got, want)
	}
}

func TestPrintMetadataLinks(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	links := []metadataLink{{Label: "imdb", URL: "https://www.imdb.com/title/tt1/"}}
	var plain bytes.Buffer
	printMetadataLinks(&plain, links, false)
	if plain.String() != "链接:\n  imdb https://www.imdb.com/title/tt1/\n" {
		t.Errorf("plain = %q", plain.String())
	}

	var linked bytes.Buffer
	printMetadataLinks(&linked, links, true)
	if !strings.Contains(linked.String(), "\x1b]8;;https://www.imdb.com/title/tt1/\x1b\\") {
		t.Errorf("linked = %q", linked.String())
	}
}

func TestShowNFO(t *testing.T) {
	doc := &nfoMovie{
		Title: "测试电影",
		Year:  "2024",
		StreamDetails: &nfoStreamDetails{
			Video: &nfoVideo{Codec: "h264", Width: 1920, Height: 1080},
		},
	}
	data, err := renderNFO(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "movie.nfo")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := showNFO(path, &stdout, &stderr, posterOptions{Width: 32, Mode: posterNone})
	if code != 0 {
		t.Fatalf("showNFO = %d, stderr = %s", code, stderr.String())
	}
	for _, want := range []string{"标题:   测试电影", "年份:   2024", "视频:   1920x1080 H.264"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("missing %q in:\n%s", want, stdout.String())
		}
	}
}

func TestParsePosterOptions(t *testing.T) {
	opts, err := parsePosterOptions(32, false, "auto", false)
	if err != nil || opts.Width != 32 || opts.Mode != posterAuto || opts.NoPoster {
		t.Errorf("default opts = %+v, err = %v", opts, err)
	}
	if _, err := parsePosterOptions(3, false, "auto", false); err == nil {
		t.Error("width 3 should fail")
	}
	if _, err := parsePosterOptions(32, false, "bad", false); err == nil {
		t.Error("bad image mode should fail")
	}
}

func TestShowVideoUsesSiblingNFO(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "movie.mkv")
	nfo := filepath.Join(dir, "movie.nfo")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := renderNFO(&nfoMovie{Title: "测试电影", Year: "2024"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nfo, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runShow([]string{"--no-poster", video}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runShow = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "标题:   测试电影") {
		t.Errorf("missing NFO content:\n%s", stdout.String())
	}
}

func TestShowVideoMissingSiblingNFO(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "movie.mkv")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runShow([]string{"--no-poster", video}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "找不到同名 NFO") {
		t.Errorf("code = %d, stderr = %s", code, stderr.String())
	}
}

func TestNFOPosterSource(t *testing.T) {
	dir := t.TempDir()
	nfoPath := filepath.Join(dir, "movie.nfo")
	doc := &nfoMovie{Thumbs: []nfoThumb{
		{Aspect: "backdrop", URL: "https://example.com/back.jpg"},
		{URL: "https://example.com/plain.jpg"},
	}}
	if got := nfoPosterSource(doc, nfoPath); got != "https://example.com/plain.jpg" {
		t.Errorf("first thumb fallback = %q", got)
	}

	doc.Thumbs = append([]nfoThumb{{Aspect: "poster", URL: "https://image.tmdb.org/t/p/original/poster.jpg"}}, doc.Thumbs...)
	if got := nfoPosterSource(doc, nfoPath); got != "https://image.tmdb.org/t/p/w500/poster.jpg" {
		t.Errorf("aspect poster = %q", got)
	}

	local := filepath.Join(dir, "poster.png")
	if err := os.WriteFile(local, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := nfoPosterSource(doc, nfoPath); got != local {
		t.Errorf("local poster = %q", got)
	}
}

func TestShowEpisodeNFO(t *testing.T) {
	doc := &nfoEpisode{
		Title:     "试播集",
		ShowTitle: "测试剧",
		Season:    1,
		Episode:   2,
		Aired:     "2024-01-02",
		Rating:    8.1,
		Votes:     100,
		Plot:      "单集剧情",
		StreamDetails: &nfoStreamDetails{
			Video: &nfoVideo{Codec: "h264", Width: 1920, Height: 1080},
		},
	}
	data, err := renderNFO(doc)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	showData, err := renderNFO(&nfoTVShow{Title: "测试剧", TmdbID: 42, UniqueIDs: []nfoUniqueID{{Type: "tmdb", Value: "42"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tvshow.nfo"), showData, 0o644); err != nil {
		t.Fatal(err)
	}
	seasonDir := filepath.Join(dir, "Season 1")
	if err := os.MkdirAll(seasonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(seasonDir, "Show.S01E02.nfo")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := showNFO(path, &stdout, &stderr, posterOptions{NoPoster: true})
	if code != 0 {
		t.Fatalf("showNFO = %d, stderr = %s", code, stderr.String())
	}
	for _, want := range []string{
		"剧名:   测试剧",
		"季集:   S01E02",
		"单集:   试播集",
		"播出:   2024-01-02",
		"评分:   8.1 (100)",
		"简介:   单集剧情",
		"视频:   1920x1080 H.264",
		"episode https://www.themoviedb.org/tv/42/season/1/episode/2",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("missing %q in:\n%s", want, stdout.String())
		}
	}
}

func TestShowSeasonNFO(t *testing.T) {
	doc := &nfoSeason{
		Title:        "第 1 季",
		SeasonNumber: 1,
		Premiered:    "2026-11-11",
		Year:         "2026",
		DoubanID:     "30145677",
		Plot:         "季简介",
	}
	data, err := renderNFO(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "season.nfo")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := showNFO(path, &stdout, &stderr, posterOptions{NoPoster: true})
	if code != 0 {
		t.Fatalf("showNFO = %d, stderr = %s", code, stderr.String())
	}
	for _, want := range []string{"季名:   第 1 季", "季号:   1", "首播:   2026-11-11", "douban https://movie.douban.com/subject/30145677/"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("missing %q in:\n%s", want, stdout.String())
		}
	}
}

func TestShowTVShowNFO(t *testing.T) {
	doc := &nfoTVShow{
		Title:     "测试剧",
		Year:      "2024",
		Rating:    8,
		Votes:     100,
		Genres:    []string{"剧情"},
		Plot:      "剧集简介",
		UniqueIDs: []nfoUniqueID{{Type: "tmdb", Value: "42"}},
	}
	data, err := renderNFO(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tvshow.nfo")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := showNFO(path, &stdout, &stderr, posterOptions{NoPoster: true})
	if code != 0 {
		t.Fatalf("showNFO = %d, stderr = %s", code, stderr.String())
	}
	for _, want := range []string{"剧名:   测试剧", "年份:   2024", "评分:   8.0 (100)", "简介:   剧集简介"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("missing %q in:\n%s", want, stdout.String())
		}
	}
}

func TestLoadSiblingTVShowLogicalName(t *testing.T) {
	dir := t.TempDir()
	showPath := filepath.Join(dir, "Show_2026_imdb-tt1.nfo")
	data, err := renderNFO(&nfoTVShow{Title: "测试剧", TmdbID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(showPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	show, gotPath := loadSiblingTVShow(filepath.Join(dir, "episode.nfo"))
	if show == nil || show.Title != "测试剧" || gotPath != showPath {
		t.Errorf("show = %+v, path = %q", show, gotPath)
	}
}

func TestNFOFanartSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "movie.nfo")
	doc := &nfoMovie{Fanart: &nfoFanart{Thumb: "https://image.tmdb.org/t/p/original/back.jpg"}}
	if got := nfoFanartSource(doc, path); got != "https://image.tmdb.org/t/p/w500/back.jpg" {
		t.Errorf("fanart = %q", got)
	}
	if got := nfoFanartSource(&nfoMovie{}, path); got != "" {
		t.Errorf("empty fanart = %q", got)
	}
}

func TestLocalFanartSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tvshow.nfo")
	local := filepath.Join(dir, "fanart.jpg")
	if err := os.WriteFile(local, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := localFanartSource(path); got != local {
		t.Errorf("local fanart = %q, want %q", got, local)
	}
}

func TestFormatNFOSubtitleLine(t *testing.T) {
	cases := []struct {
		sub  nfoSubtitle
		want string
	}{
		{nfoSubtitle{Language: "zh", Title: "简英特字"}, "Chinese, 简英特字"},
		{nfoSubtitle{Language: "zh", Title: "简体特字(六区)"}, "Chinese, 简体特字(六区)"},
		{nfoSubtitle{Language: "zh", Title: "繁體中字"}, "Chinese, 繁體中字"},
		{nfoSubtitle{Language: "en", Title: "英字"}, "English, 英字"},
		{nfoSubtitle{Language: "zh"}, "Chinese, 中文"},
	}
	for _, c := range cases {
		if got := formatNFOSubtitleLine(c.sub); got != c.want {
			t.Errorf("formatNFOSubtitleLine(%+v) = %q, want %q", c.sub, got, c.want)
		}
	}
}

func TestNFOAudioCodecLabel(t *testing.T) {
	cases := map[string]string{
		"dtshd_ma":   "DTS-HD MA",
		"dtshd_hra":  "DTS-HD HRA",
		"ac3":        "DD",
		"eac3":       "DD+",
		"truehd":     "TrueHD",
		"pcm_bluray": "PCM",
		"":           "未知",
	}
	for in, want := range cases {
		if got := nfoAudioCodecLabel(in); got != want {
			t.Errorf("nfoAudioCodecLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLanguageDisplayLabel(t *testing.T) {
	cases := map[string]string{
		"it":  "意大利语",
		"en":  "英语",
		"zho": "中文",
		"und": "未知",
		"xx":  "xx",
	}
	for in, want := range cases {
		if got := languageDisplayLabel(in); got != want {
			t.Errorf("languageDisplayLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNfoHDRLabel(t *testing.T) {
	cases := map[string]string{
		"hdr10":       "HDR10",
		"hdr10plus":   "HDR10+",
		"dolbyvision": "Dolby Vision",
		"hlg":         "HLG",
		"":            "",
	}
	for in, want := range cases {
		if got := nfoHDRLabel(in); got != want {
			t.Errorf("nfoHDRLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
