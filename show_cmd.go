package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
)

func runShow(args []string, stdout, stderr io.Writer) int {
	fs := pflag.NewFlagSet("icedreel show", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.SortFlags = false

	var (
		noPoster    bool
		posterWidth int
		imageMode   string
		showHelp    bool
	)
	fs.BoolVar(&noPoster, "no-poster", false, "不显示海报和 fanart")
	fs.IntVar(&posterWidth, "poster-width", 32, "海报宽度（终端字符数）")
	fs.StringVar(&imageMode, "image", "auto", "海报模式：auto、sixel、half、none")
	fs.BoolVarP(&showHelp, "help", "h", false, "显示帮助")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		fmt.Fprintln(stderr, "运行 icedreel show --help 查看用法。")
		return 2
	}
	if showHelp {
		printShowUsage(stdout)
		return 0
	}
	positional := fs.Args()
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "错误：show 需要且只接受一个 NFO 或视频文件")
		return 2
	}
	posterOpts, err := parsePosterOptions(posterWidth, fs.Changed("poster-width"), imageMode, noPoster)
	if err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		return 2
	}

	file := positional[0]
	nfoPath := file
	if !strings.EqualFold(filepath.Ext(file), ".nfo") {
		if !isVideoFile(file) {
			fmt.Fprintln(stderr, "错误：show 只读 NFO；请传 .nfo 或对应视频文件")
			return 2
		}
		nfoPath = nfoPathForVideo(file)
		if _, err := os.Stat(nfoPath); err != nil {
			fmt.Fprintf(stderr, "错误：找不到同名 NFO：%s；请先运行 scrape\n", nfoPath)
			return 1
		}
	}
	return showNFO(nfoPath, stdout, stderr, posterOpts)
}

func parsePosterOptions(width int, widthSet bool, imageMode string, noPoster bool) (posterOptions, error) {
	if width < 4 || width > 120 {
		return posterOptions{}, fmt.Errorf("--poster-width 应在 4-120 之间")
	}
	mode, err := parsePosterMode(imageMode)
	if err != nil {
		return posterOptions{}, err
	}
	return posterOptions{
		Width:    width,
		WidthSet: widthSet,
		Mode:     mode,
		NoPoster: noPoster,
	}, nil
}

func showNFO(path string, stdout, stderr io.Writer, posterOpts posterOptions) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "错误：读取 NFO 失败：%v\n", err)
		return 1
	}
	var root struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(data, &root); err != nil {
		fmt.Fprintf(stderr, "错误：解析 NFO 失败：%v\n", err)
		return 1
	}

	switch root.XMLName.Local {
	case "movie":
		var doc nfoMovie
		if err := xml.Unmarshal(data, &doc); err != nil {
			fmt.Fprintf(stderr, "错误：解析 NFO 失败：%v\n", err)
			return 1
		}
		return showMovieNFO(path, &doc, stdout, stderr, posterOpts)
	case "episodedetails":
		var doc nfoEpisode
		if err := xml.Unmarshal(data, &doc); err != nil {
			fmt.Fprintf(stderr, "错误：解析 NFO 失败：%v\n", err)
			return 1
		}
		return showEpisodeNFO(path, &doc, stdout, stderr, posterOpts)
	case "tvshow":
		var doc nfoTVShow
		if err := xml.Unmarshal(data, &doc); err != nil {
			fmt.Fprintf(stderr, "错误：解析 NFO 失败：%v\n", err)
			return 1
		}
		return showTVShowNFO(path, &doc, stdout, stderr, posterOpts)
	case "season":
		var doc nfoSeason
		if err := xml.Unmarshal(data, &doc); err != nil {
			fmt.Fprintf(stderr, "错误：解析 NFO 失败：%v\n", err)
			return 1
		}
		return showSeasonNFO(path, &doc, stdout, stderr, posterOpts)
	default:
		fmt.Fprintf(stderr, "错误：不认识的 NFO 根元素：%s\n", root.XMLName.Local)
		return 1
	}
}

func showMovieNFO(path string, doc *nfoMovie, stdout, stderr io.Writer, posterOpts posterOptions) int {
	if doc.Title == "" && doc.OriginalTitle == "" {
		fmt.Fprintln(stderr, "错误：NFO 中没有标题")
		return 1
	}
	if isTerminal(os.Stdout) {
		showPosterPair(stdout, nfoPosterSource(doc, path), nfoFanartSource(doc, path), posterOpts, stderr)
	}
	printNFOReport(stdout, path, doc)
	return 0
}

