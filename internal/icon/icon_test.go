package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 120, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPick(t *testing.T) {
	files := func(paths ...string) []File {
		var out []File
		for _, p := range paths {
			out = append(out, File{Path: p, Size: 1000})
		}
		return out
	}
	for _, tc := range []struct {
		files []string
		want  string
	}{
		{[]string{"README.md", "build/icon.png", "docs/logo.png"}, "build/icon.png"},
		{[]string{"build/appicon.png", "build/darwin/icons.icns", "docs/icon.png", "build/darwin/tray-template.png"}, "build/appicon.png"},
		{[]string{"src-tauri/icons/32x32.png", "src-tauri/icons/128x128@2x.png", "src-tauri/icons/icon.icns"}, "src-tauri/icons/128x128@2x.png"},
		{[]string{"node_modules/x/icon.png", "assets/icon.svg"}, "assets/icon.svg"},
		{[]string{"public/favicon.png", ".github/icon.png", "src/main.ts"}, ""},
		{[]string{"logo.png", "resources/icon.icns"}, "resources/icon.icns"},
	} {
		got, ok := Pick(files(tc.files...))
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("Pick(%v) = %q, %v; want %q", tc.files, got, ok, tc.want)
		}
	}
	if _, ok := Pick([]File{{Path: "build/icon.png", Size: MaxFile + 1}}); ok {
		t.Error("an oversized file was picked")
	}
}

func TestNormalizeShrinks(t *testing.T) {
	ic, err := Normalize(pngOf(t, 1024, 512), "build/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(ic.Data))
	if err != nil {
		t.Fatal(err)
	}
	if ic.MIME != "image/png" || cfg.Width != Size || cfg.Height != Size/2 {
		t.Errorf("got %s %dx%d", ic.MIME, cfg.Width, cfg.Height)
	}
}

func TestNormalizeSVGAndJunk(t *testing.T) {
	ic, err := Normalize([]byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`), "icon.svg")
	if err != nil || ic.MIME != "image/svg+xml" {
		t.Errorf("svg: %v %v", ic.MIME, err)
	}
	if _, err := Normalize([]byte("not an image"), "icon.png"); err != ErrNone {
		t.Errorf("junk: %v", err)
	}
	if _, err := Normalize([]byte("<html>"), "icon.svg"); err != ErrNone {
		t.Errorf("not svg: %v", err)
	}
}

func TestFromICNS(t *testing.T) {
	small, large := pngOf(t, 32, 32), pngOf(t, 256, 256)
	var buf bytes.Buffer
	entry := func(typ string, body []byte) {
		buf.WriteString(typ)
		binary.Write(&buf, binary.BigEndian, uint32(len(body)+8))
		buf.Write(body)
	}
	entry("ic11", small)
	entry("ic08", large)
	entry("is32", []byte{1, 2, 3}) // an old RLE entry
	data := append([]byte("icns"), binary.BigEndian.AppendUint32(nil, uint32(buf.Len()+8))...)
	data = append(data, buf.Bytes()...)

	got, ok := FromICNS(data)
	if !ok || !bytes.Equal(got, large) {
		t.Fatalf("FromICNS did not return the largest PNG (ok=%v, %d bytes)", ok, len(got))
	}
	ic, err := Normalize(data, "icons.icns")
	if err != nil || ic.MIME != "image/png" {
		t.Errorf("Normalize icns: %v", err)
	}
	if _, ok := FromICNS([]byte("nope")); ok {
		t.Error("junk read as icns")
	}
}
