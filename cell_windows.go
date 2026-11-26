//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/mattn/go-sixel"
)

const (
	defaultCellWidth  = 8
	defaultCellHeight = 16

	// Windows Terminal reports a fixed 10x20 virtual cell for sixel graphics,
	// even when the real font cell has a different aspect ratio.
	wtVirtualCellWidth  = 10
	wtVirtualCellHeight = 20

	logPixelsY = 90
)

// posterCellSize returns the terminal cell size used to correct image aspect.
// Priority: explicit override, Windows Terminal font metrics, terminal query.
func posterCellSize() (int, int) {
	if w, h, ok := cellSizeOverride(); ok {
		return w, h
	}
	if os.Getenv("WT_SESSION") != "" {
		if w, h, err := windowsTerminalCellSize(); err == nil && w > 0 && h > 0 {
			return w, h
		}
	}
	if w, h, err := sixel.CellSize(); err == nil && w > 0 && h > 0 {
		return w, h
	}
	if os.Getenv("WT_SESSION") != "" {
		return wtVirtualCellWidth, wtVirtualCellHeight
	}
	return defaultCellWidth, defaultCellHeight
}

func cellSizeOverride() (int, int, bool) {
	s := strings.TrimSpace(os.Getenv("ICEDREEL_CELL_SIZE"))
	if s == "" {
		return 0, 0, false
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == 'x' || r == 'X' || r == ',' || r == ' ' || r == '\t'
	})
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, errW := strconv.Atoi(parts[0])
	h, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

type wtFont struct {
	Face string  `json:"face"`
	Size float64 `json:"size"`
}

type wtProfile struct {
	GUID string `json:"guid"`
	Font wtFont `json:"font"`
}

type wtSettings struct {
	Profiles struct {
		Defaults struct {
			Font wtFont `json:"font"`
		} `json:"defaults"`
		List []wtProfile `json:"list"`
	} `json:"profiles"`
}

func windowsTerminalCellSize() (int, int, error) {
	face, size, err := windowsTerminalFont()
	if err != nil {
		return 0, 0, err
	}
	return measureFontCell(face, size)
}

func windowsTerminalFont() (string, float64, error) {
	path := ""
	for _, candidate := range windowsTerminalSettingsPaths() {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			path = candidate
			break
		}
	}
	if path == "" {
		return "", 0, errors.New("未找到 Windows Terminal settings.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	data = stripJSONComments(data)
	data = stripTrailingCommas(data)

	var settings wtSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return "", 0, fmt.Errorf("解析 Windows Terminal 设置失败：%w", err)
	}
	face := settings.Profiles.Defaults.Font.Face
	size := settings.Profiles.Defaults.Font.Size
	id := strings.TrimSpace(os.Getenv("WT_PROFILE_ID"))
	for _, p := range settings.Profiles.List {
		if id != "" && strings.EqualFold(strings.TrimSpace(p.GUID), id) {
			if p.Font.Face != "" {
				face = p.Font.Face
			}
			if p.Font.Size > 0 {
				size = p.Font.Size
			}
			break
		}
	}
	face = firstFontFace(face)
	if face == "" {
		return "", 0, errors.New("Windows Terminal 设置中没有字体")
	}
	if size <= 0 {
		size = 12
	}
	return face, size, nil
}

func windowsTerminalSettingsPaths() []string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil
	}
	packages := []string{
		"Microsoft.WindowsTerminal_8wekyb3d8bbwe",
		"Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe",
		"Microsoft.WindowsTerminalCanary_8wekyb3d8bbwe",
	}
	paths := make([]string, 0, len(packages)+1)
	for _, pkg := range packages {
		paths = append(paths, filepath.Join(local, "Packages", pkg, "LocalState", "settings.json"))
	}
	paths = append(paths, filepath.Join(local, "Microsoft", "Windows Terminal", "settings.json"))
	return paths
}

