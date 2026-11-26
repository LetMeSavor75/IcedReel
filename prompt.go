package main

import "fmt"

func promptMissingIDs(m *menu, imdb, douban string) (string, string) {
	fmt.Fprintln(m.out, "先填 IMDb id（可跳过，改用豆瓣 id）。")
	imdb = NormalizeImdbID(m.readLine("  IMDb id (tt...): "))
	if imdb == "" {
		douban = onlyDigits(m.readLine("  豆瓣 id: "))
	}
	return imdb, douban
}

func promptMissingDouban(m *menu, meta *BilingualTitle) {
	if meta.DoubanID != "" {
		return
	}
	fmt.Fprintln(m.out, "提示：没有反推到豆瓣 id，可手动补一个（回车跳过）。")
	if id := onlyDigits(m.readLine("  豆瓣 id: ")); id != "" {
		meta.DoubanID = id
	}
}
