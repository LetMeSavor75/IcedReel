package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type tmdbClient struct {
	httpClient *http.Client
	apiKey     string
	token      string
	baseURL    string
	userAgent  string
	wikidata   *wikidataClient
}

func newTMDBClient(cfg tmdbConfig, wd *wikidataClient) *tmdbClient {
	return &tmdbClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     cfg.APIKey,
		token:      cfg.Token,
		baseURL:    "https://api.themoviedb.org/3/",
		userAgent:  "IcedReel/0.1 (personal CLI)",
		wikidata:   wd,
	}
}

func (c *tmdbClient) configured() bool {
	return c.apiKey != "" || c.token != ""
}

type tmdbDetails struct {
	Title               string     `json:"title"`
	Name                string     `json:"name"`
	OriginalTitle       string     `json:"original_title"`
	OriginalName        string     `json:"original_name"`
	ReleaseDate         string     `json:"release_date"`
	FirstAirDate        string     `json:"first_air_date"`
	Runtime             int        `json:"runtime"`
	Overview            string     `json:"overview"`
	Tagline             string     `json:"tagline"`
	VoteAverage         float64    `json:"vote_average"`
	VoteCount           int        `json:"vote_count"`
	PosterPath          string     `json:"poster_path"`
	BackdropPath        string     `json:"backdrop_path"`
	Status              string     `json:"status"`
	OriginalLanguage    string     `json:"original_language"`
	Genres              []tmdbName `json:"genres"`
	ProductionCompanies []tmdbName `json:"production_companies"`
	ProductionCountries []tmdbName `json:"production_countries"`
}

type tmdbName struct {
	Name string `json:"name"`
}

type tmdbFindResult struct {
	ID           int     `json:"id"`
	Popularity   float64 `json:"popularity"`
	VoteCount    int     `json:"vote_count"`
	VoteAverage  float64 `json:"vote_average"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	Overview     string  `json:"overview"`
}

// score 用来在 IMDb find 同时返回电影和剧集时挑信息更完整的条目。
// 票数权重最高；新建的空壳条目（0 票、无海报、无简介）会被压下去。
func (r tmdbFindResult) score() float64 {
	score := float64(r.VoteCount)*10 + r.Popularity + r.VoteAverage
	if r.PosterPath != "" {
		score += 50
	}
	if r.BackdropPath != "" {
		score += 20
	}
	if strings.TrimSpace(r.Overview) != "" {
		score += 10
	}
	return score
}

func (c *tmdbClient) GetByImdbID(ctx context.Context, rawImdb, doubanOverride string) (*BilingualTitle, error) {
	imdb := NormalizeImdbID(rawImdb)
	if imdb == "" {
		return nil, fmt.Errorf("IMDb id 无效")
	}

	var find struct {
		MovieResults []tmdbFindResult `json:"movie_results"`
		TVResults    []tmdbFindResult `json:"tv_results"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("find/%s?external_source=imdb_id", url.PathEscape(imdb)), &find); err != nil {
		return nil, err
	}

	best := tmdbFindResult{}
	bestIsTV := false
	found := false
	for _, r := range find.MovieResults {
		if !found || r.score() > best.score() {
			best, bestIsTV, found = r, false, true
		}
	}
	for _, r := range find.TVResults {
		if !found || r.score() > best.score() {
			best, bestIsTV, found = r, true, true
		}
	}
	if !found || best.ID == 0 {
		return nil, fmt.Errorf("TMDB 未收录 %s", imdb)
	}

	meta, err := c.GetByTmdbID(ctx, best.ID, bestIsTV, doubanOverride)
	if err != nil {
		return nil, err
	}
	if meta.ImdbID == "" {
		meta.ImdbID = imdb
	}
	return meta, nil
}

