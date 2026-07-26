package editor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/philipparndt/delivr/internal/config"
)

// Edits are the in-flight changes to one screen: what the user has dragged or
// typed, held in memory rather than written to disk.
//
// Every field is a pointer so "not touched" is distinguishable from "set back
// to zero" — which matters, because 0 is a value someone might genuinely want
// and is also the value that starts this whole tool's reason for existing.
type Edits struct {
	// Devices is keyed by the device's index within the screen.
	Devices  map[int]*DeviceEdit `json:"devices,omitempty"`
	Title    *TextEdit           `json:"title,omitempty"`
	Subtitle *TextEdit           `json:"subtitle,omitempty"`

	// Added are devices that do not exist in the config yet, appended after
	// the ones that do. Their indices continue from the existing devices, so
	// the rest of the editor can address them the same way.
	Added []*NewDevice `json:"added,omitempty"`
}

// NewDevice is a device being added to a screen.
//
// The common reason to add one is the overlapping pair: a screen that shows the
// neighbouring screenshot's device continuing across the seam needs a second
// entry pointing at the *other* screen's screenshot and frame.
type NewDevice struct {
	Source        string  `json:"source"`
	Template      string  `json:"template,omitempty"`
	AutoCrop      bool    `json:"autoCrop"`
	CropThreshold int     `json:"cropThreshold,omitempty"`
	Width         int     `json:"width,omitempty"`
	Height        int     `json:"height,omitempty"`
	X             float64 `json:"x"`
	Y             float64 `json:"y"`
}

func (n *NewDevice) toConfig() config.DeviceImage {
	return config.DeviceImage{
		Source: n.Source, Template: n.Template,
		AutoCrop: n.AutoCrop, CropThreshold: n.CropThreshold,
		Width: n.Width, Height: n.Height, X: n.X, Y: n.Y,
	}
}

// DeviceEdit is the positioning of one device.
type DeviceEdit struct {
	X      *float64 `json:"x,omitempty"`
	Y      *float64 `json:"y,omitempty"`
	Height *int     `json:"height,omitempty"`
}

// TextEdit is a title or subtitle block.
type TextEdit struct {
	Text     *string  `json:"text,omitempty"`
	Y        *float64 `json:"y,omitempty"`
	Size     *float64 `json:"size,omitempty"`
	MaxWidth *float64 `json:"maxWidth,omitempty"`
}

func (e *Edits) device(i int) *DeviceEdit {
	if e == nil || e.Devices == nil {
		return nil
	}
	return e.Devices[i]
}

// Apply returns a copy of a screen with the edits applied, leaving the loaded
// config untouched. A nil Edits is "nothing changed".
func (e *Edits) Apply(screen *config.Screen) *config.Screen {
	if e == nil {
		return screen
	}
	out := *screen

	// Adding a device turns a single `device:` into a list, which is what the
	// config would have to become on disk too.
	existing := DevicesOf(&out)
	if len(e.Added) > 0 || len(out.Devices) > 0 {
		devices := make([]config.DeviceImage, 0, len(existing)+len(e.Added))
		for _, d := range existing {
			devices = append(devices, *d)
		}
		// Appended, never inserted: devices render back to front, so a new one
		// lands on top — which is what the overlapping-pair case wants — and,
		// more importantly, the existing devices keep the indices the rest of
		// the editor has already addressed them by.
		for _, n := range e.Added {
			devices = append(devices, n.toConfig())
		}

		for i := range devices {
			applyDevice(&devices[i], e.device(i))
		}
		out.Devices = devices
		out.Device = nil
	} else if out.Device != nil {
		d := *out.Device
		applyDevice(&d, e.device(0))
		out.Device = &d
	}

	if out.Title != nil {
		t := *out.Title
		applyText(&t, e.Title)
		out.Title = &t
	}
	if out.Subtitle != nil {
		s := *out.Subtitle
		applyText(&s, e.Subtitle)
		out.Subtitle = &s
	}

	return &out
}

func applyDevice(d *config.DeviceImage, e *DeviceEdit) {
	if e == nil {
		return
	}
	if e.X != nil {
		d.X = *e.X
	}
	if e.Y != nil {
		d.Y = *e.Y
	}
	if e.Height != nil {
		d.Height = *e.Height
	}
}

func applyText(t *config.TextConfig, e *TextEdit) {
	if e == nil {
		return
	}
	if e.Text != nil {
		t.Text = *e.Text
	}
	if e.Y != nil {
		t.Y = *e.Y
	}
	if e.Size != nil {
		t.Size = *e.Size
	}
	if e.MaxWidth != nil {
		t.MaxWidth = *e.MaxWidth
	}
}

