package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-sixel"
	"golang.org/x/image/draw"
)

const tmdbPosterBase = "https://image.tmdb.org/t/p/w500"

type posterMode int

const (
	posterAuto posterMode = iota
	posterSixel
	posterHalf
	posterNone
)

type posterOptions struct {
	Width    int
	WidthSet bool
	Mode     posterMode
	NoPoster bool
}

func parsePosterMode(s string) (posterMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return posterAuto, nil
	case "sixel":
		return posterSixel, nil
	case "half", "block":
		return posterHalf, nil
	case "none", "off":
		return posterNone, nil
	default:
		return posterAuto, fmt.Errorf("--image 只能是 auto、sixel、half 或 none")
	}
}

// posterSource prefers a local poster next to the video, then TMDB.
func posterSource(meta *BilingualTitle, mediaPath string) string {
	dir := filepath.Dir(mediaPath)
	for _, name := range []string{"poster.jpg", "poster.png", "folder.jpg", "folder.png"} {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	if meta != nil && meta.PosterPath != "" {
		return tmdbPosterBase + meta.PosterPath
	}
	return ""
}

func posterSourceForMode(source string, mode posterMode) string {
	if mode != posterSixel {
		return source
	}
	const w500Base = "https://image.tmdb.org/t/p/w500/"
	const originalBase = "https://image.tmdb.org/t/p/original/"
	const w780Base = "https://image.tmdb.org/t/p/w780/"
	if strings.HasPrefix(source, w500Base) {
		return w780Base + strings.TrimPrefix(source, w500Base)
	}
	if strings.HasPrefix(source, originalBase) {
		return w780Base + strings.TrimPrefix(source, originalBase)
	}
	return source
}

func loadPoster(source string) (image.Image, error) {
	var data []byte
	var err error
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		data, err = cachedDownload(source)
	} else {
		data, err = os.ReadFile(source)
	}
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("图片解码失败：%w", err)
	}
	return img, nil
}

func cachedDownload(url string) ([]byte, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "IcedReel", "images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	sum := sha1.Sum([]byte(url))
	cachePath := filepath.Join(dir, hex.EncodeToString(sum[:])+".img")
	if data, err := os.ReadFile(cachePath); err == nil {
		return data, nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "IcedReel/0.1 (personal CLI)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("海报下载失败：HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}

	tmp, err := os.CreateTemp(dir, ".poster-*.tmp")
	if err != nil {
		return data, nil
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err == nil {
		_ = tmp.Close()
		_ = os.Chmod(tmpName, 0o644)
		_ = os.Rename(tmpName, cachePath)
	} else {
		_ = tmp.Close()
	}
	return data, nil
}

// renderHalfBlock renders an image using upper-half block characters:
// one terminal cell holds two vertical pixels, with foreground/background
// truecolor. This works in Windows Terminal and any truecolor terminal.
func renderHalfBlock(img image.Image, width int) string {
	return renderHalfBlockCell(img, width, 8, 16)
}

func renderHalfBlockCell(img image.Image, width, cellWidth, cellHeight int) string {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return ""
	}
	if width < 4 {
		width = 4
	}
	if cellWidth <= 0 || cellHeight <= 0 {
		cellWidth, cellHeight = 8, 16
	}
	// Each cell stores two vertical pixels, so the pixel aspect ratio is
	// (cellWidth) : (cellHeight/2). Correct for fonts whose cells are not
	// exactly 1:2.
	pixelAspect := 2.0 * float64(cellWidth) / float64(cellHeight)
	pixelHeight := int(math.Round(float64(width) * float64(b.Dy()) / float64(b.Dx()) * pixelAspect))
	if pixelHeight < 2 {
		pixelHeight = 2
	}
	if pixelHeight%2 != 0 {
		pixelHeight++
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, pixelHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)

	var out strings.Builder
	for y := 0; y < pixelHeight; y += 2 {
		for x := 0; x < width; x++ {
			tr, tg, tb, _ := dst.At(x, y).RGBA()
			br, bg, bb, _ := dst.At(x, y+1).RGBA()
			fmt.Fprintf(&out, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				tr>>8, tg>>8, tb>>8, br>>8, bg>>8, bb>>8)
		}
		out.WriteString("\x1b[0m\n")
	}
	return out.String()
}

func sixelAvailable() bool {
	if sixel.IsSupported() {
		return true
	}
	// Windows Terminal sets WT_SESSION. Its 1.22+ builds support sixel even
	// when the DA1 query cannot reach the console (for example under a
	// wrapper that redirects stdout).
	return os.Getenv("WT_SESSION") != ""
}

func sixelTargetSize(cols, cellWidth, cellHeight int, windowsTerminal bool) (int, float64) {
	if cellWidth <= 0 || cellHeight <= 0 {
		cellWidth, cellHeight = 10, 20
	}
	if !windowsTerminal {
		return cols * cellWidth, 1
	}
	// Windows Terminal maps sixel pixels onto a fixed 10x20 virtual cell
	// grid. Compensate when the real font cell is not 10x20 so the image
	// keeps its aspect ratio.
	targetWidth := cols * 10
	correction := (float64(cellWidth) / 10.0) * (20.0 / float64(cellHeight))
	return targetWidth, correction
}

// renderSixel writes a true bitmap using the sixel protocol. The image is
// scaled to the requested terminal width, using the terminal cell size when
// available. It never upscales a smaller source image.
func renderSixel(w io.Writer, img image.Image, cols int) error {
	isWT := os.Getenv("WT_SESSION") != ""
	cellWidth, cellHeight := posterCellSize()
	targetWidth, correction := sixelTargetSize(cols, cellWidth, cellHeight, isWT)
	scaled := scalePosterToSize(img, targetWidth, correction)
	return encodeSixel(w, scaled)
}

