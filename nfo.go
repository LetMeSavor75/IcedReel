package main

import (
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

const (
	nfoHeader     = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"
	tmdbImageBase = "https://image.tmdb.org/t/p/original"
)

type nfoMovie struct {
	XMLName       xml.Name          `xml:"movie"`
	LogicalName   string            `xml:"logicalname,omitempty"`
	Title         string            `xml:"title"`
	OriginalTitle string            `xml:"originaltitle,omitempty"`
	Year          string            `xml:"year,omitempty"`
	Premiered     string            `xml:"premiered,omitempty"`
	Runtime       int               `xml:"runtime,omitempty"`
	Plot          string            `xml:"plot,omitempty"`
	Tagline       string            `xml:"tagline,omitempty"`
	Rating        float64           `xml:"rating,omitempty"`
	Votes         int               `xml:"votes,omitempty"`
	UniqueIDs     []nfoUniqueID     `xml:"uniqueid"`
	ImdbID        string            `xml:"imdbid,omitempty"`
	TmdbID        int               `xml:"tmdbid,omitempty"`
	TmdbType      string            `xml:"tmdbtype,omitempty"`
	DoubanID      string            `xml:"doubanid,omitempty"`
	WikidataID    string            `xml:"wikidataid,omitempty"`
	Genres        []string          `xml:"genre,omitempty"`
	Studios       []string          `xml:"studio,omitempty"`
	Countries     []string          `xml:"country,omitempty"`
	Thumbs        []nfoThumb        `xml:"thumb,omitempty"`
	Fanart        *nfoFanart        `xml:"fanart,omitempty"`
	StreamDetails *nfoStreamDetails `xml:"streamdetails,omitempty"`
}

type nfoUniqueID struct {
	Type    string `xml:"type,attr"`
	Default bool   `xml:"default,attr,omitempty"`
	Value   string `xml:",chardata"`
}

type nfoThumb struct {
	Aspect string `xml:"aspect,attr,omitempty"`
	URL    string `xml:",chardata"`
}

type nfoFanart struct {
	Thumb string `xml:"thumb"`
}

type nfoStreamDetails struct {
	Video     *nfoVideo     `xml:"video,omitempty"`
	Audio     []nfoAudio    `xml:"audio,omitempty"`
	Subtitles []nfoSubtitle `xml:"subtitle,omitempty"`
}

type nfoVideo struct {
	Codec           string `xml:"codec"`
	Width           int    `xml:"width,omitempty"`
	Height          int    `xml:"height,omitempty"`
	DurationSeconds int    `xml:"durationinseconds,omitempty"`
	HDRType         string `xml:"hdrtype,omitempty"`
}

type nfoAudio struct {
	Codec      string `xml:"codec"`
	Language   string `xml:"language,omitempty"`
	Channels   int    `xml:"channels,omitempty"`
	Title      string `xml:"title,omitempty"`
	SampleRate int    `xml:"samplerate,omitempty"`
	BitDepth   int    `xml:"bitdepth,omitempty"`
	Default    bool   `xml:"default,omitempty"`
}

type nfoSubtitle struct {
	Language string `xml:"language,omitempty"`
	Title    string `xml:"title,omitempty"`
}

func buildNFO(meta *BilingualTitle, media *mediaInfo) *nfoMovie {
	if meta == nil {
		return nil
	}

	title := meta.TitleZh
	if title == "" {
		title = meta.TitleEn
	}
	doc := &nfoMovie{
		LogicalName:   BuildName(meta),
		Title:         title,
		OriginalTitle: meta.TitleEn,
		Year:          meta.Year,
		Premiered:     meta.ReleaseDate,
		Runtime:       nfoRuntimeMinutes(meta, media),
		Plot:          meta.Overview,
		Tagline:       meta.Tagline,
		Rating:        meta.VoteAverage,
		Votes:         meta.VoteCount,
		Genres:        meta.Genres,
		Studios:       meta.Studios,
		Countries:     meta.Countries,
		DoubanID:      meta.DoubanID,
		WikidataID:    meta.WikidataID,
	}

	if meta.ImdbID != "" {
		doc.UniqueIDs = append(doc.UniqueIDs, nfoUniqueID{
			Type: "imdb", Default: true, Value: meta.ImdbID,
		})
		doc.ImdbID = meta.ImdbID
	}
	if meta.TmdbID > 0 {
		doc.UniqueIDs = append(doc.UniqueIDs, nfoUniqueID{
			Type: "tmdb", Value: fmt.Sprintf("%d", meta.TmdbID),
		})
		doc.TmdbID = meta.TmdbID
		if meta.IsTV {
			doc.TmdbType = "tv"
		}
	}
	if meta.DoubanID != "" {
		doc.UniqueIDs = append(doc.UniqueIDs, nfoUniqueID{
			Type: "douban", Value: meta.DoubanID,
		})
		doc.DoubanID = meta.DoubanID
	}

	if meta.PosterPath != "" {
		doc.Thumbs = append(doc.Thumbs, nfoThumb{
			Aspect: "poster",
			URL:    tmdbImageBase + meta.PosterPath,
		})
	}
	if meta.BackdropPath != "" {
		doc.Fanart = &nfoFanart{Thumb: tmdbImageBase + meta.BackdropPath}
	}

	doc.StreamDetails = buildNFOStreams(media)
	return doc
}

func buildNFOStreams(media *mediaInfo) *nfoStreamDetails {
	if media == nil {
		return nil
	}
	sd := &nfoStreamDetails{}
	if media.Video != nil {
		sd.Video = &nfoVideo{
			Codec:           media.Video.CodecRaw,
			Width:           media.Video.Width,
			Height:          media.Video.Height,
			DurationSeconds: int(math.Round(media.DurationSeconds)),
			HDRType:         kodiHDRType(media.Video.HDR),
		}
	}
	for _, a := range media.Audio {
		sd.Audio = append(sd.Audio, nfoAudio{
			Codec:      kodiAudioCodec(a),
			Language:   a.Language,
			Channels:   a.ChannelsCount,
			Title:      a.Title,
			SampleRate: a.SampleRate,
			BitDepth:   a.BitDepth,
			Default:    a.Default,
		})
	}
	for _, s := range media.Subtitles {
		sd.Subtitles = append(sd.Subtitles, nfoSubtitle{
			Language: s.Language,
			Title:    s.Title,
		})
	}
	if sd.Video == nil && len(sd.Audio) == 0 && len(sd.Subtitles) == 0 {
		return nil
	}
	return sd
}

func kodiAudioCodec(t audioTrack) string {
	switch strings.ToLower(t.CodecRaw) {
	case "dts":
		upper := strings.ToUpper(t.Codec)
		if strings.Contains(upper, "MA") {
			return "dtshd_ma"
		}
		if strings.Contains(upper, "HRA") {
			return "dtshd_hra"
		}
		return "dts"
	case "ac3":
		return "ac3"
	case "eac3":
		return "eac3"
	case "truehd":
		return "truehd"
	case "aac":
		return "aac"
	case "flac":
		return "flac"
	case "opus":
		return "opus"
	case "mp3":
		return "mp3"
	default:
		return strings.ToLower(t.CodecRaw)
	}
}

func kodiHDRType(hdr string) string {
	switch strings.ToLower(hdr) {
	case "hdr10":
		return "hdr10"
	case "hdr10+":
		return "hdr10plus"
	case "hlg":
		return "hlg"
	case "dolby vision":
		return "dolbyvision"
	default:
		return ""
	}
}

func nfoRuntimeMinutes(meta *BilingualTitle, media *mediaInfo) int {
	if media != nil && media.DurationSeconds > 0 {
		return int(math.Round(media.DurationSeconds / 60))
	}
	return meta.RuntimeMinutes
}

func nfoPathForVideo(video string) string {
	ext := filepath.Ext(video)
	return strings.TrimSuffix(video, ext) + ".nfo"
}

func writeNFO(path string, doc any, overwrite bool) (bool, error) {
	if doc == nil {
		return false, fmt.Errorf("NFO 内容为空")
	}
	if _, err := os.Stat(path); err == nil && !overwrite {
		return false, nil
	}

	data, err := renderNFO(doc)
	if err != nil {
		return false, err
	}
	return writeNFOData(path, data, overwrite)
}

func renderNFO(doc any) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("NFO 内容为空")
	}
	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("生成 NFO XML 失败：%w", err)
	}
	data := append([]byte(nfoHeader), body...)
	data = append(data, '\n')
	return data, nil
}

func writeNFOData(path string, data []byte, overwrite bool) (bool, error) {
	if _, err := os.Stat(path); err == nil && !overwrite {
		return false, nil
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".icedreel-nfo-*.tmp")
	if err != nil {
		return false, fmt.Errorf("创建临时 NFO 失败：%w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false, fmt.Errorf("写入 NFO 失败：%w", err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("关闭 NFO 临时文件失败：%w", err)
	}
	_ = os.Chmod(tmpName, 0o644)

	if _, err := os.Stat(path); err == nil {
		if !overwrite {
			return false, nil
		}
		if err := os.Remove(path); err != nil {
			return false, fmt.Errorf("覆盖旧 NFO 失败：%w", err)
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return false, fmt.Errorf("写入 NFO 失败：%w", err)
	}
	return true, nil
}

func normalizeNFOText(data []byte) string {
	s := strings.TrimPrefix(string(data), "\uFEFF")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func nfoDiff(existing, next []byte) (string, error) {
	a := difflib.SplitLines(normalizeNFOText(existing))
	b := difflib.SplitLines(normalizeNFOText(next))
	diff := difflib.UnifiedDiff{
		A:        a,
		B:        b,
		FromFile: "现有 NFO",
		ToFile:   "新 NFO",
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(diff)
}
