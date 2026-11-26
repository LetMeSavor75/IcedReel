package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

type mediaInfo struct {
	DurationSeconds float64         `json:"duration_seconds"`
	SizeBytes       int64           `json:"size_bytes"`
	BitRate         int64           `json:"bit_rate"`
	FormatName      string          `json:"format_name"`
	Video           *videoInfo      `json:"video,omitempty"`
	Audio           []audioTrack    `json:"audio"`
	Subtitles       []subtitleTrack `json:"subtitles"`
}

type videoInfo struct {
	Codec    string  `json:"codec"`
	CodecRaw string  `json:"codec_raw"`
	Profile  string  `json:"profile,omitempty"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	PixFmt   string  `json:"pix_fmt"`
	BitDepth int     `json:"bit_depth"`
	HDR      string  `json:"hdr"`
	FPS      float64 `json:"fps"`
}

type audioTrack struct {
	Index         int    `json:"index"`
	Language      string `json:"language"`
	Codec         string `json:"codec"`
	CodecRaw      string `json:"codec_raw"`
	Title         string `json:"title,omitempty"`
	Channels      string `json:"channels"`
	ChannelsCount int    `json:"channels_count"`
	SampleRate    int    `json:"sample_rate"`
	BitDepth      int    `json:"bit_depth"`
	BitRate       int64  `json:"bit_rate"`
	Default       bool   `json:"default"`
}

type subtitleTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Format   string `json:"format"`
	CodecRaw string `json:"codec_raw"`
	Title    string `json:"title,omitempty"`
	Default  bool   `json:"default"`
	Forced   bool   `json:"forced"`
}

type ffprobeDoc struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	Index            int               `json:"index"`
	CodecName        string            `json:"codec_name"`
	CodecType        string            `json:"codec_type"`
	Profile          string            `json:"profile"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	PixFmt           string            `json:"pix_fmt"`
	BitsPerRawSample any               `json:"bits_per_raw_sample"`
	BitsPerSample    any               `json:"bits_per_sample"`
	ColorTransfer    string            `json:"color_transfer"`
	ColorPrimaries   string            `json:"color_primaries"`
	ColorSpace       string            `json:"color_space"`
	AvgFrameRate     string            `json:"avg_frame_rate"`
	Duration         string            `json:"duration"`
	Channels         int               `json:"channels"`
	ChannelLayout    string            `json:"channel_layout"`
	SampleRate       string            `json:"sample_rate"`
	BitRate          string            `json:"bit_rate"`
	Tags             map[string]string `json:"tags"`
	Disposition      struct {
		Default     int `json:"default"`
		Forced      int `json:"forced"`
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
	SideDataList []struct {
		SideDataType string `json:"side_data_type"`
	} `json:"side_data_list"`
}

type ffprobeFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
	BitRate    string `json:"bit_rate"`
}

func probeMedia(ctx context.Context, ffprobe, file string) (*mediaInfo, error) {
	cmd := exec.CommandContext(ctx, ffprobe,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		file,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe 失败：%v：%s", err, strings.TrimSpace(stderr.String()))
	}
	return parseProbeJSON(out)
}

func parseProbeJSON(data []byte) (*mediaInfo, error) {
	var doc ffprobeDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("ffprobe JSON 解析失败：%w", err)
	}

	info := &mediaInfo{
		DurationSeconds: parseFloat(doc.Format.Duration),
		SizeBytes:       parseInt64(doc.Format.Size),
		BitRate:         parseInt64(doc.Format.BitRate),
		FormatName:      doc.Format.FormatName,
	}

	for _, s := range doc.Streams {
		switch s.CodecType {
		case "video":
			if s.Disposition.AttachedPic != 0 || info.Video != nil {
				continue
			}
			info.Video = &videoInfo{
				Codec:    videoCodecLabel(s.CodecName),
				CodecRaw: strings.ToLower(s.CodecName),
				Profile:  strings.TrimSpace(s.Profile),
				Width:    s.Width,
				Height:   s.Height,
				PixFmt:   s.PixFmt,
				BitDepth: videoBitDepth(s),
				HDR:      hdrLabel(s),
				FPS:      parseFrameRate(s.AvgFrameRate),
			}
			if info.DurationSeconds <= 0 {
				info.DurationSeconds = parseFloat(s.Duration)
			}
		case "audio":
			info.Audio = append(info.Audio, audioTrack{
				Index:         s.Index,
				Language:      normalizeLang(tagValue(s.Tags, "language")),
				Codec:         audioCodecLabel(s.CodecName, s.Profile),
				CodecRaw:      strings.ToLower(s.CodecName),
				Title:         strings.TrimSpace(tagValue(s.Tags, "title")),
				Channels:      channelsLabel(s.ChannelLayout, s.Channels),
				ChannelsCount: s.Channels,
				SampleRate:    parseInt(s.SampleRate),
				BitDepth:      audioBitDepth(s),
				BitRate:       parseInt64(s.BitRate),
				Default:       s.Disposition.Default != 0,
			})
		case "subtitle":
			info.Subtitles = append(info.Subtitles, subtitleTrack{
				Index:    s.Index,
				Language: normalizeLang(tagValue(s.Tags, "language")),
				Format:   subtitleCodecLabel(s.CodecName),
				CodecRaw: strings.ToLower(s.CodecName),
				Title:    strings.TrimSpace(tagValue(s.Tags, "title")),
				Default:  s.Disposition.Default != 0,
				Forced:   s.Disposition.Forced != 0,
			})
		}
	}
	return info, nil
}