func (c *tmdbClient) GetByTmdbID(ctx context.Context, tmdbID int, isTV bool, doubanOverride string) (*BilingualTitle, error) {
	if tmdbID <= 0 {
		return nil, fmt.Errorf("TMDB id 无效")
	}

	kind := "movie"
	if isTV {
		kind = "tv"
	}

	var en tmdbDetails
	if err := c.getJSON(ctx, fmt.Sprintf("%s/%d?language=en-US", kind, tmdbID), &en); err != nil {
		return nil, err
	}
	var zh tmdbDetails
	_ = c.getJSON(ctx, fmt.Sprintf("%s/%d?language=zh-CN", kind, tmdbID), &zh)

	titleEn := en.Title
	if isTV {
		titleEn = en.Name
	}
	if titleEn == "" {
		if isTV {
			titleEn = en.OriginalName
		} else {
			titleEn = en.OriginalTitle
		}
	}

	zhTitle := zh.Title
	zhOriginal := zh.OriginalTitle
	if isTV {
		zhTitle = zh.Name
		zhOriginal = zh.OriginalName
	}
	if zhTitle != "" && looksMostlyLatin(zhTitle) && !looksMostlyLatin(zhOriginal) {
		zhTitle = zhOriginal
	}
	if strings.EqualFold(zhTitle, titleEn) {
		zhTitle = ""
	}

	release := en.ReleaseDate
	if isTV {
		release = en.FirstAirDate
	}
	year := ""
	if len(release) >= 4 {
		year = release[:4]
	}

	imdb := ""
	wikidataID := ""
	var ext struct {
		ImdbID     string `json:"imdb_id"`
		WikidataID string `json:"wikidata_id"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("%s/%d/external_ids", kind, tmdbID), &ext); err == nil {
		imdb = NormalizeImdbID(ext.ImdbID)
		wikidataID = strings.TrimSpace(ext.WikidataID)
	}

	douban := strings.TrimSpace(doubanOverride)
	if douban == "" && c.wikidata != nil && imdb != "" {
		if id, err := c.wikidata.GetDoubanID(ctx, imdb); err == nil {
			douban = id
		}
	}

	runtime := 0
	if !isTV {
		runtime = en.Runtime
	}

	overview := strings.TrimSpace(zh.Overview)
	if overview == "" {
		overview = strings.TrimSpace(en.Overview)
	}
	tagline := strings.TrimSpace(zh.Tagline)
	if tagline == "" {
		tagline = strings.TrimSpace(en.Tagline)
	}
	genres := tmdbNames(zh.Genres)
	if len(genres) == 0 {
		genres = tmdbNames(en.Genres)
	}
	studios := tmdbNames(en.ProductionCompanies)
	countries := tmdbNames(en.ProductionCountries)
	posterPath := firstNonEmpty(en.PosterPath, zh.PosterPath)
	backdropPath := firstNonEmpty(en.BackdropPath, zh.BackdropPath)
	voteAverage := en.VoteAverage
	voteCount := en.VoteCount
	if voteCount == 0 {
		voteAverage = zh.VoteAverage
		voteCount = zh.VoteCount
	}
	releaseDate := en.ReleaseDate
	if isTV {
		releaseDate = en.FirstAirDate
	}

	if titleEn == "" {
		return nil, fmt.Errorf("TMDB 返回的标题为空")
	}

	return &BilingualTitle{
		ImdbID:         imdb,
		TmdbID:         tmdbID,
		WikidataID:     wikidataID,
		TitleEn:        titleEn,
		TitleZh:        zhTitle,
		Year:           year,
		DoubanID:       douban,
		RuntimeMinutes: runtime,
		IsTV:           isTV,
		Overview:       overview,
		Tagline:        tagline,
		VoteAverage:    voteAverage,
		VoteCount:      voteCount,
		PosterPath:     posterPath,
		BackdropPath:   backdropPath,
		Genres:         genres,
		Studios:        studios,
		Countries:      countries,
		ReleaseDate:    releaseDate,
	}, nil
}

func tmdbNames(items []tmdbName) []string {
	var out []string
	for _, item := range items {
		if name := strings.TrimSpace(item.Name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

type tmdbSearchResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	Popularity    float64 `json:"popularity"`
	VoteCount     int     `json:"vote_count"`
	IsTV          bool    `json:"-"`
}

func (c *tmdbClient) Search(ctx context.Context, query string, year int) ([]tmdbSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("搜索标题为空")
	}

	var items []tmdbSearchResult
	var errs []string

	{
		params := url.Values{}
		params.Set("query", query)
		params.Set("language", "zh-CN")
		if year > 0 {
			params.Set("year", strconv.Itoa(year))
		}
		var resp struct {
			Results []tmdbSearchResult `json:"results"`
		}
		if err := c.getJSON(ctx, "search/movie?"+params.Encode(), &resp); err != nil {
			errs = append(errs, "movie: "+err.Error())
		} else {
			for _, r := range resp.Results {
				r.IsTV = false
				items = append(items, r)
			}
		}
	}

	{
		params := url.Values{}
		params.Set("query", query)
		params.Set("language", "zh-CN")
		if year > 0 {
			params.Set("first_air_date_year", strconv.Itoa(year))
		}
		var resp struct {
			Results []tmdbSearchResult `json:"results"`
		}
		if err := c.getJSON(ctx, "search/tv?"+params.Encode(), &resp); err != nil {
			errs = append(errs, "tv: "+err.Error())
		} else {
			for _, r := range resp.Results {
				r.IsTV = true
				items = append(items, r)
			}
		}
	}

	if len(items) == 0 {
		if len(errs) > 0 {
			return nil, fmt.Errorf("TMDB 搜索失败：%s", strings.Join(errs, "；"))
		}
		return nil, nil
	}
	return items, nil
}

func pickSearchResult(query string, year int, items []tmdbSearchResult) (tmdbSearchResult, bool) {
	q := normalizeTitleKey(query)
	if q == "" {
		return tmdbSearchResult{}, false
	}
	yearStr := ""
	if year > 0 {
		yearStr = strconv.Itoa(year)
	}

	type scored struct {
		item       tmdbSearchResult
		score      int
		titleMatch bool
		yearMatch  bool
	}
	var candidates []scored

	for _, it := range items {
		titleMatch := false
		for _, t := range []string{it.Title, it.Name, it.OriginalTitle, it.OriginalName} {
			if normalizeTitleKey(t) == q {
				titleMatch = true
				break
			}
		}
		itemYear := searchItemYear(it)
		yearMatch := yearStr != "" && itemYear == yearStr
		if !titleMatch {
			continue
		}
		score := 0
		if titleMatch {
			score += 1000
		}
		if yearMatch {
			score += 500
		}
		score += int(it.Popularity)
		candidates = append(candidates, scored{it, score, titleMatch, yearMatch})
	}

	if len(candidates) == 0 {
		return tmdbSearchResult{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].item.Popularity != candidates[j].item.Popularity {
			return candidates[i].item.Popularity > candidates[j].item.Popularity
		}
		return candidates[i].item.VoteCount > candidates[j].item.VoteCount
	})
	return candidates[0].item, true
}

func searchItemYear(it tmdbSearchResult) string {
	date := it.ReleaseDate
	if it.IsTV {
		date = it.FirstAirDate
	}
	if len(date) >= 4 {
		return date[:4]
	}
	return ""
}

func normalizeTitleKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (c *tmdbClient) getJSON(ctx context.Context, path string, out any) error {
	u := c.baseURL + path
	if c.token == "" && c.apiKey != "" {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + "api_key=" + url.QueryEscape(c.apiKey)
	}

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.userAgent)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("TMDB 请求失败：%s", redactSecret(err.Error()))
			if attempt < 2 {
				time.Sleep(500 * time.Millisecond)
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("TMDB 返回 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(body)))
			if attempt < 2 {
				time.Sleep(500 * time.Millisecond)
			}
			continue
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(out)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("TMDB JSON 解析失败：%w", decodeErr)
		}
		return nil
	}
	return lastErr
}

var apiKeyRe = regexp.MustCompile(`api_key=[^&\s"]+`)

func redactSecret(s string) string {
	return apiKeyRe.ReplaceAllString(s, "api_key=***")
}
