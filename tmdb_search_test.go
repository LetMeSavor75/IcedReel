package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPickSearchResult(t *testing.T) {
	items := []tmdbSearchResult{
		{
			ID: 278, Title: "肖申克的救赎", OriginalTitle: "The Shawshank Redemption",
			ReleaseDate: "1994-10-14", Popularity: 10,
		},
		{
			ID: 999, Title: "肖申克的救赎", ReleaseDate: "2000-01-01", Popularity: 100,
		},
		{
			ID: 1000, Title: "Other", ReleaseDate: "1994-10-14", Popularity: 200, IsTV: true,
		},
	}
	item, ok := pickSearchResult("肖申克的救赎", 1994, items)
	if !ok || item.ID != 278 {
		t.Fatalf("got %+v, ok=%v", item, ok)
	}
	if _, ok := pickSearchResult("不存在", 1994, items); ok {
		t.Fatal("unexpected match")
	}
}

func TestTMDBSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/search/movie":
			if r.URL.Query().Get("query") != "肖申克的救赎" || r.URL.Query().Get("year") != "1994" {
				http.Error(w, "bad query", http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `{"results":[{
			  "id":278,"title":"肖申克的救赎","original_title":"The Shawshank Redemption",
			  "release_date":"1994-10-14","popularity":10.5,"vote_count":20
			}]}`)
		case "/3/search/tv":
			fmt.Fprint(w, `{"results":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := newTMDBClient(tmdbConfig{}, nil)
	c.baseURL = server.URL + "/3/"

	items, err := c.Search(context.Background(), "肖申克的救赎", 1994)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != 278 || items[0].IsTV {
		t.Fatalf("items = %+v", items)
	}
}
