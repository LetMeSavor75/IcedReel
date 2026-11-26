package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTMDBGetByImdbID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/3/find/tt0111161":
			fmt.Fprint(w, `{"movie_results":[{"id":278}],"tv_results":[]}`)
		case r.URL.Path == "/3/movie/278" && r.URL.Query().Get("language") == "en-US":
			fmt.Fprint(w, `{"title":"The Shawshank Redemption","release_date":"1994-09-10","runtime":142}`)
		case r.URL.Path == "/3/movie/278" && r.URL.Query().Get("language") == "zh-CN":
			fmt.Fprint(w, `{"title":"肖申克的救赎"}`)
		case r.URL.Path == "/3/movie/278/external_ids":
			fmt.Fprint(w, `{"imdb_id":"tt0111161","wikidata_id":"Q172241"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := newTMDBClient(tmdbConfig{}, nil)
	c.baseURL = server.URL + "/3/"

	meta, err := c.GetByImdbID(context.Background(), "tt0111161", "1292052")
	if err != nil {
		t.Fatal(err)
	}
	if meta.TmdbID != 278 || meta.ImdbID != "tt0111161" || meta.TitleZh != "肖申克的救赎" ||
		meta.TitleEn != "The Shawshank Redemption" || meta.Year != "1994" ||
		meta.DoubanID != "1292052" || meta.RuntimeMinutes != 142 || meta.WikidataID != "Q172241" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
}

func TestTMDBGetByImdbIDPrefersCompleteResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/3/find/tt8014982":
			fmt.Fprint(w, `{"movie_results":[{"id":1824710,"popularity":0.5,"vote_count":0,"poster_path":null,"overview":"duplicate"}],"tv_results":[{"id":291507,"popularity":131.9,"vote_count":42,"vote_average":7.3,"poster_path":"/poster.jpg","backdrop_path":"/back.jpg","overview":"show"}]}`)
		case r.URL.Path == "/3/tv/291507" && r.URL.Query().Get("language") == "en-US":
			fmt.Fprint(w, `{"name":"The Body","first_air_date":"2026-11-11","vote_count":42,"vote_average":7.3,"poster_path":"/poster.jpg"}`)
		case r.URL.Path == "/3/tv/291507" && r.URL.Query().Get("language") == "zh-CN":
			fmt.Fprint(w, `{"name":"青春共体","overview":"万圣夜"}`)
		case r.URL.Path == "/3/tv/291507/external_ids":
			fmt.Fprint(w, `{"imdb_id":"tt8014982","wikidata_id":"Q137794773"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := newTMDBClient(tmdbConfig{}, nil)
	c.baseURL = server.URL + "/3/"

	meta, err := c.GetByImdbID(context.Background(), "tt8014982", "")
	if err != nil {
		t.Fatal(err)
	}
	if meta.TmdbID != 291507 || !meta.IsTV || meta.TitleZh != "青春共体" || meta.WikidataID != "Q137794773" {
		t.Errorf("metadata = %+v", meta)
	}
}

func TestTMDBGetByImdbIDPrefersCompleteMovie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/3/find/tt0000001":
			fmt.Fprint(w, `{"movie_results":[{"id":278,"popularity":80,"vote_count":100,"vote_average":8.7,"poster_path":"/poster.jpg","overview":"movie"}],"tv_results":[{"id":999,"popularity":1,"vote_count":0,"poster_path":null,"overview":"duplicate"}]}`)
		case r.URL.Path == "/3/movie/278" && r.URL.Query().Get("language") == "en-US":
			fmt.Fprint(w, `{"title":"Movie","release_date":"1994-09-10","vote_count":100,"vote_average":8.7,"poster_path":"/poster.jpg"}`)
		case r.URL.Path == "/3/movie/278" && r.URL.Query().Get("language") == "zh-CN":
			fmt.Fprint(w, `{"title":"电影"}`)
		case r.URL.Path == "/3/movie/278/external_ids":
			fmt.Fprint(w, `{"imdb_id":"tt0000001","wikidata_id":"Q999"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := newTMDBClient(tmdbConfig{}, nil)
	c.baseURL = server.URL + "/3/"

	meta, err := c.GetByImdbID(context.Background(), "tt0000001", "")
	if err != nil {
		t.Fatal(err)
	}
	if meta.TmdbID != 278 || meta.IsTV || meta.WikidataID != "Q999" {
		t.Errorf("metadata = %+v", meta)
	}
}

func TestTMDBGetTVNoRuntime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/3/find/tt0944947":
			fmt.Fprint(w, `{"movie_results":[],"tv_results":[{"id":1399}]}`)
		case r.URL.Path == "/3/tv/1399" && r.URL.Query().Get("language") == "en-US":
			fmt.Fprint(w, `{"name":"Game of Thrones","first_air_date":"2011-04-17"}`)
		case r.URL.Path == "/3/tv/1399" && r.URL.Query().Get("language") == "zh-CN":
			fmt.Fprint(w, `{"name":"权力的游戏"}`)
		case r.URL.Path == "/3/tv/1399/external_ids":
			fmt.Fprint(w, `{"imdb_id":"tt0944947"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := newTMDBClient(tmdbConfig{}, nil)
	c.baseURL = server.URL + "/3/"

	meta, err := c.GetByImdbID(context.Background(), "tt0944947", "3016187")
	if err != nil {
		t.Fatal(err)
	}
	if meta.TmdbID != 1399 || meta.TitleZh != "权力的游戏" ||
		meta.Year != "2011" || meta.RuntimeMinutes != 0 {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
}
