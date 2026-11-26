package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

func runScrape(args []string, stdout, stderr io.Writer) int {
	fs := pflag.NewFlagSet("icedreel scrape", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.SortFlags = false

	var (
		imdbFlag    string
		doubanFlag  string
		ffprobePath string
		showHelp    bool
	)
	fs.StringVar(&imdbFlag, "imdb", "", "IMDb id 或 URL")
	fs.StringVar(&doubanFlag, "douban", "", "豆瓣电影 id")
	fs.StringVar(&ffprobePath, "ffprobe", "", "ffprobe.exe 路径")
	fs.BoolVarP(&showHelp, "help", "h", false, "显示帮助")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		fmt.Fprintln(stderr, "运行 icedreel scrape --help 查看用法。")
		return 2
	}
	if showHelp {
		printScrapeUsage(stdout)
		return 0
	}

	positional := fs.Args()
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "错误：scrape 需要且只接受一个视频文件")
		return 2
	}
	file := positional[0]
	info, err := os.Stat(file)
	if err != nil {
		fmt.Fprintf(stderr, "错误：文件不可用：%s：%v\n", file, err)
		return 2
	}
	if info.IsDir() {
		fmt.Fprintln(stderr, "错误：scrape 需要视频文件，不接受目录")
		return 2
	}
	if !isVideoFile(file) {
		if strings.EqualFold(filepath.Ext(file), ".iso") {
			fmt.Fprintln(stderr, "错误：v1 不直接探测 ISO；请先挂载或用 bluray: 协议")
		} else {
			fmt.Fprintf(stderr, "错误：不认识的视频扩展名：%s\n", filepath.Ext(file))
		}
		return 2
	}

	imdb := NormalizeImdbID(imdbFlag)
	if imdbFlag != "" && imdb == "" {
		fmt.Fprintf(stderr, "错误：IMDb id 无效：%s\n", imdbFlag)
		return 2
	}
	douban := onlyDigits(doubanFlag)
	if doubanFlag != "" && douban == "" {
		fmt.Fprintf(stderr, "错误：豆瓣 id 无效：%s\n", doubanFlag)
		return 2
	}

	interactive := isTerminal(os.Stdin) && isTerminal(os.Stdout)
	m := &menu{reader: bufio.NewReader(os.Stdin), out: stdout}
	if imdb == "" && douban == "" {
		if !interactive {
			fmt.Fprintln(stderr, "错误：至少提供一个 IMDb id 或豆瓣 id")
			return 2
		}
		imdb, douban = promptMissingIDs(m, imdb, douban)
		if imdb == "" && douban == "" {
			fmt.Fprintln(stderr, "错误：至少提供一个有效的 IMDb id 或豆瓣 id")
			return 2
		}
	}

	ffprobe, err := findFFprobe(ffprobePath)
	if err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	wd := newWikidataClient()
	tmdb := newTMDBClient(loadTMDBConfig(), wd)

	var meta *BilingualTitle
	err = retryTMDB(m, interactive, stderr, func() error {
		var err error
		meta, err = resolveScrapeMetadata(ctx, tmdb, wd, imdb, douban)
		return err
	})
	if err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		if strings.Contains(err.Error(), "TMDB") {
			fmt.Fprintln(stderr, "请检查网络/代理（可设置 HTTPS_PROXY），或用 --imdb/--douban 指定。")
		}
		if imdb == "" && douban != "" {
			fmt.Fprintln(stderr, "提示：只有豆瓣 id 时依赖 Wikidata 的 IMDb 映射；失败请补 --imdb。")
		}
		return 1
	}
	if meta.ImdbID == "" {
		meta.ImdbID = imdb
	}
	if meta.DoubanID == "" {
		meta.DoubanID = douban
	}
	if interactive {
		promptMissingDouban(m, meta)
	}
	if meta.TmdbID == 0 {
		fmt.Fprintln(stderr, "提示：没有 TMDB 数据，NFO 将缺少评分、简介、类型等；请检查 --imdb 或 TMDB 配置。")
	}

	media, err := probeMedia(ctx, ffprobe, file)
	if err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		return 1
	}

	nfoPath := nfoPathForVideo(file)
	doc := buildNFO(meta, media)
	posterOpts := posterOptions{Width: 32, Mode: posterAuto}
	if code := showMovieNFO(nfoPath, doc, stdout, stderr, posterOpts); code != 0 {
		return code
	}
	nextData, err := renderNFO(doc)
	if err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		return 1
	}
	nfoAction, err := writeNFOChecked(stdout, nfoPath, nextData, interactive)
	if err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		return 1
	}
	printNFOAction(stdout, nfoPath, nfoAction)
	return 0
}

func writeNFOChecked(stdout io.Writer, path string, data []byte, interactive bool) (string, error) {
	existing, readErr := os.ReadFile(path)
	switch {
	case readErr == nil:
		if normalizeNFOText(existing) == normalizeNFOText(data) {
			return "unchanged", nil
		}
		diff, err := nfoDiff(existing, data)
		if err != nil {
			return "", fmt.Errorf("生成 NFO diff 失败：%w", err)
		}
		printNFODiff(stdout, path, diff)
		if interactive && confirmNFOOverwrite(stdout, filepath.Base(path)) {
			if _, err := writeNFOData(path, data, true); err != nil {
				return "", err
			}
			return "overwritten", nil
		}
		return "skipped", nil
	case os.IsNotExist(readErr):
		if _, err := writeNFOData(path, data, false); err != nil {
			return "", err
		}
		return "written", nil
	default:
		return "", fmt.Errorf("读取现有 NFO 失败：%v", readErr)
	}
}

