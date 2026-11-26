package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestPromptMissingIDs(t *testing.T) {
	var out bytes.Buffer
	m := &menu{reader: bufio.NewReader(strings.NewReader("tt0111161\n")), out: &out}
	imdb, douban := promptMissingIDs(m, "", "")
	if imdb != "tt0111161" || douban != "" {
		t.Errorf("ids = %q, %q", imdb, douban)
	}
	if !strings.Contains(out.String(), "IMDb id") {
		t.Errorf("prompt = %q", out.String())
	}
}

func TestPromptMissingIDsFallsBackToDouban(t *testing.T) {
	var out bytes.Buffer
	m := &menu{reader: bufio.NewReader(strings.NewReader("\n1292052\n")), out: &out}
	imdb, douban := promptMissingIDs(m, "", "")
	if imdb != "" || douban != "1292052" {
		t.Errorf("ids = %q, %q", imdb, douban)
	}
}
