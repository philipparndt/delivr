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

	x, ax := textAnchor(dc, textCfg)

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

// textAnchor returns the draw x and the horizontal anchor for a text block.
func textAnchor(dc *gg.Context, textCfg *config.TextConfig) (x, ax float64) {
	switch textCfg.Align {
	case "left":
		return textCfg.X, 0
	case "right":
		return float64(dc.Width()) + textCfg.X, 1
	case "center":
		fallthrough
	default:
		return float64(dc.Width())/2 + textCfg.X, 0.5
	}
}

// AnchorTop and AnchorMiddle name the two vertical anchoring modes a text block
// can be in. Which one applies is decided by whether max_width is set, and the
// difference is half the block's height — so adding a max_width to stop a title
// overflowing also drops it, often straight onto the subtitle.
const (
	// AnchorTop: DrawStringWrapped puts y at the TOP of the block.
	AnchorTop = "top"
	// AnchorMiddle: DrawStringAnchored centres the block ON y.
	AnchorMiddle = "middle"
)

// TextMetrics is where a text block actually lands, measured with the same
// code that draws it.
type TextMetrics struct {
	X0, Y0, X1, Y1 float64 `json:"-"`
	// Anchor is AnchorTop or AnchorMiddle — which of the two vertical
	// anchoring rules this block is subject to.
	Anchor string `json:"anchor"`
	// Lines is the text as it will break, so a stranded word shows up before
	// the render does.
	Lines []string `json:"lines"`
	// Overflow is set when the block escapes the canvas on any side. Without a
	// max_width an overlong string does not wrap, it runs off both edges.
	Overflow bool `json:"overflow"`
	// Wrapped reports whether max_width is in play at all.
	Wrapped bool `json:"wrapped"`
}

// MeasureText reports where a text block will be drawn, without drawing it.
//
// It goes through the same textBounds the backdrop uses, so the measurement and
// the render agree by construction — including the anchoring split above, which
// is otherwise invisible until two lines of copy collide.
//
// The font face is set on dc as a side effect, exactly as RenderText would.
func MeasureText(dc *gg.Context, textCfg *config.TextConfig, fontLoader *fonts.Loader) (TextMetrics, error) {
	if textCfg == nil || textCfg.Text == "" {
		return TextMetrics{}, nil
	}

	face, err := fontLoader.Load(textCfg.Font, textCfg.Size)
	if err != nil {
		return TextMetrics{}, err
	}
	dc.SetFontFace(face)

	x, ax := textAnchor(dc, textCfg)
	x0, y0, x1, y1 := textBounds(dc, textCfg, x, ax)

	m := TextMetrics{
		X0: x0, Y0: y0, X1: x1, Y1: y1,
		Anchor:  AnchorMiddle,
		Wrapped: textCfg.MaxWidth > 0,
		Lines:   []string{textCfg.Text},
	}
	if m.Wrapped {
		m.Anchor = AnchorTop
		m.Lines = dc.WordWrap(textCfg.Text, textCfg.MaxWidth)
	}
	m.Overflow = x0 < 0 || y0 < 0 ||
		x1 > float64(dc.Width()) || y1 > float64(dc.Height())

	return m, nil
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
