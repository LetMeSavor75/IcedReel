package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRenderHalfBlock(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if y < 2 {
				img.Set(x, y, red)
			} else {
				img.Set(x, y, blue)
			}
		}
	}

	got := renderHalfBlock(img, 4)
	redCell := "\x1b[38;2;255;0;0m\x1b[48;2;255;0;0m▀"
	blueCell := "\x1b[38;2;0;0;255m\x1b[48;2;0;0;255m▀"
	want := strings.Repeat(redCell, 4) + "\x1b[0m\n" + strings.Repeat(blueCell, 4) + "\x1b[0m\n"
	if got != want {
		t.Errorf("renderHalfBlock mismatch\n got = %q\nwant = %q", got, want)
	}
	if n := strings.Count(got, "\n"); n != 2 {
		t.Errorf("rows = %d, want 2", n)
	}
}

func TestRenderHalfBlockEmpty(t *testing.T) {
	if got := renderHalfBlock(image.NewRGBA(image.Rect(0, 0, 0, 0)), 32); got != "" {
		t.Errorf("empty image = %q, want empty", got)
	}
}

func TestComposePosterPair(t *testing.T) {
	poster := image.NewRGBA(image.Rect(0, 0, 100, 150))
	fanart := image.NewRGBA(image.Rect(0, 0, 160, 90))
	got := composePosterPair(poster, fanart)
	if got.Bounds().Dy() != 90 {
		t.Errorf("height = %d", got.Bounds().Dy())
	}
	if got.Bounds().Dx() <= fanart.Bounds().Dx() {
		t.Errorf("width = %d", got.Bounds().Dx())
	}
}

func TestPosterSourcePrefersLocal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "poster.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "poster.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := posterSource(nil, filepath.Join(dir, "movie.mkv"))
	if got != filepath.Join(dir, "poster.jpg") {
		t.Errorf("posterSource = %q", got)
	}
}

func TestShowPosterLocal(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 60), G: uint8(y * 40), B: 128, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "poster.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if !showPoster(&buf, path, posterOptions{Width: 8, Mode: posterHalf}, io.Discard) {
		t.Fatal("showPoster returned false")
	}
	if !strings.Contains(buf.String(), "▀") {
		t.Errorf("poster output has no half block: %q", buf.String())
	}
}

func TestShowPosterPair(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, w, h int, c color.Color) string {
		t.Helper()
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.Set(x, y, c)
			}
		}
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		return path
	}

	poster := write("poster.png", 4, 6, color.RGBA{R: 255, A: 255})
	fanart := write("fanart.png", 8, 4, color.RGBA{B: 255, A: 255})
	var buf bytes.Buffer
	if !showPosterPair(&buf, poster, fanart, posterOptions{Width: 20, Mode: posterHalf}, io.Discard) {
		t.Fatal("showPosterPair returned false")
	}
	if !strings.Contains(buf.String(), "▀") {
		t.Errorf("pair output has no half block")
	}
	if showPosterPair(io.Discard, poster, fanart, posterOptions{Width: 20, Mode: posterHalf, NoPoster: true}, io.Discard) {
		t.Error("noPoster should skip pair")
	}
}

func TestShowPosterSkips(t *testing.T) {
	if showPoster(io.Discard, "https://example.com/poster.jpg", posterOptions{Width: 32, NoPoster: true}, io.Discard) {
		t.Error("noPoster should skip")
	}
	t.Setenv("NO_COLOR", "1")
	if showPoster(io.Discard, "https://example.com/poster.jpg", posterOptions{Width: 32, Mode: posterHalf}, io.Discard) {
		t.Error("NO_COLOR should skip")
	}
}

func TestParsePosterMode(t *testing.T) {
	cases := map[string]posterMode{
		"":      posterAuto,
		"auto":  posterAuto,
		"sixel": posterSixel,
		"half":  posterHalf,
		"block": posterHalf,
		"none":  posterNone,
	}
	for in, want := range cases {
		got, err := parsePosterMode(in)
		if err != nil || got != want {
			t.Errorf("parsePosterMode(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parsePosterMode("kitty"); err == nil {
		t.Error("kitty should be invalid")
	}
}

func TestPosterSourceForMode(t *testing.T) {
	if got := posterSourceForMode("https://image.tmdb.org/t/p/w500/poster.jpg", posterSixel); got != "https://image.tmdb.org/t/p/w780/poster.jpg" {
		t.Errorf("sixel source = %q", got)
	}
	if got := posterSourceForMode("https://image.tmdb.org/t/p/w500/poster.jpg", posterHalf); got != "https://image.tmdb.org/t/p/w500/poster.jpg" {
		t.Errorf("half source = %q", got)
	}
	if got := posterSourceForMode("/tmp/poster.jpg", posterSixel); got != "/tmp/poster.jpg" {
		t.Errorf("local source = %q", got)
	}
}

func TestScalePosterToWidth(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))
	scaled := scalePosterToWidth(img, 50)
	if b := scaled.Bounds(); b.Dx() != 50 || b.Dy() != 100 {
		t.Errorf("scaled = %dx%d, want 50x100", b.Dx(), b.Dy())
	}
	if got := scalePosterToWidth(img, 200); got != img {
		t.Error("small image should not be upscaled")
	}
	corrected := scalePosterToSize(img, 50, 0.9)
	if b := corrected.Bounds(); b.Dx() != 50 || b.Dy() != 90 {
		t.Errorf("corrected = %dx%d, want 50x90", b.Dx(), b.Dy())
	}
	notUpscaled := scalePosterToSize(img, 200, 0.9)
	if b := notUpscaled.Bounds(); b.Dx() != 100 || b.Dy() != 180 {
		t.Errorf("not upscaled = %dx%d, want 100x180", b.Dx(), b.Dy())
	}
}