func printNFOAction(w io.Writer, path, action string) {
	label := "有差异，未重写"
	switch action {
	case "written":
		label = "已写入"
	case "overwritten":
		label = "已重写"
	case "unchanged":
		label = "已一致"
	}
	fmt.Fprintf(w, "\nNFO: %s（%s）\n", path, label)
}

func printNFODiff(w io.Writer, path, diff string) {
	if diff == "" {
		return
	}
	fmt.Fprintf(w, "\nNFO diff (%s):\n%s", filepath.Base(path), diff)
	if !strings.HasSuffix(diff, "\n") {
		fmt.Fprintln(w)
	}
}

func confirmNFOOverwrite(w io.Writer, name string) bool {
	fmt.Fprintf(w, "重写 %s？[y/N]: ", name)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func retryTMDB(m *menu, interactive bool, stderr io.Writer, action func() error) error {
	for {
		err := action()
		if err == nil {
			return nil
		}
		if !interactive || !isRetryableTMDBError(err) {
			return err
		}
		fmt.Fprintf(stderr, "TMDB 查询失败：%v\n", err)
		answer := strings.ToLower(strings.TrimSpace(m.readLine("重试 TMDB 查询？[Y/n]: ")))
		if answer != "" && answer != "y" && answer != "yes" {
			return err
		}
	}
}

func isRetryableTMDBError(err error) bool {
	s := strings.ToLower(err.Error())
	for _, needle := range []string{"无效", "未返回有效", "缺少豆瓣", "未配置", "未收录", "404", "401", "403"} {
		if strings.Contains(s, needle) {
			return false
		}
	}
	for _, needle := range []string{"tmdb 查询失败", "查询单集失败", "timeout", "deadline", "connection", "connect", "eof", "tls", "proxy", "network", "http"} {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func resolveScrapeMetadata(ctx context.Context, tmdb *tmdbClient, wd *wikidataClient, imdb, douban string) (*BilingualTitle, error) {
	if imdb == "" && douban == "" {
		return nil, fmt.Errorf("至少提供一个 IMDb id 或豆瓣 id")
	}
	if !tmdb.configured() {
		return nil, fmt.Errorf("未配置 TMDB，无法生成完整 NFO")
	}
	if imdb == "" {
		resolved, err := wd.GetImdbID(ctx, douban)
		if err != nil {
			return nil, fmt.Errorf("豆瓣 id 反查 IMDb 失败：%w", err)
		}
		if resolved == "" {
			return nil, fmt.Errorf("Wikidata 缺少豆瓣 id %s 的 IMDb 映射；请补 --imdb", douban)
		}
		imdb = resolved
	}

	meta, err := tmdb.GetByImdbID(ctx, imdb, douban)
	if err != nil {
		return nil, fmt.Errorf("TMDB 查询失败：%w", err)
	}
	if meta == nil || meta.TmdbID == 0 {
		return nil, fmt.Errorf("TMDB 未返回有效条目：%s", imdb)
	}
	return meta, nil
}

var (
	executablePath = os.Executable
	lookPath       = exec.LookPath
)

func findFFprobe(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("ffprobe 路径不可用：%s：%v", explicit, err)
		}
		return explicit, nil
	}
	if path := ffprobeBesideExecutable(); path != "" {
		return path, nil
	}
	path, err := lookPath("ffprobe")
	if err != nil {
		return "", fmt.Errorf("找不到 ffprobe；请安装 FFmpeg、把 ffprobe 放到 IcedReel.exe 同目录，或用 --ffprobe 指定")
	}
	return path, nil
}

func ffprobeBesideExecutable() string {
	exe, err := executablePath()
	if err != nil || exe == "" {
		return ""
	}
	name := "ffprobe"
	if runtime.GOOS == "windows" {
		name = "ffprobe.exe"
	}
	path := filepath.Join(filepath.Dir(exe), name)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return path
}

var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".mov": true,
	".avi": true, ".wmv": true, ".ts": true, ".m2ts": true, ".mts": true,
	".webm": true, ".mpg": true, ".mpeg": true, ".flv": true,
	".vob": true, ".rmvb": true, ".rm": true, ".ogv": true,
	".3gp": true, ".asf": true, ".f4v": true, ".mxf": true, ".y4m": true,
}

func isVideoFile(path string) bool {
	return videoExts[strings.ToLower(filepath.Ext(path))]
}

func printScrapeUsage(w io.Writer) {
	fmt.Fprint(w, `用法：
  icedreel scrape [选项] <视频文件>

探测单个视频的技术信息，按 IMDb / 豆瓣 id 查 TMDB，写同名电影 NFO；不修改视频文件名。
IMDb / TMDB 同时返回电影和剧集时，按完整度选条目（票数 / 热度 / 海报 / 简介），不盲目电影优先。

选项：
  --imdb <id>      IMDb id 或 URL
  --douban <id>    豆瓣电影 id
  --ffprobe <path> ffprobe.exe 路径
  -h, --help       显示帮助

ffprobe 查找顺序：--ffprobe → IcedReel.exe 同目录 → PATH。

查询只用 --imdb / --douban；TTY 下缺 id 时会依次询问，反推不到豆瓣 id 时补问一次；TMDB 失败时询问是否重试；非交互模式必须显式提供。

示例：
  icedreel scrape "D:\Movies\Movie.mkv" --imdb tt0111161 --douban 1292052
`)
}
