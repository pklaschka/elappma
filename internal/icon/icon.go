// Package icon finds an app's icon: in its .app bundle when it is installed,
// else in its repository, where Electron, Wails and Tauri projects keep the
// file they build the app icon from. Icons come out as small PNGs (or SVG,
// which stays as it is).
package icon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // icons in JPEG are rare but decode for free
	"image/png"
	"path"
	"strings"
)

// Size is the edge in pixels icons are scaled down to: the list shows them
// at 36 points, so this covers Retina with room to spare.
const Size = 128

// MaxFile is the largest file read as an icon.
const MaxFile = 4 << 20

// ErrNone means there is no usable icon.
var ErrNone = errors.New("no icon")

// Icon is an image ready to show.
type Icon struct {
	MIME string `json:"mime"` // image/png or image/svg+xml
	Data []byte `json:"data"`
}

// Normalize turns the bytes of a file called name into an Icon: PNG and JPEG
// are scaled down to Size, ICNS gives up its largest PNG, SVG is kept.
func Normalize(data []byte, name string) (Icon, error) {
	if len(data) == 0 || len(data) > MaxFile {
		return Icon{}, ErrNone
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".svg":
		if !bytes.Contains(data[:min(len(data), 4096)], []byte("<svg")) {
			return Icon{}, ErrNone
		}
		return Icon{MIME: "image/svg+xml", Data: data}, nil
	case ".icns":
		png, ok := FromICNS(data)
		if !ok {
			return Icon{}, ErrNone
		}
		data = png
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Icon{}, ErrNone
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, Shrink(img, Size)); err != nil {
		return Icon{}, err
	}
	return Icon{MIME: "image/png", Data: buf.Bytes()}, nil
}

// FromICNS returns the largest PNG an .icns file holds. Icons made since
// macOS 10.7 keep their large sizes (ic07 to ic14) as PNG; older ones that
// only have the RLE formats report false.
func FromICNS(data []byte) ([]byte, bool) {
	if len(data) < 8 || string(data[:4]) != "icns" {
		return nil, false
	}
	var best []byte
	bestSize := 0
	for off := 8; off+8 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[off+4 : off+8]))
		if n < 8 || off+n > len(data) {
			break
		}
		body := data[off+8 : off+n]
		if bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")) {
			if cfg, err := png.DecodeConfig(bytes.NewReader(body)); err == nil && cfg.Width > bestSize {
				best, bestSize = body, cfg.Width
			}
		}
		off += n
	}
	return best, best != nil
}

// Shrink scales img down so neither side exceeds max, averaging the pixels
// each target pixel covers. Smaller images are returned as they are.
func Shrink(img image.Image, max int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return img
	}
	tw, th := max, max
	if w > h {
		th = max * h / w
	} else if h > w {
		tw = max * w / h
	}
	src := image.NewNRGBA(b)
	draw.Draw(src, b, img, b.Min, draw.Src)
	dst := image.NewNRGBA(image.Rect(0, 0, tw, th))
	for y := 0; y < th; y++ {
		y0, y1 := y*h/th, max1((y+1)*h/th, y*h/th+1)
		for x := 0; x < tw; x++ {
			x0, x1 := x*w/tw, max1((x+1)*w/tw, x*w/tw+1)
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					c := src.NRGBAAt(b.Min.X+sx, b.Min.Y+sy)
					// weight colour by alpha so transparent edges do not darken
					r += uint64(c.R) * uint64(c.A)
					g += uint64(c.G) * uint64(c.A)
					bl += uint64(c.B) * uint64(c.A)
					a += uint64(c.A)
					n++
				}
			}
			if a == 0 {
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{R: uint8(r / a), G: uint8(g / a), B: uint8(bl / a), A: uint8(a / n)})
		}
	}
	return dst
}

func max1(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// File is one file in a repository's tree.
type File struct {
	Path string
	Size int64
}

// Pick chooses the file in a repository that most likely is the app icon,
// or reports false. It knows where electron-builder (build/icon.png),
// Electron Forge (assets/icon.png), Wails (build/appicon.png) and Tauri
// (src-tauri/icons/) keep theirs, and takes a plain icon or logo elsewhere.
func Pick(files []File) (string, bool) {
	best, bestScore := "", 0
	for _, f := range files {
		if s := score(f); s > bestScore {
			best, bestScore = f.Path, s
		}
	}
	return best, best != ""
}

func score(f File) int {
	p := strings.ToLower(f.Path)
	if f.Size <= 0 || f.Size > MaxFile {
		return 0
	}
	dir, file := path.Split(p)
	ext := path.Ext(file)
	base := strings.TrimSuffix(file, ext)

	s := 0
	switch ext {
	case ".png":
		s = 30
	case ".svg":
		s = 25
	case ".icns":
		s = 20 // may hold only the old formats, which need sips
	default:
		return 0
	}

	switch {
	case base == "icon" || base == "appicon" || base == "app-icon" || base == "app_icon":
		s += 40
	case base == "128x128@2x" || base == "icon@2x" || base == "256x256" || base == "512x512" || base == "icon-512" || base == "icon-256":
		s += 35
	case base == "logo" || base == "app" || base == "icons" || strings.HasPrefix(base, "icon-") || strings.HasPrefix(base, "icon_"):
		s += 20
	case strings.HasPrefix(base, "logo"):
		s += 12
	default:
		return 0
	}
	// icons for files, the tray or the browser tab are not the app's icon
	for _, w := range []string{"tray", "favicon", "file-icon", "dmg", "template", "badge", "notification", "toolbar"} {
		if strings.Contains(base, w) {
			return 0
		}
	}

	for _, part := range strings.Split(strings.Trim(dir, "/"), "/") {
		switch part {
		case "node_modules", "vendor", "test", "tests", "__tests__", "fixtures", "testdata", "examples", "example", "dist", "out", "release":
			return 0
		}
		if strings.HasPrefix(part, ".") && part != "" {
			return 0
		}
	}
	switch strings.Trim(dir, "/") {
	case "build", "src-tauri/icons", "build/icons", "buildresources", "build-resources":
		s += 25
	case "assets", "resources", "icons", "assets/icons", "resources/icons", "public", "static", "src/assets", "app/assets", "electron/assets", "build/darwin":
		s += 15
	case "":
		s += 12
	default:
		s += 5
	}
	s -= strings.Count(p, "/") // shallower wins a tie
	return s
}