func TestSixelTargetSize(t *testing.T) {
	if width, correction := sixelTargetSize(60, 9, 20, true); width != 600 || math.Abs(correction-0.9) > 1e-9 {
		t.Errorf("wt target = %d, %f; want 600, 0.9", width, correction)
	}
	if width, correction := sixelTargetSize(60, 8, 16, false); width != 480 || correction != 1 {
		t.Errorf("non-wt target = %d, %f; want 480, 1", width, correction)
	}
	if width, correction := sixelTargetSize(60, 8, 21, true); width != 600 || math.Abs(correction-16.0/21.0) > 1e-9 {
		t.Errorf("tall cell target = %d, %f; want 600, %f", width, correction, 16.0/21.0)
	}
	if width, correction := sixelTargetSize(60, 0, 0, true); width != 600 || correction != 1 {
		t.Errorf("fallback target = %d, %f; want 600, 1", width, correction)
	}
}

func TestPosterCellSizeOverride(t *testing.T) {
	t.Setenv("ICEDREEL_CELL_SIZE", "8x21")
	if w, h := posterCellSize(); w != 8 || h != 21 {
		t.Errorf("override cell = %dx%d, want 8x21", w, h)
	}
	t.Setenv("ICEDREEL_CELL_SIZE", "bad")
	if _, _, ok := cellSizeOverride(); ok {
		t.Error("invalid override should not be accepted")
	}
}

func TestFirstFontFace(t *testing.T) {
	cases := map[string]string{
		"'NewComputerModern Mono 10', Cascadia Mono NF": "NewComputerModern Mono 10",
		`"Cascadia Mono", Consolas`:                     "Cascadia Mono",
		"Cascadia Mono NF,Cascadia Mono":                "Cascadia Mono NF",
		"":                                              "",
	}
	for in, want := range cases {
		if got := firstFontFace(in); got != want {
			t.Errorf("firstFontFace(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripJSONCommentsAndTrailingCommas(t *testing.T) {
	src := []byte(`{
  // line comment
  "profiles": {
    "defaults": {
      "font": {
        "face": "'NewComputerModern Mono 10', Cascadia Mono NF",
        "size": 12,
      },
    },
  },
}`)
	clean := stripTrailingCommas(stripJSONComments(src))
	var settings wtSettings
	if err := json.Unmarshal(clean, &settings); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, clean)
	}
	if got := settings.Profiles.Defaults.Font.Face; got != "'NewComputerModern Mono 10', Cascadia Mono NF" {
		t.Errorf("face = %q", got)
	}
	if got := settings.Profiles.Defaults.Font.Size; got != 12 {
		t.Errorf("size = %v", got)
	}
	if got := firstFontFace(settings.Profiles.Defaults.Font.Face); got != "NewComputerModern Mono 10" {
		t.Errorf("first face = %q", got)
	}
}

func TestRenderSixelUsesCellSizeOverride(t *testing.T) {
	t.Setenv("ICEDREEL_CELL_SIZE", "8x21")
	img := image.NewRGBA(image.Rect(0, 0, 100, 150))
	var buf bytes.Buffer
	if err := renderSixel(&buf, img, 60); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	const prefix = `"1;1;100;`
	start := strings.Index(out, prefix)
	if start < 0 {
		t.Fatalf("missing raster header: %q", out[:min(80, len(out))])
	}
	rest := out[start+len(prefix):]
	end := strings.IndexAny(rest, "#;")
	if end < 0 {
		t.Fatalf("unterminated raster header: %q", rest[:min(80, len(rest))])
	}
	height, err := strconv.Atoi(rest[:end])
	if err != nil {
		t.Fatalf("height %q: %v", rest[:end], err)
	}
	if height != 114 {
		t.Errorf("raster height = %d, want 114", height)
	}
}

func TestFontNameCandidates(t *testing.T) {
	got := fontNameCandidates("NewComputerModern Mono 10")
	want := map[string]bool{
		"NewComputerModern Mono 10": true,
		"NewComputerModernMono10":   true,
	}
	for _, name := range got {
		delete(want, name)
	}
	if len(want) != 0 {
		t.Errorf("missing candidates: %v (got %v)", want, got)
	}
}

func TestRenderHalfBlockCellAspect(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))
	got := renderHalfBlockCell(img, 50, 9, 20)
	if rows := strings.Count(got, "\n"); rows != 45 {
		t.Errorf("rows = %d, want 45", rows)
	}
}

func TestEncodeSixel(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := encodeSixel(&buf, img); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "\x1bP") || !strings.HasSuffix(out, "\x1b\\") {
		t.Errorf("not a sixel stream: %q", out)
	}
}