func showEpisodeNFO(path string, doc *nfoEpisode, stdout, stderr io.Writer, posterOpts posterOptions) int {
	showDoc, showPath := loadSiblingTVShow(path)
	return printEpisodeView(path, doc, showDoc, showPath, stdout, stderr, posterOpts)
}

func printEpisodeView(path string, doc *nfoEpisode, showDoc *nfoTVShow, showPath string, stdout, stderr io.Writer, posterOpts posterOptions) int {
	if doc.Title == "" && doc.ShowTitle == "" {
		fmt.Fprintln(stderr, "错误：NFO 中没有标题")
		return 1
	}
	if isTerminal(os.Stdout) {
		showPosterPair(stdout, nfoEpisodePosterSource(doc, path, showDoc, showPath), nfoTVShowFanartSource(showDoc, showPath), posterOpts, stderr)
	}
	printEpisodeReport(stdout, path, doc, showDoc)
	return 0
}

func showTVShowNFO(path string, doc *nfoTVShow, stdout, stderr io.Writer, posterOpts posterOptions) int {
	if doc.Title == "" && doc.OriginalTitle == "" {
		fmt.Fprintln(stderr, "错误：NFO 中没有标题")
		return 1
	}
	if isTerminal(os.Stdout) {
		showPosterPair(stdout, nfoTVShowPosterSource(doc, path), nfoTVShowFanartSource(doc, path), posterOpts, stderr)
	}
	printTVShowReport(stdout, path, doc)
	return 0
}

func showSeasonNFO(path string, doc *nfoSeason, stdout, stderr io.Writer, posterOpts posterOptions) int {
	showDoc, showPath := loadSiblingTVShow(path)
	return printSeasonView(path, doc, showDoc, showPath, stdout, stderr, posterOpts)
}

func printSeasonView(path string, doc *nfoSeason, showDoc *nfoTVShow, showPath string, stdout, stderr io.Writer, posterOpts posterOptions) int {
	if isTerminal(os.Stdout) {
		showPosterPair(stdout, nfoSeasonPosterSource(doc, path, showDoc, showPath), nfoTVShowFanartSource(showDoc, showPath), posterOpts, stderr)
	}
	printSeasonReport(stdout, path, doc, showDoc)
	return 0
}

func nfoPosterSource(doc *nfoMovie, path string) string {
	if source := posterSource(nil, path); source != "" {
		return source
	}
	firstThumb := ""
	plainThumb := ""
	for _, thumb := range doc.Thumbs {
		if thumb.URL == "" {
			continue
		}
		if firstThumb == "" {
			firstThumb = thumb.URL
		}
		switch {
		case strings.EqualFold(thumb.Aspect, "poster"):
			return tmdbW500URL(thumb.URL)
		case thumb.Aspect == "" && plainThumb == "":
			plainThumb = tmdbW500URL(thumb.URL)
		}
	}
	if plainThumb != "" {
		return plainThumb
	}
	return tmdbW500URL(firstThumb)
}

