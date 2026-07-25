package render

import (
	"image/color"
	"math"

	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/fonts"
)

// lineSpacing is the multiple of the font's line height used for wrapped text.
const lineSpacing = 1.4

// RenderText renders title and subtitle text
func RenderText(dc *gg.Context, textCfg *config.TextConfig, fontLoader *fonts.Loader) error {
	if textCfg == nil || textCfg.Text == "" {
		return nil
	}

	face, err := fontLoader.Load(textCfg.Font, textCfg.Size)
	if err != nil {
		return err
	}

	dc.SetFontFace(face)

	c, err := parseHexColor(textCfg.Color)
	if err != nil {
		return err
	}

	// Calculate X position based on alignment
	var x float64
	var ax float64 // anchor x (0=left, 0.5=center, 1=right)

	switch textCfg.Align {
	case "left":
		x = textCfg.X
		ax = 0
	case "right":
		x = float64(dc.Width()) + textCfg.X
		ax = 1
	case "center":
		fallthrough
	default:
		x = float64(dc.Width())/2 + textCfg.X
		ax = 0.5
	}

	// The backdrop goes down first, or it would cover the text it exists to
	// make readable.
	if textCfg.Backdrop != nil {
		if err := renderTextBackdrop(dc, textCfg, x, ax); err != nil {
			return err
		}
	}

	dc.SetColor(c)

	// Use word wrapping if max_width is specified
	if textCfg.MaxWidth > 0 {
		dc.DrawStringWrapped(textCfg.Text, x, textCfg.Y, ax, 0, textCfg.MaxWidth, lineSpacing, gg.AlignCenter)
	} else {
		dc.DrawStringAnchored(textCfg.Text, x, textCfg.Y, ax, 0.5)
	}

	return nil
}

// textBounds returns the box the text will occupy, in the same coordinates the
// draw calls use.
func textBounds(dc *gg.Context, cfg *config.TextConfig, x, ax float64) (x0, y0, x1, y1 float64) {
	if cfg.MaxWidth > 0 {
		// DrawStringWrapped anchors the block by its top edge (ay = 0).
		w, h := dc.MeasureMultilineString(wrapped(dc, cfg), lineSpacing)
		return x - w*ax, cfg.Y, x - w*ax + w, cfg.Y + h
	}
	w, h := dc.MeasureString(cfg.Text)
	// DrawStringAnchored with ay = 0.5 centres on Y.
	return x - w*ax, cfg.Y - h/2, x - w*ax + w, cfg.Y + h/2
}

// wrapped reproduces the line breaking DrawStringWrapped will apply, so the
// backdrop is measured against the text that actually gets drawn.
func wrapped(dc *gg.Context, cfg *config.TextConfig) string {
	lines := dc.WordWrap(cfg.Text, cfg.MaxWidth)
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

func renderTextBackdrop(dc *gg.Context, cfg *config.TextConfig, x, ax float64) error {
	bd := cfg.Backdrop
	fill, err := parseHexColor(bd.Color)
	if err != nil {
		return err
	}

	x0, y0, x1, y1 := textBounds(dc, cfg, x, ax)

	switch bd.Type {
	case "panel":
		pad := bd.Padding
		dc.SetColor(fill)
		dc.DrawRoundedRectangle(x0-pad, y0-pad, (x1-x0)+pad*2, (y1-y0)+pad*2, bd.Radius)
		dc.Fill()

	default: // "scrim"
		// Fade from whichever edge the text sits nearest, so a headline at the
		// top and a caption at the bottom both get a scrim that reads as
		// vignetting rather than as a band floating in the middle.
		height := bd.Height
		fromTop := y0 < float64(dc.Height())/2
		if height <= 0 {
			// Reach just past the text so the copy sits in the opaque part.
			if fromTop {
				height = y1 * 1.35
			} else {
				height = (float64(dc.Height()) - y0) * 1.35
			}
		}
		height = math.Max(height, 1)

		r, g, b, a := fill.RGBA()
		opaque := color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
		clear := color.NRGBA{opaque.R, opaque.G, opaque.B, 0}

		var grad gg.Gradient
		if fromTop {
			grad = gg.NewLinearGradient(0, 0, 0, height)
			grad.AddColorStop(0, opaque)
			grad.AddColorStop(1, clear)
			dc.SetFillStyle(grad)
			dc.DrawRectangle(0, 0, float64(dc.Width()), height)
		} else {
			top := float64(dc.Height()) - height
			grad = gg.NewLinearGradient(0, top, 0, float64(dc.Height()))
			grad.AddColorStop(0, clear)
			grad.AddColorStop(1, opaque)
			dc.SetFillStyle(grad)
			dc.DrawRectangle(0, top, float64(dc.Width()), height)
		}
		dc.Fill()
	}

	return nil
}
