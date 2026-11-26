package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

var imdbLikeRe = regexp.MustCompile(`(?i)^(tt\d+|https?://(?:www\.)?imdb\.com/title/tt\d+)`)

type menu struct {
	reader *bufio.Reader
	out    io.Writer
}

func (m *menu) readLine(prompt string) string {
	fmt.Fprint(m.out, prompt)
	line, _ := m.reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func (m *menu) promptField(label, current string) string {
	input := m.readLine(fmt.Sprintf("  %s [%s]: ", label, current))
	switch input {
	case "":
		return current
	case "-":
		return ""
	default:
		return input
	}
}

func runName(args []string, stdout, stderr io.Writer) int {
	fs := pflag.NewFlagSet("icedreel name", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.SortFlags = false

	var (
		imdbFlag   string
		doubanFlag string
		yes        bool
		fallback   bool
		showHelp   bool
	)
	fs.StringVar(&imdbFlag, "imdb", "", "IMDb id 或 URL")
	fs.StringVar(&doubanFlag, "douban", "", "豆瓣 id")
	fs.BoolVarP(&yes, "yes", "y", false, "不交互，只输出目录名")
	fs.BoolVar(&fallback, "fallback", false, "TMDB 失败时允许 Wikidata 兜底")
	fs.BoolVarP(&showHelp, "help", "h", false, "显示帮助")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "错误：%v\n", err)
		fmt.Fprintln(stderr, "运行 icedreel name --help 查看用法。")
		return 2
	}
	if showHelp {
		printNameUsage(stdout)
		return 0
	}

	positional := fs.Args()
	if len(positional) > 1 {
		fmt.Fprintln(stderr, "错误：name 只接受一个 IMDb id 或 URL")
		return 2
	}
	if len(positional) == 1 {
		arg := positional[0]
		if imdbFlag != "" {
			fmt.Fprintln(stderr, "错误：IMDb id 不能同时用位置参数和 --imdb 指定")
			return 2
		}
		if !imdbLikeRe.MatchString(arg) {
			fmt.Fprintln(stderr, "错误：name 只做逻辑命名，不探测文件；实际时长/音轨/字幕请用 scrape")
			return 2
		}
		imdbFlag = arg
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

	interactive := !yes && isTerminal(os.Stdin) && isTerminal(os.Stdout)
	m := &menu{reader: bufio.NewReader(os.Stdin), out: stdout}

	if interactive && imdb == "" {
		fmt.Fprintln(stdout, "先填 IMDb id（可跳过，改用豆瓣 id）。")
		imdb = NormalizeImdbID(m.readLine("  IMDb id (tt...): "))
		if imdb == "" && douban == "" {
			douban = onlyDigits(m.readLine("  豆瓣 id: "))
		}
	}
	if imdb == "" && douban == "" && !interactive {
		fmt.Fprintln(stderr, "错误：至少提供一个 IMDb id 或豆瓣 id")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	wd := newWikidataClient()
	cfg := loadTMDBConfig()
	tmdb := newTMDBClient(cfg, wd)

	var meta *BilingualTitle
	var err error

	if imdb != "" || douban != "" {
		if tmdb.configured() {
			if imdb == "" && douban != "" {
				resolved, resolveErr := wd.GetImdbID(ctx, douban)
				if resolveErr != nil {
					fmt.Fprintf(stderr, "提示：豆瓣 id 反查 IMDb 失败：%v\n", resolveErr)
				} else if resolved != "" {
					imdb = resolved
				}
			}
			if imdb != "" {
				meta, err = tmdb.GetByImdbID(ctx, imdb, douban)
				if err != nil {
					if interactive {
						fmt.Fprintf(stderr, "提示：TMDB 查询失败：%v\n", err)
						if douban == "" {
							fmt.Fprintln(stdout, "如果有豆瓣 id，可补上用于 Wikidata 兜底（回车跳过）：")
							douban = onlyDigits(m.readLine("  豆瓣 id: "))
						}
					} else if !fallback {
						fmt.Fprintf(stderr, "错误：TMDB 查询失败：%v\n", err)
						fmt.Fprintln(stderr, "请检查网络/代理（可设置 HTTPS_PROXY），或加 --fallback 使用 Wikidata 兜底（可能缺中文名/ID）。")
						return 1
					} else {
						fmt.Fprintf(stderr, "提示：TMDB 查询失败，按 --fallback 使用 Wikidata：%v\n", err)
					}
				}
			}
		} else if !interactive {
			fmt.Fprintln(stderr, "提示：未配置 TMDB，使用 Wikidata 兜底（覆盖可能不全）")
		}

		if meta == nil {
			wdMeta, wdErr := wd.GetTitle(ctx, imdb, douban)
			if wdErr != nil {
				if interactive {
					fmt.Fprintf(stderr, "提示：Wikidata 查询失败，进入手动补充：%v\n", wdErr)
				} else if !fallback {
					fmt.Fprintf(stderr, "错误：Wikidata 查询失败：%v\n", wdErr)
					return 1
				} else {
					fmt.Fprintf(stderr, "提示：Wikidata 查询失败：%v\n", wdErr)
				}
			} else if wdMeta != nil {
				meta = wdMeta
			}
		}
	}

	if meta == nil {
		if !interactive {
			fmt.Fprintln(stderr, "错误：未找到元数据（TMDB / Wikidata 均未收录，或 id 有误）")
			return 1
		}
		meta = &BilingualTitle{}
	}
	if meta.ImdbID == "" {
		meta.ImdbID = imdb
	}
	if meta.DoubanID == "" {
		meta.DoubanID = douban
	}

	if interactive {
		editFields(m, meta)
	}
	if strings.TrimSpace(meta.TitleZh) == "" && strings.TrimSpace(meta.TitleEn) == "" {
		fmt.Fprintln(stderr, "错误：中文名和 English 至少填一个")
		return 1
	}

	name := BuildName(meta)
	if name == "" {
		fmt.Fprintln(stderr, "错误：生成的目录名为空")
		return 1
	}

	if yes {
		fmt.Fprintln(stdout, name)
		return 0
	}

	if !interactive {
		printNameFields(stdout, meta)
		fmt.Fprintf(stdout, "目录名: %s\n", name)
		return 0
	}

	answer := strings.ToLower(m.readLine("\n确认生成？[Y/n]: "))
	if answer == "n" || answer == "no" {
		fmt.Fprintln(stdout, "已取消。")
		return 0
	}
	fmt.Fprintf(stdout, "目录名: %s\n", name)
	return 0
}

func editFields(m *menu, meta *BilingualTitle) {
	fmt.Fprintln(m.out, "逐项确认（回车接受反查值，输入新值覆盖，输入 - 清空）：")
	meta.TitleZh = m.promptField("中文名", meta.TitleZh)
	meta.TitleEn = m.promptField("English", meta.TitleEn)
	meta.Year = m.promptField("年份", meta.Year)
	meta.ImdbID = m.promptField("imdb-id", meta.ImdbID)
	tmdbCurrent := ""
	if meta.TmdbID > 0 {
		tmdbCurrent = strconv.Itoa(meta.TmdbID)
	}
	tmdbInput := m.promptField("tmdb-id", tmdbCurrent)
	if tmdbInput == "" {
		meta.TmdbID = 0
	} else if id, err := strconv.Atoi(onlyDigits(tmdbInput)); err == nil {
		meta.TmdbID = id
	}
	meta.DoubanID = m.promptField("douban-id", meta.DoubanID)

	if meta.ImdbID != "" {
		meta.ImdbID = NormalizeImdbID(meta.ImdbID)
	}
	if meta.DoubanID != "" {
		meta.DoubanID = onlyDigits(meta.DoubanID)
	}
}

func printNameFields(w io.Writer, meta *BilingualTitle) {
	tmdb := "-"
	if meta.TmdbID > 0 {
		tmdb = fmt.Sprintf("%d", meta.TmdbID)
	}
	douban := meta.DoubanID
	if douban == "" {
		douban = "-"
	}
	year := meta.Year
	if year == "" {
		year = "-"
	}
	fmt.Fprintf(w, "TMDB:    %s\n", tmdb)
	fmt.Fprintf(w, "IMDb:    %s\n", meta.ImdbID)
	fmt.Fprintf(w, "豆瓣:    %s\n", douban)
	fmt.Fprintf(w, "中文:    %s\n", orDash(meta.TitleZh))
	fmt.Fprintf(w, "English: %s\n", orDash(meta.TitleEn))
	fmt.Fprintf(w, "年份:    %s\n", year)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func printNameUsage(w io.Writer) {
	fmt.Fprint(w, `用法：
  icedreel name [选项] [tt...]

只生成逻辑目录名，不探测文件；实际时长、音轨、字幕、编码由 scrape 处理。

选项：
  --imdb <id>      IMDb id 或 URL
  --douban <id>    豆瓣 id
  -y, --yes        不交互，只输出目录名
  --fallback       非交互模式下 TMDB 失败时用 Wikidata 兜底；交互模式会自动进入手动补充
  -h, --help       显示帮助

配置：
  ICEDREEL_TMDB_TOKEN      TMDB Read Access Token（推荐）
  ICEDREEL_TMDB_APIKEY     TMDB API Key（可选）

TMDB 不可达时：
  先开代理/VPN，再设置 HTTPS_PROXY，例如：
    $env:HTTPS_PROXY = 'http://127.0.0.1:7890'
  或加 --fallback 用 Wikidata 兜底。

示例：
  icedreel name tt0111161
  icedreel name --douban 1292052
  icedreel name tt0111161 --douban 1292052 -y
`)
}
