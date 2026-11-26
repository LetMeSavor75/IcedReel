package main

import (
	"fmt"
	"io"
	"os"
)

const version = "0.1.0"

func main() {
	restore := enableUTF8Console()
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	restore()
	os.Exit(code)
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "name":
		return runName(args[1:], stdout, stderr)
	case "scrape":
		return runScrape(args[1:], stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "rank":
		fmt.Fprintln(stderr, "rank 尚未实现；当前支持 name、scrape、show。")
		return 2
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "-V", "--version":
		fmt.Fprintf(stdout, "icedreel %s\n", version)
		return 0
	default:
		fmt.Fprintf(stderr, "错误：未知命令 %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `IcedReel — 视频库整理工具

用法：
  icedreel name [选项] [tt...]
  icedreel scrape [选项] <视频文件>
  icedreel show [选项] <视频或 .nfo>
  icedreel --help
  icedreel --version

命令：
  name               生成逻辑目录名（TMDB / Wikidata）
  scrape             写电影 NFO
  show               显示 NFO 元数据（TTY 下带 poster/fanart）
  rank               视频质量排名（后续）

命名格式：
  肖申克的救赎 (The Shawshank Redemption) (1994) {imdb-tt0111161} {tmdb-278} {douban-1292052}
  name 只生成逻辑目录名；时长/音轨/字幕/编码由 scrape 处理。
`)
}
