package main

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPromptMissingDouban(t *testing.T) {
	meta := &BilingualTitle{}
	var out bytes.Buffer
	m := &menu{reader: bufio.NewReader(strings.NewReader("1292052\n")), out: &out}
	promptMissingDouban(m, meta)
	if meta.DoubanID != "1292052" {
		t.Errorf("douban = %q", meta.DoubanID)
	}
	if !strings.Contains(out.String(), "没有反推到豆瓣 id") {
		t.Errorf("prompt = %q", out.String())
	}
}

func TestPromptMissingDoubanKeepsExisting(t *testing.T) {
	meta := &BilingualTitle{DoubanID: "1292052"}
	var out bytes.Buffer
	m := &menu{reader: bufio.NewReader(strings.NewReader("")), out: &out}
	promptMissingDouban(m, meta)
	if meta.DoubanID != "1292052" || out.Len() != 0 {
		t.Errorf("meta = %+v, out = %q", meta, out.String())
	}
}

func TestRetryTMDB(t *testing.T) {
	calls := 0
	var out bytes.Buffer
	m := &menu{reader: bufio.NewReader(strings.NewReader("y\n")), out: &out}
	err := retryTMDB(m, true, &out, func() error {
		calls++
		if calls == 1 {
			return errors.New("context deadline exceeded")
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Errorf("retry = %v, calls = %d, out = %q", err, calls, out.String())
	}
}

func TestRetryTMDBDeclines(t *testing.T) {
	calls := 0
	var out bytes.Buffer
	m := &menu{reader: bufio.NewReader(strings.NewReader("n\n")), out: &out}
	err := retryTMDB(m, true, &out, func() error {
		calls++
		return errors.New("context deadline exceeded")
	})
	if err == nil || calls != 1 {
		t.Errorf("retry = %v, calls = %d", err, calls)
	}
}

func TestRetryTMDBNoRetryForInvalidID(t *testing.T) {
	calls := 0
	var out bytes.Buffer
	m := &menu{reader: bufio.NewReader(strings.NewReader("y\n")), out: &out}
	err := retryTMDB(m, true, &out, func() error {
		calls++
		return errors.New("TMDB 未收录 tt0000000")
	})
	if err == nil || calls != 1 || out.Len() != 0 {
		t.Errorf("retry = %v, calls = %d, out = %q", err, calls, out.String())
	}
}

func TestWriteNFOChecked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "movie.nfo")
	first := []byte("first\n")
	action, err := writeNFOChecked(&bytes.Buffer{}, path, first, false)
	if err != nil || action != "written" {
		t.Fatalf("write new = %q, %v", action, err)
	}
	action, err = writeNFOChecked(&bytes.Buffer{}, path, first, false)
	if err != nil || action != "unchanged" {
		t.Fatalf("write same = %q, %v", action, err)
	}

	var out bytes.Buffer
	action, err = writeNFOChecked(&out, path, []byte("second\n"), false)
	if err != nil || action != "skipped" {
		t.Fatalf("write diff = %q, %v", action, err)
	}
	if !strings.Contains(out.String(), "NFO diff (movie.nfo):") {
		t.Errorf("diff output = %q", out.String())
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(first) {
		t.Errorf("file changed: %q, %v", data, err)
	}
}

func TestPrintNFODiff(t *testing.T) {
	var buf bytes.Buffer
	printNFODiff(&buf, "movie.nfo", "--- 现有 NFO\n+++ 新 NFO\n@@ -1 +1 @@\n-a\n+b\n")
	out := buf.String()
	for _, want := range []string{"NFO diff (movie.nfo):", "--- 现有 NFO", "+b"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPrintNFODiffEmpty(t *testing.T) {
	var buf bytes.Buffer
	printNFODiff(&buf, "movie.nfo", "")
	if buf.Len() != 0 {
		t.Errorf("empty diff printed %q", buf.String())
	}
}

func TestFindFFprobeExplicit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-ffprobe.exe")
	if err := os.WriteFile(path, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := findFFprobe(path)
	if err != nil || got != path {
		t.Fatalf("findFFprobe = %q, %v; want %q", got, err, path)
	}
}

func TestFindFFprobeBesideExecutable(t *testing.T) {
	dir := t.TempDir()
	name := "ffprobe"
	if runtime.GOOS == "windows" {
		name = "ffprobe.exe"
	}
	want := filepath.Join(dir, name)
	if err := os.WriteFile(want, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldExecutablePath, oldLookPath := executablePath, lookPath
	t.Cleanup(func() {
		executablePath = oldExecutablePath
		lookPath = oldLookPath
	})
	executablePath = func() (string, error) {
		return filepath.Join(dir, "IcedReel.exe"), nil
	}
	lookPath = func(string) (string, error) {
		t.Fatal("同目录存在 ffprobe 时不应再查 PATH")
		return "", nil
	}

	got, err := findFFprobe("")
	if err != nil {
		t.Fatalf("findFFprobe: %v", err)
	}
	if got != want {
		t.Fatalf("findFFprobe = %q, want %q", got, want)
	}
}

func TestFindFFprobeFallsBackToPath(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "bin", "ffprobe.exe")

	oldExecutablePath, oldLookPath := executablePath, lookPath
	t.Cleanup(func() {
		executablePath = oldExecutablePath
		lookPath = oldLookPath
	})
	executablePath = func() (string, error) {
		return filepath.Join(dir, "IcedReel.exe"), nil
	}
	lookPath = func(name string) (string, error) {
		if name != "ffprobe" {
			t.Fatalf("LookPath(%q), want ffprobe", name)
		}
		return want, nil
	}

	got, err := findFFprobe("")
	if err != nil {
		t.Fatalf("findFFprobe: %v", err)
	}
	if got != want {
		t.Fatalf("findFFprobe = %q, want %q", got, want)
	}
}

func TestFindFFprobeMissing(t *testing.T) {
	dir := t.TempDir()

	oldExecutablePath, oldLookPath := executablePath, lookPath
	t.Cleanup(func() {
		executablePath = oldExecutablePath
		lookPath = oldLookPath
	})
	executablePath = func() (string, error) {
		return filepath.Join(dir, "IcedReel.exe"), nil
	}
	lookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	_, err := findFFprobe("")
	if err == nil || !strings.Contains(err.Error(), "找不到 ffprobe") {
		t.Fatalf("findFFprobe error = %v", err)
	}
	if !strings.Contains(err.Error(), "同目录") {
		t.Errorf("error should mention executable directory: %v", err)
	}
}
