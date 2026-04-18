package theme

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sort"
)

// Color represents an RGB color with its dominance weight.
type Color struct {
	R, G, B uint8
	Weight  float64
}

// Hex returns the hex string representation of the color.
func (c Color) Hex() string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// IsDark returns true if the color is perceptually dark.
func (c Color) IsDark() bool {
	// Relative luminance (WCAG formula)
	r := linearize(float64(c.R) / 255.0)
	g := linearize(float64(c.G) / 255.0)
	b := linearize(float64(c.B) / 255.0)
	luminance := 0.2126*r + 0.7152*g + 0.0722*b
	return luminance < 0.4
}

func linearize(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// TextColor returns white or black hex string depending on background darkness.
func (c Color) TextColor() string {
	if c.IsDark() {
		return "#FFFFFF"
	}
	return "#000000"
}

// ColorDescription returns a human-readable description for use in prompts.
func (c Color) ColorDescription() string {
	return fmt.Sprintf("%s (%s)", namedColor(c), c.Hex())
}

type rgbPixel struct{ r, g, b uint8 }

// Extract extracts the dominant colors from image data using a simple
// k-means style quantization (downsampled for performance).
func Extract(imgData []byte, n int) ([]Color, error) {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Max.X - bounds.Min.X
	h := bounds.Max.Y - bounds.Min.Y

	// Sample every Nth pixel for performance
	step := max(1, min(w, h)/64)

	// Collect sampled pixels
	var pixels []rgbPixel
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			if a < 0x8000 {
				continue // skip transparent
			}
			pixels = append(pixels, rgbPixel{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)})
		}
	}

	if len(pixels) == 0 {
		return nil, fmt.Errorf("no pixels sampled")
	}

	// Simple k-means color quantization
	centers := kMeans(pixels, n, 10)

	// Sort by weight descending
	sort.Slice(centers, func(i, j int) bool {
		return centers[i].Weight > centers[j].Weight
	})

	return centers, nil
}

func kMeans(pixels []rgbPixel, k, iterations int) []Color {
	if k > len(pixels) {
		k = len(pixels)
	}

	// Initialize centers from evenly spaced pixels
	centers := make([]Color, k)
	step := len(pixels) / k
	for i := range centers {
		p := pixels[i*step]
		centers[i] = Color{R: p.r, G: p.g, B: p.b}
	}

	assignments := make([]int, len(pixels))

	for iter := 0; iter < iterations; iter++ {
		// Assign pixels to nearest center
		for i, p := range pixels {
			nearest := 0
			minDist := colorDist(p.r, p.g, p.b, centers[0].R, centers[0].G, centers[0].B)
			for j := 1; j < k; j++ {
				d := colorDist(p.r, p.g, p.b, centers[j].R, centers[j].G, centers[j].B)
				if d < minDist {
					minDist = d
					nearest = j
				}
			}
			assignments[i] = nearest
		}

		// Recompute centers
		sums := make([][3]float64, k)
		counts := make([]int, k)
		for i, p := range pixels {
			j := assignments[i]
			sums[j][0] += float64(p.r)
			sums[j][1] += float64(p.g)
			sums[j][2] += float64(p.b)
			counts[j]++
		}
		for j := range centers {
			if counts[j] == 0 {
				continue
			}
			centers[j].R = uint8(sums[j][0] / float64(counts[j]))
			centers[j].G = uint8(sums[j][1] / float64(counts[j]))
			centers[j].B = uint8(sums[j][2] / float64(counts[j]))
			centers[j].Weight = float64(counts[j]) / float64(len(pixels))
		}
	}

	return centers
}

func colorDist(r1, g1, b1, r2, g2, b2 uint8) float64 {
	dr := float64(int(r1) - int(r2))
	dg := float64(int(g1) - int(g2))
	db := float64(int(b1) - int(b2))
	return dr*dr + dg*dg + db*db
}

// namedColor returns a rough English name for a color.
func namedColor(c Color) string {
	r, g, b := float64(c.R), float64(c.G), float64(c.B)
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	brightness := max / 255.0

	if brightness < 0.15 {
		return "deep black"
	}
	if brightness > 0.9 && (max-min) < 30 {
		return "bright white"
	}
	if max-min < 20 {
		if brightness < 0.4 {
			return "dark gray"
		}
		return "light gray"
	}

	// Determine hue
	switch {
	case r > g && r > b:
		if g > b*1.5 {
			return "warm orange"
		}
		if brightness < 0.4 {
			return "deep red"
		}
		return "vivid red"
	case g > r && g > b:
		if r > b {
			return "yellow green"
		}
		if brightness < 0.4 {
			return "dark green"
		}
		return "fresh green"
	case b > r && b > g:
		if r > g {
			return "violet blue"
		}
		if brightness < 0.4 {
			return "midnight blue"
		}
		return "sky blue"
	case r > b && g > b:
		return "golden yellow"
	case r > g && b > g:
		return "magenta"
	default:
		return "cyan"
	}
}

// BuildColorDescription builds a prompt-ready color description string.
func BuildColorDescription(colors []Color) string {
	if len(colors) == 0 {
		return "rich and varied tones"
	}
	primary := colors[0]
	desc := primary.ColorDescription()
	if len(colors) > 1 {
		desc += " and " + colors[1].ColorDescription()
	}
	return desc
}

// Dominant returns the most dominant color from image data.
func Dominant(imgData []byte) (Color, error) {
	colors, err := Extract(imgData, 5)
	if err != nil {
		return Color{}, err
	}
	if len(colors) == 0 {
		return Color{R: 0, G: 0, B: 0}, nil
	}
	return colors[0], nil
}

// clamp helper (Go 1.21+)
func clamp[T int | float64](v, lo, hi T) T {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RGBA implements color.Color for Color.
func (c Color) RGBA() (r, g, b, a uint32) {
	return color.RGBA{R: c.R, G: c.G, B: c.B, A: 255}.RGBA()
}