func localFanartSource(path string) string {
	dir := filepath.Dir(path)
	for _, name := range []string{"fanart.jpg", "fanart.png", "backdrop.jpg", "backdrop.png"} {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func nfoFanartSource(doc *nfoMovie, path string) string {
	if local := localFanartSource(path); local != "" {
		return local
	}
	if doc.Fanart == nil || doc.Fanart.Thumb == "" {
		return ""
	}
	return tmdbW500URL(doc.Fanart.Thumb)
}

func nfoTVShowPosterSource(doc *nfoTVShow, path string) string {
	if source := posterSource(nil, path); source != "" {
		return source
	}
	firstThumb := ""
	plainThumb := ""
	for _, thumb := range doc.Thumbs {
		if thumb.URL == "" {
			continue
		}
		if firstThumb == "" {
			firstThumb = thumb.URL
		}
		switch {
		case strings.EqualFold(thumb.Aspect, "poster"):
			return tmdbW500URL(thumb.URL)
		case thumb.Aspect == "" && plainThumb == "":
			plainThumb = tmdbW500URL(thumb.URL)
		}
	}
	if plainThumb != "" {
		return plainThumb
	}
	return tmdbW500URL(firstThumb)
}

func nfoTVShowFanartSource(doc *nfoTVShow, path string) string {
	if local := localFanartSource(path); local != "" {
		return local
	}
	if doc == nil || doc.Fanart == nil || doc.Fanart.Thumb == "" {
		return ""
	}
	return tmdbW500URL(doc.Fanart.Thumb)
}

func nfoEpisodePosterSource(doc *nfoEpisode, path string, show *nfoTVShow, showPath string) string {
	if source := posterSource(nil, path); source != "" {
		return source
	}
	if doc.Thumb != "" {
		return tmdbW500URL(doc.Thumb)
	}
	if show != nil {
		return nfoTVShowPosterSource(show, showPath)
	}
	return ""
}

func nfoSeasonPosterSource(doc *nfoSeason, path string, show *nfoTVShow, showPath string) string {
	if source := posterSource(nil, path); source != "" {
		return source
	}
	if doc != nil {
		firstThumb := ""
		for _, thumb := range doc.Thumbs {
			if thumb.URL == "" {
				continue
			}
			if firstThumb == "" {
				firstThumb = thumb.URL
			}
			if strings.EqualFold(thumb.Aspect, "poster") {
				return tmdbW500URL(thumb.URL)
			}
		}
		if firstThumb != "" {
			return tmdbW500URL(firstThumb)
		}
	}
	if show != nil {
		return nfoTVShowPosterSource(show, showPath)
	}
	return ""
}

func loadSiblingTVShow(path string) (*nfoTVShow, string) {
	dir := filepath.Dir(path)
	dirs := []string{dir, filepath.Dir(dir)}
	seen := make(map[string]bool)
	for _, d := range dirs {
		candidates := []string{filepath.Join(d, "tvshow.nfo")}
		if matches, err := filepath.Glob(filepath.Join(d, "*.nfo")); err == nil {
			candidates = append(candidates, matches...)
		}
		for _, candidate := range candidates {
			if candidate == path || seen[candidate] {
				continue
			}
			seen[candidate] = true
			data, err := os.ReadFile(candidate)
			if err != nil {
				continue
			}
			var root struct {
				XMLName xml.Name
			}
			if err := xml.Unmarshal(data, &root); err != nil || root.XMLName.Local != "tvshow" {
				continue
			}
			var doc nfoTVShow
			if err := xml.Unmarshal(data, &doc); err != nil {
				continue
			}
			return &doc, candidate
		}
	}
	return nil, ""
}

func nfoTVShowIDs(doc *nfoTVShow) (imdb string, tmdbID int, douban string) {
	for _, uid := range doc.UniqueIDs {
		switch strings.ToLower(uid.Type) {
		case "imdb":
			imdb = uid.Value
		case "tmdb":
			tmdbID, _ = strconv.Atoi(uid.Value)
		case "douban":
			douban = uid.Value
		}
	}
	if imdb == "" {
		imdb = doc.ImdbID
	}
	if tmdbID == 0 {
		tmdbID = doc.TmdbID
	}
	if douban == "" {
		douban = doc.DoubanID
	}
	return imdb, tmdbID, douban
}

func tmdbW500URL(raw string) string {
	const originalBase = "https://image.tmdb.org/t/p/original"
	if strings.HasPrefix(raw, originalBase+"/") {
		return tmdbPosterBase + strings.TrimPrefix(raw, originalBase)
	}
	return raw
}

func printNFOReport(w io.Writer, path string, doc *nfoMovie) {
	title := doc.Title
	if title == "" {
		title = doc.OriginalTitle
	}
	fmt.Fprintf(w, "文件:   %s\n", filepath.Base(path))
	if doc.LogicalName != "" {
		fmt.Fprintf(w, "逻辑名: %s\n", doc.LogicalName)
	}
	fmt.Fprintf(w, "标题:   %s\n", title)
	if doc.OriginalTitle != "" && doc.OriginalTitle != title {
		fmt.Fprintf(w, "原名:   %s\n", doc.OriginalTitle)
	}
	if doc.Year != "" {
		fmt.Fprintf(w, "年份:   %s\n", doc.Year)
	}
	if doc.Runtime > 0 && !nfoHasVideoDuration(doc) {
		fmt.Fprintf(w, "时长:   %dmin\n", doc.Runtime)
	}

	imdb, tmdbID, douban := nfoIDs(doc)
	isTV := strings.EqualFold(doc.TmdbType, "tv")
	links := metadataLinks(imdb, tmdbID, isTV, douban)
	if doc.WikidataID != "" {
		links = append(links, metadataLink{
			Label: "wikidata",
			URL:   "https://www.wikidata.org/wiki/" + doc.WikidataID,
		})
	}
	printMetadataLinks(w, links, terminalHyperlinksEnabled(w))
	printNFORating(w, doc.Rating, doc.Votes)
	if len(doc.Genres) > 0 {
		fmt.Fprintf(w, "类型:   %s\n", strings.Join(doc.Genres, " / "))
	}
	if len(doc.Studios) > 0 {
		fmt.Fprintf(w, "制片:   %s\n", strings.Join(doc.Studios, " / "))
	}
	if len(doc.Countries) > 0 {
		fmt.Fprintf(w, "国家:   %s\n", strings.Join(doc.Countries, " / "))
	}
	if doc.Plot != "" {
		fmt.Fprintf(w, "简介:   %s\n", doc.Plot)
	}
	printNFOStreams(w, doc.StreamDetails)
}

func episodeLabel(season, episode int) string {
	return fmt.Sprintf("S%02dE%02d", season, episode)
}

func printEpisodeReport(w io.Writer, path string, doc *nfoEpisode, show *nfoTVShow) {
	showTitle := doc.ShowTitle
	if showTitle == "" && show != nil {
		showTitle = show.Title
	}
	fmt.Fprintf(w, "文件:   %s\n", filepath.Base(path))
	if doc.LogicalName != "" {
		fmt.Fprintf(w, "逻辑名: %s\n", doc.LogicalName)
	}
	if showTitle != "" {
		fmt.Fprintf(w, "剧名:   %s\n", showTitle)
	}
	fmt.Fprintf(w, "季集:   %s\n", episodeLabel(doc.Season, doc.Episode))
	if doc.Title != "" {
		fmt.Fprintf(w, "单集:   %s\n", doc.Title)
	}
	if doc.Aired != "" {
		fmt.Fprintf(w, "播出:   %s\n", doc.Aired)
	}
	if doc.Runtime > 0 && !nfoStreamHasDuration(doc.StreamDetails) {
		fmt.Fprintf(w, "时长:   %dmin\n", doc.Runtime)
	}

	if show != nil {
		imdb, tmdbID, douban := nfoTVShowIDs(show)
		links := metadataLinks(imdb, tmdbID, true, douban)
		if tmdbID > 0 {
			links = append(links, metadataLink{
				Label: "episode",
				URL:   fmt.Sprintf("https://www.themoviedb.org/tv/%d/season/%d/episode/%d", tmdbID, doc.Season, doc.Episode),
			})
		}
		printMetadataLinks(w, links, terminalHyperlinksEnabled(w))
	}

	printNFORating(w, doc.Rating, doc.Votes)
	if doc.Plot != "" {
		fmt.Fprintf(w, "简介:   %s\n", doc.Plot)
	}
	printNFOStreams(w, doc.StreamDetails)
}

func printTVShowReport(w io.Writer, path string, doc *nfoTVShow) {
	title := doc.Title
	if title == "" {
		title = doc.OriginalTitle
	}
	fmt.Fprintf(w, "文件:   %s\n", filepath.Base(path))
	if doc.LogicalName != "" {
		fmt.Fprintf(w, "逻辑名: %s\n", doc.LogicalName)
	}
	fmt.Fprintf(w, "剧名:   %s\n", title)
	if doc.OriginalTitle != "" && doc.OriginalTitle != title {
		fmt.Fprintf(w, "原名:   %s\n", doc.OriginalTitle)
	}
	if doc.Year != "" {
		fmt.Fprintf(w, "年份:   %s\n", doc.Year)
	}

	imdb, tmdbID, douban := nfoTVShowIDs(doc)
	links := metadataLinks(imdb, tmdbID, true, douban)
	printMetadataLinks(w, links, terminalHyperlinksEnabled(w))
	printNFORating(w, doc.Rating, doc.Votes)
	if len(doc.Genres) > 0 {
		fmt.Fprintf(w, "类型:   %s\n", strings.Join(doc.Genres, " / "))
	}
	if len(doc.Studios) > 0 {
		fmt.Fprintf(w, "制片:   %s\n", strings.Join(doc.Studios, " / "))
	}
	if len(doc.Countries) > 0 {
		fmt.Fprintf(w, "国家:   %s\n", strings.Join(doc.Countries, " / "))
	}
	if doc.Plot != "" {
		fmt.Fprintf(w, "简介:   %s\n", doc.Plot)
	}
}

func printSeasonReport(w io.Writer, path string, doc *nfoSeason, show *nfoTVShow) {
	fmt.Fprintf(w, "文件:   %s\n", filepath.Base(path))
	if doc.LogicalName != "" {
		fmt.Fprintf(w, "逻辑名: %s\n", doc.LogicalName)
	}
	if doc.Title != "" {
		fmt.Fprintf(w, "季名:   %s\n", doc.Title)
	}
	if doc.SeasonNumber > 0 {
		fmt.Fprintf(w, "季号:   %d\n", doc.SeasonNumber)
	}
	if doc.Premiered != "" {
		fmt.Fprintf(w, "首播:   %s\n", doc.Premiered)
	} else if doc.ReleaseDate != "" {
		fmt.Fprintf(w, "首播:   %s\n", doc.ReleaseDate)
	}
	if doc.Year != "" {
		fmt.Fprintf(w, "年份:   %s\n", doc.Year)
	}

	var links []metadataLink
	if show != nil {
		imdb, tmdbID, douban := nfoTVShowIDs(show)
		links = metadataLinks(imdb, tmdbID, true, douban)
		if tmdbID > 0 && doc.SeasonNumber > 0 {
			links = append(links, metadataLink{
				Label: "season",
				URL:   fmt.Sprintf("https://www.themoviedb.org/tv/%d/season/%d", tmdbID, doc.SeasonNumber),
			})
		}
	}
	if doc.DoubanID != "" {
		links = append(links, metadataLink{
			Label: "douban",
			URL:   "https://movie.douban.com/subject/" + doc.DoubanID + "/",
		})
	}
	printMetadataLinks(w, links, terminalHyperlinksEnabled(w))

	plot := doc.Plot
	if plot == "" {
		plot = doc.Outline
	}
	if plot != "" {
		fmt.Fprintf(w, "简介:   %s\n", plot)
	}
}

func printNFORating(w io.Writer, rating float64, votes int) {
	if rating <= 0 && votes <= 0 {
		return
	}
	if votes > 0 {
		fmt.Fprintf(w, "评分:   %.1f (%d)\n", rating, votes)
		return
	}
	fmt.Fprintf(w, "评分:   %.1f\n", rating)
}

func nfoStreamHasDuration(sd *nfoStreamDetails) bool {
	return sd != nil && sd.Video != nil && sd.Video.DurationSeconds > 0
}

func nfoHasVideoDuration(doc *nfoMovie) bool {
	return nfoStreamHasDuration(doc.StreamDetails)
}

func nfoIDs(doc *nfoMovie) (imdb string, tmdbID int, douban string) {
	for _, uid := range doc.UniqueIDs {
		switch strings.ToLower(uid.Type) {
		case "imdb":
			imdb = uid.Value
		case "tmdb":
			tmdbID, _ = strconv.Atoi(uid.Value)
		case "douban":
			douban = uid.Value
		}
	}
	if imdb == "" {
		imdb = doc.ImdbID
	}
	if tmdbID == 0 {
		tmdbID = doc.TmdbID
	}
	if douban == "" {
		douban = doc.DoubanID
	}
	return imdb, tmdbID, douban
}

func printNFOStreams(w io.Writer, sd *nfoStreamDetails) {
	if sd == nil {
		return
	}
	if sd.Video != nil {
		parts := []string{}
		if sd.Video.Width > 0 && sd.Video.Height > 0 {
			parts = append(parts, fmt.Sprintf("%dx%d", sd.Video.Width, sd.Video.Height))
		}
		if sd.Video.Codec != "" {
			parts = append(parts, videoCodecLabel(sd.Video.Codec))
		}
		if sd.Video.HDRType != "" {
			parts = append(parts, nfoHDRLabel(sd.Video.HDRType))
		}
		if sd.Video.DurationSeconds > 0 {
			clock, minutes := formatDurationHuman(float64(sd.Video.DurationSeconds))
			parts = append(parts, fmt.Sprintf("%s (%s)", clock, minutes))
		}
		if len(parts) > 0 {
			fmt.Fprintf(w, "视频:   %s\n", strings.Join(parts, " "))
		}
	}

	if len(sd.Audio) == 0 {
		fmt.Fprintln(w, "音轨:   无")
	} else {
		fmt.Fprintln(w, "音轨:")
		for i, a := range sd.Audio {
			fmt.Fprintf(w, "  %s\n", formatNFOAudioLine(a, i+1))
		}
	}

	if len(sd.Subtitles) == 0 {
		fmt.Fprintln(w, "字幕:   无")
	} else {
		fmt.Fprintln(w, "字幕:")
		for _, s := range sd.Subtitles {
			fmt.Fprintf(w, "  %s\n", formatNFOSubtitleLine(s))
		}
	}
}

func streamDisplayName(language, title string) string {
	lang := normalizeLang(language)
	english := languageEnglishLabel(lang)
	local := strings.TrimSpace(title)
	if local == "" || strings.EqualFold(local, english) {
		local = languageDisplayLabel(lang)
	}
	if lang == "und" {
		if local == "" {
			return "未知"
		}
		return local
	}
	if local == "" || strings.EqualFold(local, english) {
		return english
	}
	return english + ", " + local
}

func formatNFOSubtitleLine(s nfoSubtitle) string {
	return streamDisplayName(s.Language, s.Title)
}

func formatNFOAudioLine(a nfoAudio, number int) string {
	name := streamDisplayName(a.Language, a.Title)

	var parts []string
	if codec := nfoAudioCodecLabel(a.Codec); codec != "" {
		parts = append(parts, codec)
	}
	if a.SampleRate > 0 {
		parts = append(parts, fmt.Sprintf("%.1f kHz", float64(a.SampleRate)/1000))
	}
	if ch := channelsLabel("", a.Channels); ch != "" && ch != "-" {
		if strings.HasSuffix(ch, "ch") {
			ch = strings.TrimSuffix(ch, "ch") + " chn"
		} else {
			ch += " chn"
		}
		parts = append(parts, ch)
	}
	if a.BitDepth > 0 {
		parts = append(parts, fmt.Sprintf("%d bit", a.BitDepth))
	}

	line := fmt.Sprintf("%s (Audio %d)", name, number)
	if a.Default {
		line += " [Default]"
	}
	if len(parts) > 0 {
		line += " (" + strings.Join(parts, ", ") + ")"
	}
	return line
}

func nfoAudioCodecLabel(codec string) string {
	codec = strings.ToLower(strings.TrimSpace(codec))
	switch codec {
	case "dtshd_ma":
		return "DTS-HD MA"
	case "dtshd_hra":
		return "DTS-HD HRA"
	case "dts":
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
	case "vorbis":
		return "Vorbis"
	case "mp3":
		return "MP3"
	}
	if strings.HasPrefix(codec, "pcm") {
		return "PCM"
	}
	if codec == "" {
		return "未知"
	}
	return strings.ToUpper(codec)
}

func nfoHDRLabel(hdr string) string {
	switch strings.ToLower(hdr) {
	case "hdr10":
		return "HDR10"
	case "hdr10plus":
		return "HDR10+"
	case "dolbyvision":
		return "Dolby Vision"
	case "hlg":
		return "HLG"
	default:
		return hdr
	}
}

func printShowUsage(w io.Writer) {
	fmt.Fprint(w, `用法：
  icedreel show [选项] <NFO 或视频>

显示 NFO 元数据，支持 movie / episodedetails / tvshow；传视频时读同名 .nfo，不查 TMDB、不跑 ffprobe。TTY 下并排显示 poster 和 fanart，远程图可能下载。

选项：
  --no-poster          不显示海报和 fanart
  --poster-width <n>   海报宽度，默认 32（4-120）；并排或 Sixel 未指定时用 60
  --image <mode>       海报模式：auto（默认）、sixel、half、none
  -h, --help           显示帮助

示例：
  icedreel show "D:\Movies\Movie.nfo"
  icedreel show "D:\Movies\Movie.mkv"
  icedreel show "D:\Movies\Movie.nfo" --image sixel --poster-width 60
  icedreel show "D:\Movies\Movie.nfo" --no-poster
`)
}