func encodeSixel(w io.Writer, img image.Image) error {
	enc := sixel.NewEncoder(w)
	enc.Dither = true
	enc.Colors = 256
	return enc.Encode(img)
}

func scalePosterToSize(img image.Image, targetWidth int, correction float64) image.Image {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || targetWidth <= 0 {
		return img
	}
	if correction <= 0 {
		correction = 1
	}
	if b.Dx() <= targetWidth && math.Abs(correction-1) < 1e-9 {
		return img
	}
	if b.Dx() < targetWidth {
		targetWidth = b.Dx()
	}
	targetHeight := int(math.Round(float64(targetWidth) * float64(b.Dy()) / float64(b.Dx()) * correction))
	if targetHeight < 1 {
		targetHeight = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

func scalePosterToWidth(img image.Image, targetWidth int) image.Image {
	return scalePosterToSize(img, targetWidth, 1)
}

func showPoster(w io.Writer, source string, opts posterOptions, stderr io.Writer) bool {
	if opts.NoPoster || opts.Mode == posterNone || source == "" || os.Getenv("NO_COLOR") != "" {
		return false
	}
	mode := posterDisplayMode(opts)
	img, err := loadPoster(posterSourceForMode(source, mode))
	if err != nil {
		fmt.Fprintf(stderr, "提示：海报不可用：%v\n", err)
		return false
	}
	return renderPosterImage(w, img, mode, opts, stderr)
}

func showPosterPair(w io.Writer, posterSource, fanartSource string, opts posterOptions, stderr io.Writer) bool {
	if fanartSource == posterSource {
		fanartSource = ""
	}
	if opts.NoPoster || opts.Mode == posterNone || (posterSource == "" && fanartSource == "") || os.Getenv("NO_COLOR") != "" {
		return false
	}
	if !opts.WidthSet && opts.Width < 60 {
		opts.Width = 60
	}
	mode := posterDisplayMode(opts)

	var posterImg, fanartImg image.Image
	if posterSource != "" {
		img, err := loadPoster(posterSourceForMode(posterSource, mode))
		if err != nil {
			fmt.Fprintf(stderr, "提示：海报不可用：%v\n", err)
		} else {
			posterImg = img
		}
	}
	if fanartSource != "" {
		img, err := loadPoster(posterSourceForMode(fanartSource, mode))
		if err != nil {
			fmt.Fprintf(stderr, "提示：fanart 不可用：%v\n", err)
		} else {
			fanartImg = img
		}
	}

	switch {
	case posterImg == nil && fanartImg == nil:
		return false
	case posterImg == nil:
		return renderPosterImage(w, fanartImg, mode, opts, stderr)
	case fanartImg == nil:
		return renderPosterImage(w, posterImg, mode, opts, stderr)
	default:
		return renderPosterImage(w, composePosterPair(posterImg, fanartImg), mode, opts, stderr)
	}
}

func posterDisplayMode(opts posterOptions) posterMode {
	if opts.Mode != posterAuto {
		return opts.Mode
	}
	if sixelAvailable() {
		return posterSixel
	}
	return posterHalf
}

func renderPosterImage(w io.Writer, img image.Image, mode posterMode, opts posterOptions, stderr io.Writer) bool {
	switch mode {
	case posterSixel:
		cols := opts.Width
		if !opts.WidthSet {
			cols = 60
		}
		var buf bytes.Buffer
		if err := renderSixel(&buf, img, cols); err != nil {
			fmt.Fprintf(stderr, "提示：Sixel 渲染失败，改用半块字符：%v\n", err)
			cellWidth, cellHeight := posterCellSize()
			fmt.Fprint(w, renderHalfBlockCell(img, opts.Width, cellWidth, cellHeight))
			fmt.Fprintln(w)
			return true
		}
		_, _ = w.Write(buf.Bytes())
		fmt.Fprintln(w)
	default:
		cellWidth, cellHeight := posterCellSize()
		fmt.Fprint(w, renderHalfBlockCell(img, opts.Width, cellWidth, cellHeight))
		fmt.Fprintln(w)
	}
	return true
}

func composePosterPair(poster, fanart image.Image) image.Image {
	const (
		maxHeight = 600
		gap       = 16
	)
	if poster == nil || poster.Bounds().Dx() <= 0 || poster.Bounds().Dy() <= 0 {
		return fanart
	}
	if fanart == nil || fanart.Bounds().Dx() <= 0 || fanart.Bounds().Dy() <= 0 {
		return poster
	}

	height := maxHeight
	if h := poster.Bounds().Dy(); h < height {
		height = h
	}
	if h := fanart.Bounds().Dy(); h < height {
		height = h
	}
	posterWidth := scaledPosterWidth(poster, height)
	fanartWidth := scaledPosterWidth(fanart, height)
	if posterWidth <= 0 || fanartWidth <= 0 {
		return poster
	}

	dst := image.NewRGBA(image.Rect(0, 0, posterWidth+gap+fanartWidth, height))
	draw.Draw(dst, dst.Bounds(), image.Black, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, image.Rect(0, 0, posterWidth, height), poster, poster.Bounds(), draw.Over, nil)
	draw.CatmullRom.Scale(dst, image.Rect(posterWidth+gap, 0, posterWidth+gap+fanartWidth, height), fanart, fanart.Bounds(), draw.Over, nil)
	return dst
}

func scaledPosterWidth(img image.Image, height int) int {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || height <= 0 {
		return 0
	}
	return max(1, int(math.Round(float64(height)*float64(b.Dx())/float64(b.Dy()))))
}
