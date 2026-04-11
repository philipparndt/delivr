package render

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
)

// RenderBackground renders a solid or gradient background
func RenderBackground(dc *gg.Context, bg *config.Background) error {
	switch bg.Type {
	case "solid":
		return renderSolidBackground(dc, bg.Color)
	case "gradient":
		return renderGradientBackground(dc, bg.Angle, bg.Stops)
	default:
		return fmt.Errorf("unknown background type: %s", bg.Type)
	}
}

func renderSolidBackground(dc *gg.Context, hexColor string) error {
	c, err := parseHexColor(hexColor)
	if err != nil {
		return err
	}
	dc.SetColor(c)
	dc.Clear()
	return nil
}

func renderGradientBackground(dc *gg.Context, angleDeg float64, stops []config.GradientStop) error {
	if len(stops) < 2 {
		return fmt.Errorf("gradient requires at least 2 stops")
	}

	w := float64(dc.Width())
	h := float64(dc.Height())

	// Convert angle to radians (0 = top-down, 90 = left-right)
	angleRad := angleDeg * math.Pi / 180.0

	// Calculate gradient direction vector
	dx := math.Sin(angleRad)
	dy := math.Cos(angleRad)

	// Calculate gradient start and end points
	// Project corners onto the gradient direction to find bounds
	diagonal := math.Sqrt(w*w + h*h)
	cx, cy := w/2, h/2

	x0 := cx - dx*diagonal/2
	y0 := cy - dy*diagonal/2
	x1 := cx + dx*diagonal/2
	y1 := cy + dy*diagonal/2

	// Create linear gradient
	grad := gg.NewLinearGradient(x0, y0, x1, y1)

	for _, stop := range stops {
		c, err := parseHexColor(stop.Color)
		if err != nil {
			return fmt.Errorf("invalid gradient stop color: %w", err)
		}
		grad.AddColorStop(stop.Pos, c)
	}

	dc.SetFillStyle(grad)
	dc.DrawRectangle(0, 0, w, h)
	dc.Fill()

	return nil
}

// parseHexColor parses a hex color string (#RGB, #RGBA, #RRGGBB, #RRGGBBAA)
func parseHexColor(hex string) (color.Color, error) {
	hex = strings.TrimPrefix(hex, "#")

	var r, g, b, a uint8 = 0, 0, 0, 255

	switch len(hex) {
	case 3: // RGB
		r = parseHexByte(hex[0:1] + hex[0:1])
		g = parseHexByte(hex[1:2] + hex[1:2])
		b = parseHexByte(hex[2:3] + hex[2:3])
	case 4: // RGBA
		r = parseHexByte(hex[0:1] + hex[0:1])
		g = parseHexByte(hex[1:2] + hex[1:2])
		b = parseHexByte(hex[2:3] + hex[2:3])
		a = parseHexByte(hex[3:4] + hex[3:4])
	case 6: // RRGGBB
		r = parseHexByte(hex[0:2])
		g = parseHexByte(hex[2:4])
		b = parseHexByte(hex[4:6])
	case 8: // RRGGBBAA
		r = parseHexByte(hex[0:2])
		g = parseHexByte(hex[2:4])
		b = parseHexByte(hex[4:6])
		a = parseHexByte(hex[6:8])
	default:
		return nil, fmt.Errorf("invalid hex color: %s", hex)
	}

	return color.NRGBA{R: r, G: g, B: b, A: a}, nil
}

func parseHexByte(s string) uint8 {
	val, _ := strconv.ParseUint(s, 16, 8)
	return uint8(val)
}
