package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type wikidataClient struct {
	httpClient *http.Client
	endpoint   string
	userAgent  string
}

func newWikidataClient() *wikidataClient {
	return &wikidataClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		endpoint:   "https://query.wikidata.org/sparql",
		userAgent:  "IcedReel/0.1 (personal CLI)",
	}
}

type sparqlValue struct {
	Value string `json:"value"`
	Lang  string `json:"xml:lang"`
}

type sparqlResponse struct {
	Results struct {
		Bindings []map[string]sparqlValue `json:"bindings"`
	} `json:"results"`
}

func (w *wikidataClient) GetDoubanID(ctx context.Context, imdbID string) (string, error) {
	tt := NormalizeImdbID(imdbID)
	if tt == "" {
		return "", nil
	}
	query := fmt.Sprintf(
		`SELECT ?douban WHERE { ?film wdt:P345 "%s" ; wdt:P4529 ?douban . } LIMIT 1`, tt)
	return w.queryFirst(ctx, query, "douban")
}

func (w *wikidataClient) GetImdbID(ctx context.Context, doubanID string) (string, error) {
	id := onlyDigits(doubanID)
	if id == "" {
		return "", nil
	}
	query := fmt.Sprintf(
		`SELECT ?imdb WHERE { ?film wdt:P4529 "%s" ; wdt:P345 ?imdb . } LIMIT 1`, id)
	tt, err := w.queryFirst(ctx, query, "imdb")
	if err != nil {
		return "", err
	}
	return NormalizeImdbID(tt), nil
}

func (w *wikidataClient) GetTitle(ctx context.Context, imdbID, doubanID string) (*BilingualTitle, error) {
	tt := NormalizeImdbID(imdbID)
	if tt == "" {
		tt = ""
	}
	douban := onlyDigits(doubanID)
	if tt == "" && douban == "" {
		return nil, nil
	}

	var match string
	if tt != "" {
		match = fmt.Sprintf(`{ ?item wdt:P345 "%s" . }`, tt)
	} else {
		match = fmt.Sprintf(`{ ?item wdt:P4529 "%s" . }`, douban)
	}
	query := "SELECT ?item ?date ?imdb ?douban ?tmdbMovie ?tmdbTV ?zhLabel ?enLabel ?zhArticle ?enArticle WHERE { " +
		match +
		" OPTIONAL { ?item wdt:P577 ?date . }" +
		" OPTIONAL { ?item wdt:P345 ?imdb . }" +
		" OPTIONAL { ?item wdt:P4529 ?douban . }" +
		" OPTIONAL { ?item wdt:P4947 ?tmdbMovie . }" +
		" OPTIONAL { ?item wdt:P4983 ?tmdbTV . }" +
		` OPTIONAL { ?item rdfs:label ?zhLabel . FILTER(lang(?zhLabel) IN ("zh","zh-hans","zh-cn","zh-hant","zh-tw","zh-hk")) }` +
		` OPTIONAL { ?item rdfs:label ?enLabel . FILTER(lang(?enLabel) = "en") }` +
		" OPTIONAL { ?zhArticle schema:about ?item ; schema:isPartOf <https://zh.wikipedia.org/> . }" +
		" OPTIONAL { ?enArticle schema:about ?item ; schema:isPartOf <https://en.wikipedia.org/> . }" +
		" }"

	resp, err := w.query(ctx, query)
	if err != nil {
		return nil, err
	}
	return parseTitleBindings(resp, nullable(tt), nullable(douban)), nil
}

func (w *wikidataClient) queryFirst(ctx context.Context, query, variable string) (string, error) {
	resp, err := w.query(ctx, query)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Results.Bindings) == 0 {
		return "", nil
	}
	if v, ok := resp.Results.Bindings[0][variable]; ok {
		return strings.TrimSpace(v.Value), nil
	}
	return "", nil
}

func (w *wikidataClient) query(ctx context.Context, query string) (*sparqlResponse, error) {
	endpoint := w.endpoint + "?format=json&query=" + url.QueryEscape(query)
	var lastErr error

	for attempt := 1; attempt <= 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", w.userAgent)
		req.Header.Set("Accept", "application/sparql-results+json")

		resp, err := w.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < 2 {
				time.Sleep(400 * time.Millisecond)
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("Wikidata 返回 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(body)))
			if attempt < 2 {
				time.Sleep(400 * time.Millisecond)
			}
			continue
		}
		var out sparqlResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&out)
		_ = resp.Body.Close()
		if decodeErr != nil {
			lastErr = decodeErr
			if attempt < 2 {
				time.Sleep(400 * time.Millisecond)
			}
			continue
		}
		return &out, nil
	}
	return nil, lastErr
}

