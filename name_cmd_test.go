package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestMenuPromptField(t *testing.T) {
	m := &menu{
		reader: bufio.NewReader(strings.NewReader("\nNew English\n-\n")),
		out:    &bytes.Buffer{},
	}
	if got := m.promptField("中文名", "旧名"); got != "旧名" {
		t.Errorf("empty input should keep current, got %q", got)
	}
	if got := m.promptField("English", "Old"); got != "New English" {
		t.Errorf("new input should override, got %q", got)
	}
	if got := m.promptField("年份", "1995"); got != "" {
		t.Errorf("- should clear, got %q", got)
	}
}

func TestEditFields(t *testing.T) {
	meta := &BilingualTitle{
		ImdbID:   "tt0111161",
		TmdbID:   278,
		TitleZh:  "肖申克的救赎",
		TitleEn:  "The Shawshank Redemption",
		Year:     "1994",
		DoubanID: "1292052",
	}
	m := &menu{
		reader: bufio.NewReader(strings.NewReader("\nNew English\n\n\n279\n-\n")),
		out:    &bytes.Buffer{},
	}
	editFields(m, meta)
	if meta.TitleZh != "肖申克的救赎" || meta.TitleEn != "New English" || meta.Year != "1994" ||
		meta.ImdbID != "tt0111161" || meta.TmdbID != 279 || meta.DoubanID != "" {
		t.Fatalf("unexpected meta: %+v", meta)
	}
}

func TestEditFieldsNormalizesIDs(t *testing.T) {
	meta := &BilingualTitle{ImdbID: "", TitleEn: "Foo"}
	m := &menu{
		reader: bufio.NewReader(strings.NewReader("\n\n\nhttps://www.imdb.com/title/tt0111161/\n\n1292052\n")),
		out:    &bytes.Buffer{},
	}
	editFields(m, meta)
	if meta.ImdbID != "tt0111161" || meta.DoubanID != "1292052" {
		t.Fatalf("unexpected IDs: %+v", meta)
	}
}