func tagValue(tags map[string]string, key string) string {
	if tags == nil {
		return ""
	}
	return tags[key]
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}

func parseFrameRate(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0/0" {
		return 0
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return parseFloat(s)
	}
	num := parseFloat(parts[0])
	den := parseFloat(parts[1])
	if den == 0 {
		return 0
	}
	return num / den
}

func jsonInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case string:
		return parseInt(n)
	default:
		return 0
	}
}

func audioBitDepth(s ffprobeStream) int {
	if v := jsonInt(s.BitsPerRawSample); v > 0 {
		return v
	}
	return jsonInt(s.BitsPerSample)
}

func videoBitDepth(s ffprobeStream) int {
	if v := jsonInt(s.BitsPerRawSample); v > 0 {
		return v
	}
	p := strings.ToLower(s.PixFmt)
	switch {
	case strings.Contains(p, "p010"), strings.Contains(p, "10le"), strings.Contains(p, "10be"):
		return 10
	case strings.Contains(p, "p012"), strings.Contains(p, "12le"), strings.Contains(p, "12be"):
		return 12
	case strings.Contains(p, "p016"), strings.Contains(p, "16le"), strings.Contains(p, "16be"):
		return 16
	default:
		return 8
	}
}

func hdrLabel(s ffprobeStream) string {
	for _, sd := range s.SideDataList {
		t := strings.ToLower(sd.SideDataType)
		switch {
		case strings.Contains(t, "dovi"), strings.Contains(t, "dolby vision"):
			return "Dolby Vision"
		case strings.Contains(t, "hdr10+"):
			return "HDR10+"
		}
	}
	switch strings.ToLower(strings.TrimSpace(s.ColorTransfer)) {
	case "smpte2084":
		return "HDR10"
	case "arib-std-b67":
		return "HLG"
	default:
		return "SDR"
	}
}

func normalizeLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "und" || s == "unknown" {
		return "und"
	}
	s = strings.ReplaceAll(s, "_", "-")
	base := strings.SplitN(s, "-", 2)[0]
	switch base {
	case "eng":
		return "en"
	case "ita":
		return "it"
	case "chi", "zho", "cmn", "zh":
		return "zh"
	case "jpn":
		return "ja"
	case "kor":
		return "ko"
	case "fra", "fre":
		return "fr"
	case "deu", "ger":
		return "de"
	case "spa":
		return "es"
	case "rus":
		return "ru"
	case "por":
		return "pt"
	case "nld", "dut":
		return "nl"
	case "swe":
		return "sv"
	case "nor":
		return "no"
	case "dan":
		return "da"
	case "fin":
		return "fi"
	case "pol":
		return "pl"
	case "tur":
		return "tr"
	case "ara":
		return "ar"
	case "hin":
		return "hi"
	case "tha":
		return "th"
	case "vie":
		return "vi"
	}
	return base
}

func videoCodecLabel(codec string) string {
	switch strings.ToLower(codec) {
	case "hevc", "h265":
		return "HEVC"
	case "h264", "avc":
		return "H.264"
	case "av1":
		return "AV1"
	case "vp9":
		return "VP9"
	case "mpeg2", "mpeg2video":
		return "MPEG-2"
	case "vc1":
		return "VC-1"
	default:
		return strings.ToUpper(codec)
	}
}

