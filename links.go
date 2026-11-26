package main

import (
	"fmt"
	"io"
	"os"
)

type metadataLink struct {
	Label string
	URL   string
}

func metadataLinks(imdb string, tmdbID int, isTV bool, douban string) []metadataLink {
	var links []metadataLink
	if imdb != "" {
		links = append(links, metadataLink{
			Label: "imdb",
			URL:   "https://www.imdb.com/title/" + imdb + "/",
		})
	}
	if tmdbID > 0 {
		kind := "movie"
		if isTV {
			kind = "tv"
		}
		links = append(links, metadataLink{
			Label: "tmdb",
			URL:   fmt.Sprintf("https://www.themoviedb.org/%s/%d", kind, tmdbID),
		})
	}
	if douban != "" {
		links = append(links, metadataLink{
			Label: "douban",
			URL:   "https://movie.douban.com/subject/" + douban + "/",
		})
	}
	return links
}

func printMetadataLinks(w io.Writer, links []metadataLink, hyperlinks bool) {
	if len(links) == 0 {
		return
	}
	if os.Getenv("NO_COLOR") != "" {
		hyperlinks = false
	}
	fmt.Fprintln(w, "链接:")
	for _, link := range links {
		url := link.URL
		if hyperlinks {
			url = osc8Link(link.URL, link.URL)
		}
		fmt.Fprintf(w, "  %s %s\n", link.Label, url)
	}
}

func terminalHyperlinksEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isTerminal(f)
}

func osc8Link(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