func parseTitleBindings(resp *sparqlResponse, fallbackImdb, fallbackDouban string) *BilingualTitle {
	if resp == nil || len(resp.Results.Bindings) == 0 {
		return nil
	}

	zh := ""
	zhRank := int(^uint(0) >> 1)
	en := ""
	zhArticle := ""
	enArticle := ""
	wikidataID := ""
	bindingImdb := ""
	bindingDouban := ""
	tmdbID := 0
	year := ""

	for _, row := range resp.Results.Bindings {
		if v, ok := row["zhLabel"]; ok && strings.TrimSpace(v.Value) != "" {
			rank := zhLangRank(v.Lang)
			if rank < zhRank {
				zhRank = rank
				zh = v.Value
			}
		}
		if en == "" {
			if v, ok := row["enLabel"]; ok {
				en = strings.TrimSpace(v.Value)
			}
		}
		if zhArticle == "" {
			if v, ok := row["zhArticle"]; ok {
				zhArticle = articleTitle(v.Value)
			}
		}
		if enArticle == "" {
			if v, ok := row["enArticle"]; ok {
				enArticle = articleTitle(v.Value)
			}
		}
		if wikidataID == "" {
			if v, ok := row["item"]; ok {
				wikidataID = wikidataEntityID(v.Value)
			}
		}
		if bindingImdb == "" {
			if v, ok := row["imdb"]; ok {
				bindingImdb = strings.TrimSpace(v.Value)
			}
		}
		if bindingDouban == "" {
			if v, ok := row["douban"]; ok {
				bindingDouban = strings.TrimSpace(v.Value)
			}
		}
		if tmdbID == 0 {
			if v, ok := row["tmdbMovie"]; ok {
				if id, err := strconv.Atoi(onlyDigits(v.Value)); err == nil {
					tmdbID = id
				}
			}
		}
		if tmdbID == 0 {
			if v, ok := row["tmdbTV"]; ok {
				if id, err := strconv.Atoi(onlyDigits(v.Value)); err == nil {
					tmdbID = id
				}
			}
		}
		if v, ok := row["date"]; ok {
			if y := extractYear(v.Value); y != "" && (year == "" || y < year) {
				year = y
			}
		}
	}

	if bindingImdb != "" {
		bindingImdb = NormalizeImdbID(bindingImdb)
	}
	finalImdb := firstNonEmpty(bindingImdb, fallbackImdb)
	finalDouban := firstNonEmpty(bindingDouban, fallbackDouban)

	titleZh := strings.TrimSpace(zh)
	if titleZh == "" {
		titleZh = strings.TrimSpace(zhArticle)
	}
	titleEn := strings.TrimSpace(en)
	if titleEn == "" {
		titleEn = strings.TrimSpace(enArticle)
	}
	if titleEn == "" {
		titleEn = titleZh
	}
	if titleEn == "" {
		return nil
	}
	return &BilingualTitle{
		ImdbID:     finalImdb,
		TmdbID:     tmdbID,
		WikidataID: wikidataID,
		TitleEn:    titleEn,
		TitleZh:    titleZh,
		Year:       year,
		DoubanID:   finalDouban,
	}
}

func wikidataEntityID(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	const prefix = "/entity/"
	if !strings.HasPrefix(u.Path, prefix) {
		return ""
	}
	id := strings.TrimPrefix(u.Path, prefix)
	if len(id) < 2 || id[0] != 'Q' {
		return ""
	}
	for _, r := range id[1:] {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return id
}

func articleTitle(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	const prefix = "/wiki/"
	if !strings.HasPrefix(u.Path, prefix) {
		return ""
	}
	title, err := url.PathUnescape(strings.TrimPrefix(u.Path, prefix))
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(title, "_", " ")
}

func zhLangRank(lang string) int {
	switch strings.ToLower(lang) {
	case "zh":
		return 0
	case "zh-hans":
		return 1
	case "zh-cn":
		return 2
	case "zh-hant":
		return 3
	case "zh-tw":
		return 4
	case "zh-hk":
		return 5
	default:
		return 99
	}
}

func extractYear(value string) string {
	if len(value) >= 4 {
		y := value[:4]
		for _, r := range y {
			if r < '0' || r > '9' {
				return ""
			}
		}
		return y
	}
	return ""
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func nullable(s string) string {
	if s == "" {
		return ""
	}
	return s
}