// firstFontFace takes the first face from a Windows Terminal font list such as
// "'NewComputerModern Mono 10', Cascadia Mono NF".
func firstFontFace(list string) string {
	var b strings.Builder
	var quote rune
	for _, r := range list {
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			b.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ',':
			return strings.TrimSpace(b.String())
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// stripJSONComments removes // and /* */ comments outside strings.
func stripJSONComments(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inString := false
	escaped := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(src) {
			switch src[i+1] {
			case '/':
				for i < len(src) && src[i] != '\n' {
					i++
				}
				if i < len(src) {
					out = append(out, '\n')
				}
				continue
			case '*':
				i += 2
				for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
					i++
				}
				i++
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// stripTrailingCommas removes commas before } or ] outside strings.
func stripTrailingCommas(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inString := false
	escaped := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r' || src[j] == '\n') {
				j++
			}
			if j < len(src) && (src[j] == '}' || src[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

type gdiTextMetric struct {
	TmHeight           int32
	TmAscent           int32
	TmDescent          int32
	TmInternalLeading  int32
	TmExternalLeading  int32
	TmAveCharWidth     int32
	TmMaxCharWidth     int32
	TmWeight           int32
	TmOverhang         int32
	TmDigitizedAspectX int32
	TmDigitizedAspectY int32
	TmFirstChar        uint16
	TmLastChar         uint16
	TmDefaultChar      uint16
	TmBreakChar        uint16
	TmItalic           byte
	TmUnderlined       byte
	TmStruckOut        byte
	TmPitchAndFamily   byte
	TmCharSet          byte
}

type gdiSize struct {
	Cx int32
	Cy int32
}

var (
	gdiUser32 = syscall.NewLazyDLL("user32.dll")
	gdi32DLL  = syscall.NewLazyDLL("gdi32.dll")

	procGetDC                = gdiUser32.NewProc("GetDC")
	procReleaseDC            = gdiUser32.NewProc("ReleaseDC")
	procGetDeviceCaps        = gdi32DLL.NewProc("GetDeviceCaps")
	procCreateFontW          = gdi32DLL.NewProc("CreateFontW")
	procSelectObject         = gdi32DLL.NewProc("SelectObject")
	procDeleteObject         = gdi32DLL.NewProc("DeleteObject")
	procGetTextFaceW         = gdi32DLL.NewProc("GetTextFaceW")
	procGetTextMetricsW      = gdi32DLL.NewProc("GetTextMetricsW")
	procGetTextExtentPoint32 = gdi32DLL.NewProc("GetTextExtentPoint32W")
)

// measureFontCell measures the advance width and line height of a font at the
// given point size, in pixels at the display DPI.
func measureFontCell(face string, points float64) (int, int, error) {
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return 0, 0, errors.New("GetDC 失败")
	}
	defer procReleaseDC.Call(0, hdc)

	dpiY, _, _ := procGetDeviceCaps.Call(hdc, logPixelsY)
	if dpiY == 0 {
		dpiY = 96
	}
	height := -int32(math.Round(points * float64(dpiY) / 72))
	if height >= 0 {
		height = -16
	}

	for _, name := range fontNameCandidates(face) {
		namePtr, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		hfont, _, _ := procCreateFontW.Call(
			uintptr(height), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0,
			uintptr(unsafe.Pointer(namePtr)),
		)
		if hfont == 0 {
			continue
		}
		old, _, _ := procSelectObject.Call(hdc, hfont)
		actual := gdiTextFace(hdc)
		if normalizeFontName(actual) != normalizeFontName(name) {
			procSelectObject.Call(hdc, old)
			procDeleteObject.Call(hfont)
			continue
		}

		var tm gdiTextMetric
		procGetTextMetricsW.Call(hdc, uintptr(unsafe.Pointer(&tm)))
		text, _ := syscall.UTF16PtrFromString("M")
		var sz gdiSize
		procGetTextExtentPoint32.Call(hdc, uintptr(unsafe.Pointer(text)), 1, uintptr(unsafe.Pointer(&sz)))

		procSelectObject.Call(hdc, old)
		procDeleteObject.Call(hfont)

		width := int(sz.Cx)
		cellHeight := int(tm.TmHeight + tm.TmExternalLeading)
		if width <= 0 || cellHeight <= 0 {
			continue
		}
		return width, cellHeight, nil
	}
	return 0, 0, fmt.Errorf("无法度量字体：%s", face)
}

func gdiTextFace(hdc uintptr) string {
	var buf [64]uint16
	n, _, _ := procGetTextFaceW.Call(hdc, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if n == 0 {
		return ""
	}
	if int(n) > len(buf) {
		n = uintptr(len(buf))
	}
	return syscall.UTF16ToString(buf[:n])
}

func fontNameCandidates(face string) []string {
	face = strings.TrimSpace(face)
	seen := make(map[string]bool)
	var names []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		names = append(names, s)
	}
	add(face)
	add(strings.ReplaceAll(face, " ", ""))
	add(strings.ReplaceAll(face, "-", ""))
	add(strings.NewReplacer(" ", "", "-", "").Replace(face))
	return names
}

func normalizeFontName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer(" ", "", "-", "", "_", "", "'", "", `"`, "").Replace(s)
}
