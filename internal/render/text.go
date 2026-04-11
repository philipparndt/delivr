package render

import (
	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/fonts"
)

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
	dc.SetColor(c)

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

	// Use word wrapping if max_width is specified
	if textCfg.MaxWidth > 0 {
		dc.DrawStringWrapped(textCfg.Text, x, textCfg.Y, ax, 0, textCfg.MaxWidth, 1.4, gg.AlignCenter)
	} else {
		dc.DrawStringAnchored(textCfg.Text, x, textCfg.Y, ax, 0.5)
	}

	return nil
}
