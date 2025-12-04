package image

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"

	resvg "github.com/kanrichan/resvg-go"
	"golang.org/x/image/draw"
)

var (
	resvgCtx  *resvg.Context
	resvgOnce sync.Once
	resvgMu   sync.Mutex
)

func getResvgContext() *resvg.Context {
	resvgOnce.Do(func() {
		ctx, err := resvg.NewContext(context.Background())
		if err == nil {
			resvgCtx = ctx
		}
	})
	return resvgCtx
}

// RasterizeSVG converts SVG to raster image using resvg (full SVG support including gradients)
// Preserves transparency
func RasterizeSVG(svgBytes []byte, width, height int) (image.Image, error) {
	svgBytes = preprocessSVG(svgBytes)

	ctx := getResvgContext()
	if ctx == nil {
		return nil, fmt.Errorf("resvg not available")
	}

	resvgMu.Lock()
	defer resvgMu.Unlock()

	renderer, err := ctx.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("renderer: %w", err)
	}
	defer renderer.Close()

	pngData, err := renderer.RenderWithSize(svgBytes, uint32(width), uint32(height))
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	// Convert to RGBA but preserve transparency
	return toRGBA(img), nil
}

func preprocessSVG(data []byte) []byte {
	s := string(data)
	if !strings.Contains(s, "xmlns") && strings.Contains(s, "<svg") {
		s = strings.Replace(s, "<svg", `<svg xmlns="http://www.w3.org/2000/svg"`, 1)
	}
	s = strings.ReplaceAll(s, "currentColor", "#333333")
	return []byte(s)
}

// toRGBA converts any image to RGBA preserving transparency
func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)
	draw.Draw(result, bounds, img, bounds.Min, draw.Src)
	return result
}

// compositeOnWhite composites an image onto a white background (removes transparency)
func compositeOnWhite(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)
	fillWithWhite(result)
	draw.Draw(result, bounds, img, bounds.Min, draw.Over)
	return result
}

func fillWithWhite(img *image.RGBA) {
	white := color.RGBA{255, 255, 255, 255}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = white.R, white.G, white.B, white.A
	}
}

func IsNearlyBlank(img image.Image) bool {
	if img == nil {
		return true
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return true
	}
	stepX, stepY := max(w/20, 1), max(h/20, 1)
	count := 0
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r, g, bb, a := img.At(x, y).RGBA()
			if a >= 0x8000 {
				r8, g8, b8 := r>>8, g>>8, bb>>8
				if !(r8 > 250 && g8 > 250 && b8 > 250) {
					count++
					if count > 3 {
						return false
					}
				}
			}
		}
	}
	return true
}

func IsNearlyBlankOrBlack(img image.Image) bool {
	if img == nil {
		return true
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return true
	}
	stepX, stepY := max(w/20, 1), max(h/20, 1)
	colored, opaque := 0, 0
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r, g, bb, a := img.At(x, y).RGBA()
			if a >= 0x8000 {
				opaque++
				r8, g8, b8 := r>>8, g>>8, bb>>8
				isBlack := r8 < 15 && g8 < 15 && b8 < 15
				isWhite := r8 > 240 && g8 > 240 && b8 > 240
				if !isBlack && !isWhite {
					colored++
				}
			}
		}
	}
	return opaque < 5 || colored < 3
}

func ResizeImage(img image.Image, size int) image.Image {
	bounds := img.Bounds()
	if bounds.Dx() == size && bounds.Dy() == size {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	// Transparent background
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	return dst
}

func ResizeImageWithBackground(img image.Image, size int, bgColor color.Color) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

func CreateFallbackImage(size int) (image.Image, error) {
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 100 100">
  <circle cx="50" cy="50" r="45" fill="#e3f2fd" stroke="#1976d2" stroke-width="2"/>
  <ellipse cx="50" cy="50" rx="45" ry="20" fill="none" stroke="#1976d2" stroke-width="1"/>
  <ellipse cx="50" cy="50" rx="20" ry="45" fill="none" stroke="#1976d2" stroke-width="1"/>
</svg>`, size, size)
	return RasterizeSVG([]byte(svg), size, size)
}

func CreateBlankImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	// Transparent
	return img
}

func EnsureOpaque(img image.Image) image.Image {
	return compositeOnWhite(img)
}

func HasVisibleContent(img image.Image) bool {
	return !IsNearlyBlank(img)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
