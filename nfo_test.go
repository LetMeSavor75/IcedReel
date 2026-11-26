package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testNFOInputs() (*BilingualTitle, *mediaInfo) {
	meta := &BilingualTitle{
		ImdbID:       "tt0111161",
		TmdbID:       278,
		TitleZh:      "肖申克的救赎",
		TitleEn:      "The Shawshank Redemption",
		Year:         "1994",
		DoubanID:     "1292052",
		WikidataID:   "Q172241",
		Overview:     "剧情简介",
		Tagline:      "tagline",
		VoteAverage:  6.8,
		VoteCount:    123,
		PosterPath:   "/poster.jpg",
		BackdropPath: "/backdrop.jpg",
		Genres:       []string{"剧情"},
		Studios:      []string{"Studio"},
		Countries:    []string{"United States"},
		ReleaseDate:  "1994-10-14",
	}
	media := &mediaInfo{
		DurationSeconds: 8520.0,
		SizeBytes:       58864563961,
		BitRate:         83955985,
		Video: &videoInfo{
			Codec: "HEVC", CodecRaw: "hevc", Profile: "Main 10",
			Width: 3840, Height: 2160, BitDepth: 10, HDR: "HDR10", FPS: 23.976,
		},
		Audio: []audioTrack{
			{Index: 1, Language: "it", Codec: "DTS-HD MA", CodecRaw: "dts", Title: "意大利语 DTS-HD 2.0", Channels: "2.0", ChannelsCount: 2, SampleRate: 48000, BitDepth: 24, Default: true},
			{Index: 2, Language: "en", Codec: "DD", CodecRaw: "ac3", Title: "英语 DD 2.0", Channels: "2.0", ChannelsCount: 2, SampleRate: 48000, BitDepth: 16, Default: true},
		},
		Subtitles: []subtitleTrack{
			{Index: 4, Language: "zh", Format: "PGS", CodecRaw: "hdmv_pgs_subtitle", Title: "简英双语", Default: true},
		},
	}
	return meta, media
}

func TestBuildNFOTVType(t *testing.T) {
	meta, media := testNFOInputs()
	meta.IsTV = true
	doc := buildNFO(meta, media)
	if doc.TmdbType != "tv" {
		t.Errorf("tmdbtype = %q", doc.TmdbType)
	}
	data, err := renderNFO(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<tmdbtype>tv</tmdbtype>") {
		t.Errorf("missing tmdbtype:\n%s", data)
	}
}

func TestBuildNFOAndWrite(t *testing.T) {
	meta, media := testNFOInputs()
	doc := buildNFO(meta, media)
	if doc == nil {
		t.Fatal("nil NFO")
	}
	if doc.Title != "肖申克的救赎" || doc.OriginalTitle != "The Shawshank Redemption" || doc.Runtime != 142 {
		t.Errorf("doc = %+v", doc)
	}
	if len(doc.UniqueIDs) != 3 || doc.UniqueIDs[0].Type != "imdb" || !doc.UniqueIDs[0].Default {
		t.Errorf("uniqueids = %+v", doc.UniqueIDs)
	}
	if doc.StreamDetails == nil || doc.StreamDetails.Video == nil ||
		doc.StreamDetails.Video.Codec != "hevc" || doc.StreamDetails.Video.HDRType != "hdr10" {
		t.Fatalf("streamdetails = %+v", doc.StreamDetails)
	}
	if len(doc.StreamDetails.Audio) != 2 || doc.StreamDetails.Audio[0].Codec != "dtshd_ma" ||
		doc.StreamDetails.Audio[0].Title != "意大利语 DTS-HD 2.0" ||
		doc.StreamDetails.Audio[0].SampleRate != 48000 || doc.StreamDetails.Audio[0].BitDepth != 24 ||
		!doc.StreamDetails.Audio[0].Default {
		t.Errorf("audio = %+v", doc.StreamDetails.Audio)
	}
	if len(doc.StreamDetails.Subtitles) != 1 || doc.StreamDetails.Subtitles[0].Language != "zh" ||
		doc.StreamDetails.Subtitles[0].Title != "简英双语" {
		t.Errorf("subtitles = %+v", doc.StreamDetails.Subtitles)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "movie.nfo")
	written, err := writeNFO(path, doc, false)
	if err != nil || !written {
		t.Fatalf("writeNFO = %v, %v", written, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`,
		"<logicalname>肖申克的救赎 (The Shawshank Redemption) (1994) {imdb-tt0111161} {tmdb-278} {douban-1292052}</logicalname>",
		"<title>肖申克的救赎</title>",
		"<premiered>1994-10-14</premiered>",
		"<runtime>142</runtime>",
		`<uniqueid type="imdb" default="true">tt0111161</uniqueid>`,
		"<samplerate>48000</samplerate>",
		"<bitdepth>24</bitdepth>",
		"<default>true</default>",
		"<wikidataid>Q172241</wikidataid>",
		"<hdrtype>hdr10</hdrtype>",
		"<codec>dtshd_ma</codec>",
		"<title>意大利语 DTS-HD 2.0</title>",
		"<language>zh</language>",
		"<title>简英双语</title>",
		"https://image.tmdb.org/t/p/original/poster.jpg",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("NFO missing %q:\n%s", want, text)
		}
	}

	written, err = writeNFO(path, doc, false)
	if err != nil || written {
		t.Fatalf("second writeNFO = %v, %v; should skip", written, err)
	}
	written, err = writeNFO(path, doc, true)
	if err != nil || !written {
		t.Fatalf("overwrite writeNFO = %v, %v", written, err)
	}
}

func TestNFODiff(t *testing.T) {
	old := []byte("a\nb\nc\n")
	next := []byte("a\nB\nc\n")
	diff, err := nfoDiff(old, next)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "-b") || !strings.Contains(diff, "+B") {
		t.Errorf("diff = %q", diff)
	}
	if diff, _ := nfoDiff([]byte("a\r\nb\r\n"), []byte("a\nb\n")); diff != "" {
		t.Errorf("line endings should normalize, diff = %q", diff)
	}
}

func TestKodiCodecMappings(t *testing.T) {
	if got := kodiAudioCodec(audioTrack{CodecRaw: "dts", Codec: "DTS-HD MA"}); got != "dtshd_ma" {
		t.Errorf("dtshd_ma = %q", got)
	}
	if got := kodiAudioCodec(audioTrack{CodecRaw: "ac3"}); got != "ac3" {
		t.Errorf("ac3 = %q", got)
	}
	if got := kodiHDRType("HDR10+"); got != "hdr10plus" {
		t.Errorf("hdr10plus = %q", got)
	}
	if got := kodiHDRType("SDR"); got != "" {
		t.Errorf("sdr = %q", got)
	}
}