func languageDisplayLabel(code string) string {
	switch normalizeLang(code) {
	case "en":
		return "英语"
	case "zh":
		return "中文"
	case "it":
		return "意大利语"
	case "ja":
		return "日语"
	case "ko":
		return "韩语"
	case "fr":
		return "法语"
	case "de":
		return "德语"
	case "es":
		return "西班牙语"
	case "ru":
		return "俄语"
	case "pt":
		return "葡萄牙语"
	case "nl":
		return "荷兰语"
	case "sv":
		return "瑞典语"
	case "no":
		return "挪威语"
	case "da":
		return "丹麦语"
	case "fi":
		return "芬兰语"
	case "pl":
		return "波兰语"
	case "tr":
		return "土耳其语"
	case "ar":
		return "阿拉伯语"
	case "hi":
		return "印地语"
	case "th":
		return "泰语"
	case "vi":
		return "越南语"
	case "und":
		return "未知"
	default:
		return code
	}
}

func languageEnglishLabel(code string) string {
	switch normalizeLang(code) {
	case "en":
		return "English"
	case "zh":
		return "Chinese"
	case "it":
		return "Italian"
	case "ja":
		return "Japanese"
	case "ko":
		return "Korean"
	case "fr":
		return "French"
	case "de":
		return "German"
	case "es":
		return "Spanish"
	case "ru":
		return "Russian"
	case "pt":
		return "Portuguese"
	case "nl":
		return "Dutch"
	case "sv":
		return "Swedish"
	case "no":
		return "Norwegian"
	case "da":
		return "Danish"
	case "fi":
		return "Finnish"
	case "pl":
		return "Polish"
	case "tr":
		return "Turkish"
	case "ar":
		return "Arabic"
	case "hi":
		return "Hindi"
	case "th":
		return "Thai"
	case "vi":
		return "Vietnamese"
	case "und":
		return "Unknown"
	default:
		return strings.ToUpper(code)
	}
}

func audioCodecLabel(codec, profile string) string {
	codec = strings.ToLower(codec)
	profile = strings.TrimSpace(profile)
	switch codec {
	case "dts":
		if profile != "" {
			return profile
		}
		return "DTS"
	case "truehd":
		return "TrueHD"
	case "ac3":
		return "DD"
	case "eac3":
		return "DD+"
	case "aac":
		return "AAC"
	case "flac":
		return "FLAC"
	case "opus":
		return "Opus"
	case "mp3":
		return "MP3"
	}
	if strings.HasPrefix(codec, "pcm_") {
		return "PCM"
	}
	return strings.ToUpper(codec)
}

func subtitleCodecLabel(codec string) string {
	switch strings.ToLower(codec) {
	case "hdmv_pgs_subtitle":
		return "PGS"
	case "subrip":
		return "SRT"
	case "ass", "ssa":
		return "ASS"
	case "mov_text":
		return "MOV_TEXT"
	case "webvtt":
		return "WebVTT"
	case "dvd_subtitle":
		return "VOBSUB"
	default:
		return strings.ToUpper(codec)
	}
}

func channelsLabel(layout string, channels int) string {
	l := strings.ToLower(layout)
	switch {
	case strings.Contains(l, "stereo"):
		return "2.0"
	case strings.Contains(l, "mono"):
		return "1.0"
	case strings.Contains(l, "5.1"):
		return "5.1"
	case strings.Contains(l, "7.1"):
		return "7.1"
	}
	switch channels {
	case 1:
		return "1.0"
	case 2:
		return "2.0"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	case 0:
		return "-"
	default:
		return fmt.Sprintf("%dch", channels)
	}
}

func formatDurationHuman(seconds float64) (string, string) {
	total := int(math.Round(seconds))
	if total <= 0 {
		return "-", ""
	}
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d:%02d", h, m, s), fmt.Sprintf("%d min", int(math.Round(float64(total)/60)))
}

func formatBytes(n int64) string {
	if n <= 0 {
		return "-"
	}
	const unit = 1024.0
	value := float64(n)
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	i := 0
	for value >= unit && i < len(units)-1 {
		value /= unit
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[i])
	}
	return fmt.Sprintf("%.2f %s", value, units[i])
}

func formatBitrate(bps int64) string {
	if bps <= 0 {
		return "-"
	}
	if bps >= 1_000_000_000 {
		return fmt.Sprintf("%.2f Gbps", float64(bps)/1_000_000_000)
	}
	if bps >= 1_000_000 {
		return fmt.Sprintf("%.2f Mbps", float64(bps)/1_000_000)
	}
	if bps >= 1_000 {
		return fmt.Sprintf("%.2f Kbps", float64(bps)/1_000)
	}
	return fmt.Sprintf("%d bps", bps)
}