// DevicesOf flattens a screen's device configuration into a list, so the rest
// of the editor does not have to care whether the screen used `device:` or
// `devices:`.
func DevicesOf(screen *config.Screen) []*config.DeviceImage {
	if len(screen.Devices) > 0 {
		out := make([]*config.DeviceImage, len(screen.Devices))
		for i := range screen.Devices {
			out[i] = &screen.Devices[i]
		}
		return out
	}
	if screen.Device != nil {
		return []*config.DeviceImage{screen.Device}
	}
	return nil
}

// YAML renders the edits as the YAML fragment they correspond to: only what
// changed, under the right keys, ready to paste.
func (e *Edits) YAML(screen *config.Screen, screenID string) string {
	if e == nil {
		return "# nothing changed yet"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", screenID)

	devices := DevicesOf(screen)
	existing := len(devices) - len(e.Added)
	multi := len(devices) > 1

	var wroteAny bool
	for i := 0; i < existing && i < len(devices); i++ {
		ed := e.device(i)
		if ed == nil || (ed.X == nil && ed.Y == nil && ed.Height == nil) {
			continue
		}
		wroteAny = true
		indent := "  "
		if multi {
			fmt.Fprintf(&b, "devices:\n  - # %s\n", devices[i].Source)
			indent = "    "
		} else {
			b.WriteString("device:\n")
		}
		if ed.Height != nil {
			fmt.Fprintf(&b, "%sheight: %d\n", indent, *ed.Height)
		}
		if ed.X != nil {
			fmt.Fprintf(&b, "%sx: %s\n", indent, num(*ed.X))
		}
		if ed.Y != nil {
			fmt.Fprintf(&b, "%sy: %s\n", indent, num(*ed.Y))
		}
	}

	// A device being added is written out whole, since there is nothing on
	// disk to merge it into.
	if len(e.Added) > 0 {
		wroteAny = true
		if existing == 1 && len(screen.Devices) == 0 {
			b.WriteString("# `device:` becomes `devices:` — the existing entry moves under it\n")
		}
		b.WriteString("devices:\n")
		for i, d := range devices {
			if i < existing {
				fmt.Fprintf(&b, "  - # existing: %s\n", d.Source)
				continue
			}
			fmt.Fprintf(&b, "  - source: %s\n", quoteYAML(d.Source))
			if d.Template != "" {
				fmt.Fprintf(&b, "    template: %s\n", quoteYAML(d.Template))
			}
			if d.AutoCrop {
				b.WriteString("    auto_crop: true\n")
			}
			if d.CropThreshold != 0 {
				fmt.Fprintf(&b, "    crop_threshold: %d\n", d.CropThreshold)
			}
			if d.Width != 0 {
				fmt.Fprintf(&b, "    width: %d\n", d.Width)
			}
			if d.Height != 0 {
				fmt.Fprintf(&b, "    height: %d\n", d.Height)
			}
			fmt.Fprintf(&b, "    x: %s\n    y: %s\n", num(d.X), num(d.Y))
		}
	}

	for _, t := range []struct {
		key  string
		edit *TextEdit
	}{{"title", e.Title}, {"subtitle", e.Subtitle}} {
		if t.edit == nil {
			continue
		}
		var lines []string
		if t.edit.Text != nil {
			lines = append(lines, fmt.Sprintf("  text: %s", quoteYAML(*t.edit.Text)))
		}
		if t.edit.Size != nil {
			lines = append(lines, fmt.Sprintf("  size: %s", num(*t.edit.Size)))
		}
		if t.edit.Y != nil {
			lines = append(lines, fmt.Sprintf("  y: %s", num(*t.edit.Y)))
		}
		if t.edit.MaxWidth != nil {
			lines = append(lines, fmt.Sprintf("  max_width: %s", num(*t.edit.MaxWidth)))
		}
		if len(lines) == 0 {
			continue
		}
		wroteAny = true
		fmt.Fprintf(&b, "%s:\n%s\n", t.key, strings.Join(lines, "\n"))
	}

	if !wroteAny {
		return "# nothing changed yet"
	}
	return b.String()
}

// num prints a float the way a person would write it in YAML: 170 rather than
// 170.000000, but 170.5 when that is what it is.
func num(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func quoteYAML(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// added is the pending new devices, nil-safe.
func (e *Edits) added() []*NewDevice {
	if e == nil {
		return nil
	}
	return e.Added
}
