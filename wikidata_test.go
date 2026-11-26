package main

import (
	"encoding/json"
	"testing"
)

func parseResp(t *testing.T, body string) *sparqlResponse {
	t.Helper()
	var resp sparqlResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &resp
}

func TestParseTitleBindingsFull(t *testing.T) {
	resp := parseResp(t, `{
	  "results": { "bindings": [ {
	    "item": {"type":"uri","value":"http://www.wikidata.org/entity/Q172241"},
	    "date": {"type":"literal","value":"1994-09-10T00:00:00Z"},
	    "imdb": {"type":"literal","value":"tt0111161"},
	    "douban": {"type":"literal","value":"1292052"},
	    "tmdbMovie": {"type":"literal","value":"278"},
	    "zhLabel": {"type":"literal","value":"肖申克的救赎","xml:lang":"zh"},
	    "enLabel": {"type":"literal","value":"The Shawshank Redemption","xml:lang":"en"}
	  } ] }
	}`)
	meta := parseTitleBindings(resp, "", "")
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.ImdbID != "tt0111161" || meta.TitleZh != "肖申克的救赎" || meta.TitleEn != "The Shawshank Redemption" ||
		meta.Year != "1994" || meta.DoubanID != "1292052" || meta.TmdbID != 278 ||
		meta.WikidataID != "Q172241" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	want := "肖申克的救赎_The Shawshank Redemption_(1994)_imdb-tt0111161_tmdb-278_douban-1292052"
	if got := BuildName(meta); got != want {
		t.Fatalf("BuildName = %q, want %q", got, want)
	}
}

func TestParsePrefersZhOverRegionalVariants(t *testing.T) {
	resp := parseResp(t, `{
	  "results": { "bindings": [
	    {"zhLabel":{"value":"繁體名","xml:lang":"zh-hant"},"enLabel":{"value":"Title","xml:lang":"en"}},
	    {"zhLabel":{"value":"简体名","xml:lang":"zh"},"enLabel":{"value":"Title","xml:lang":"en"}}
	  ] }
	}`)
	meta := parseTitleBindings(resp, "", "")
	if meta == nil || meta.TitleZh != "简体名" {
		t.Fatalf("got %+v", meta)
	}
}

func TestParsePicksEarliestYear(t *testing.T) {
	resp := parseResp(t, `{
	  "results": { "bindings": [
	    {"date":{"value":"1995-03-09T00:00:00Z"},"enLabel":{"value":"Film","xml:lang":"en"}},
	    {"date":{"value":"1994-09-10T00:00:00Z"},"enLabel":{"value":"Film","xml:lang":"en"}}
	  ] }
	}`)
	meta := parseTitleBindings(resp, "", "")
	if meta == nil || meta.Year != "1994" {
		t.Fatalf("got %+v", meta)
	}
}

func TestParseEnglishOnlyDropsChinese(t *testing.T) {
	resp := parseResp(t, `{
	  "results": { "bindings": [
	    {"imdb":{"value":"tt9"},"enLabel":{"value":"Foo","xml:lang":"en"}}
	  ] }
	}`)
	meta := parseTitleBindings(resp, "", "")
	if meta == nil || meta.TitleZh != "" || meta.TitleEn != "Foo" {
		t.Fatalf("got %+v", meta)
	}
	if got := BuildName(meta); got != "Foo_imdb-tt9" {
		t.Fatalf("BuildName = %q", got)
	}
}

func TestParseUsesFallbackIDs(t *testing.T) {
	resp := parseResp(t, `{
	  "results": { "bindings": [
	    {"enLabel":{"value":"Foo","xml:lang":"en"}}
	  ] }
	}`)
	meta := parseTitleBindings(resp, "tt123", "456")
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.ImdbID != "tt123" || meta.DoubanID != "456" {
		t.Fatalf("got %+v", meta)
	}
	if got := BuildName(meta); got != "Foo_imdb-tt123_douban-456" {
		t.Fatalf("BuildName = %q", got)
	}
}

func TestParseTmdbIDAndArticleTitles(t *testing.T) {
	resp := parseResp(t, `{
	  "results": { "bindings": [ {
	    "date": {"value":"1994-09-10T00:00:00Z"},
	    "item": {"value":"http://www.wikidata.org/entity/Q12345"},
	    "imdb": {"value":"tt0111161"},
	    "douban": {"value":"1292052"},
	    "tmdbMovie": {"value":"278"},
	    "enLabel": {"value":"The Shawshank Redemption","xml:lang":"en"},
	    "zhArticle": {"value":"https://zh.wikipedia.org/wiki/%E8%82%96%E7%94%B3%E5%85%8B%E7%9A%84%E6%95%91%E8%B5%8E"},
	    "enArticle": {"value":"https://en.wikipedia.org/wiki/The_Shawshank_Redemption"}
	  } ] }
	}`)
	meta := parseTitleBindings(resp, "", "")
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.TmdbID != 278 || meta.TitleZh != "肖申克的救赎" || meta.Year != "1994" || meta.DoubanID != "1292052" ||
		meta.WikidataID != "Q12345" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	want := "肖申克的救赎_The Shawshank Redemption_(1994)_imdb-tt0111161_tmdb-278_douban-1292052"
	if got := BuildName(meta); got != want {
		t.Fatalf("BuildName = %q, want %q", got, want)
	}
}

func TestParseEmptyReturnsNil(t *testing.T) {
	if meta := parseTitleBindings(parseResp(t, `{"results":{"bindings":[]}}`), "", ""); meta != nil {
		t.Fatalf("expected nil, got %+v", meta)
	}
	if meta := parseTitleBindings(parseResp(t, `{"results":{"bindings":[{"item":{"value":"http://x"}}]}}`), "", ""); meta != nil {
		t.Fatalf("expected nil, got %+v", meta)
	}
}
