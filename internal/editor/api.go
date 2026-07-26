package editor

import (
	"fmt"
	"image"
	"math"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/fogleman/gg"

	"github.com/philipparndt/delivr/internal/config"
	"github.com/philipparndt/delivr/internal/geometry"
	"github.com/philipparndt/delivr/internal/render"
)

// StateResponse is what the page needs to populate its pickers.
type StateResponse struct {
	Config    string         `json:"config"`
	ReadOnly  bool           `json:"readOnly"`
	Languages []string       `json:"languages"`
	Outputs   []OutputInfo   `json:"outputs"`
	Screens   []ScreenInfo   `json:"screens"`
	Templates []TemplateInfo `json:"templates"`
	// Catalog is every device already configured anywhere in the project, so a
	// new one can be added by copying an existing entry rather than by typing
	// a path. The overlap case wants exactly this: the device to add to screen
	// two is the one screen one already has.
	Catalog []CatalogDevice `json:"catalog"`
}

// TemplateInfo is one template and what it drives.
type TemplateInfo struct {
	Name    string   `json:"name"`
	Screens []string `json:"screens"`
	Devices int      `json:"devices"`
}

// CatalogDevice is an existing device somewhere in the config.
type CatalogDevice struct {
	Screen        string  `json:"screen"`
	Index         int     `json:"index"`
	Source        string  `json:"source"`
	Template      string  `json:"template"`
	AutoCrop      bool    `json:"autoCrop"`
	CropThreshold int     `json:"cropThreshold"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	X             float64 `json:"x"`
	Y             float64 `json:"y"`
}

// OutputInfo is one store slot: a device and the screens rendered for it.
type OutputInfo struct {
	Device  string   `json:"device"`
	Name    string   `json:"name"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Screens []string `json:"screens"`
}

// ScreenInfo is one screen in the config.
type ScreenInfo struct {
	ID       string `json:"id"`
	Index    int    `json:"index"`
	Template string `json:"template"`
	Devices  int    `json:"devices"`
}

// PreviewRequest renders a whole store row: every screenshot in an output,
// laid out in order with the carousel gap between them.
//
// One screen at a time was the wrong unit. The layout questions this tool
// exists to answer are about the row — is this device centred *relative to its
// neighbours*, does that phone continue across the boundary, does the copy sit
// at a consistent height down the page — and a viewport showing one card with
// an optional partner makes all of them into a memory test.
type PreviewRequest struct {
	// Screen is the one whose details the panel shows. Every screen renders
	// regardless; this only says which is selected.
	Screen string `json:"screen"`
	Output string `json:"output"`
	Lang   string `json:"lang"`

	// Edits are keyed by screen id. Every screen in the row is live.
	Edits map[string]*Edits `json:"edits"`

	// Gap between screenshots; -1 for the store default of 4%.
	Gap float64 `json:"gap"`

	// MaxWidth and MaxHeight bound the display scale. The render itself is
	// always at full canvas size.
	MaxWidth  int `json:"maxWidth"`
	MaxHeight int `json:"maxHeight"`

	// Zoom overrides the fit-to-window scale when non-zero.
	Zoom float64 `json:"zoom"`

	// Order, when set, previews the row in a different arrangement — a pending
	// reorder, not yet written.
	Order []string `json:"order"`
}

// PreviewResponse carries the rendered image plus every measurement the
// overlays draw from. The image is the truth about pixels; this is the truth
// about where things are.
type PreviewResponse struct {
	// Layers is the preview as a stack the browser composites, so dragging a
	// device is a CSS offset rather than a round trip. PNG is the same thing
	// flattened, kept for the seam view's neighbour and as a fallback.
	Layers []Layer `json:"layers"`
	// Cards are the screenshot rectangles inside the frame. Each screen's
	// layers are clipped to its own card, exactly as drawing onto a canvas of
	// that size clips them — a device that runs off the edge must not appear
	// on the screenshot next door.
	Cards []Card `json:"cards"`
	PNG   string `json:"png,omitempty"`
	// Canvas is one screenshot's size.
	Canvas [2]int `json:"canvas"`
	// Frame is the whole rendered area, which in seam view is two screenshots
	// and the gap between them.
	Frame   [2]int         `json:"frame"`
	Shown   [2]int         `json:"shown"`
	Scale   float64        `json:"scale"`
	Millis  int64          `json:"millis"`
	Devices []DeviceReport `json:"devices"`
	Texts   []TextReport   `json:"texts"`
	YAML    string         `json:"yaml"`
	// Seams is one report per adjacent pair in the row.
	Seams []SeamReport `json:"seams"`
	Gap   float64      `json:"gap"`
	// CaptionStrip is the room left under each card for its name, in screen
	// pixels. The fit scale already accounts for it.
	CaptionStrip int    `json:"captionStrip"`
	Warning      string `json:"warning,omitempty"`
	// Template is the template this screen inherits from, and the screens that
	// share it — so an edit's blast radius is visible before it is made.
	Template        string   `json:"template,omitempty"`
	TemplateScreens []string `json:"templateScreens,omitempty"`
}

// Card is one screenshot's rectangle within the frame.
type Card struct {
	Screen string `json:"screen"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	W      int    `json:"w"`
	H      int    `json:"h"`
}

// DeviceReport is one device's resolved values and measured geometry.
type DeviceReport struct {
	// Screen is which of the two screens in a seam view this device is on.
	Screen   string `json:"screen"`
	Index    int    `json:"index"`
	Source   string `json:"source"`
	Template string `json:"template"`

	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  int     `json:"width"`
	Height int     `json:"height"`

	AutoCrop bool `json:"autoCrop"`
	// Added marks a device that exists only as a pending edit, so the UI can
	// say it has to be written out whole rather than nudged.
	Added bool `json:"added"`

	Placement geometry.Placement `json:"placement"`

	// Shadow is how far the crop box reaches past the body on each side, in
	// canvas pixels. The asymmetry here is the reason x: 0 is not centred.
	Shadow geometry.Margins `json:"shadow"`

	// Solved are the answers the buttons would apply, shown live so the size
	// of the correction is visible before anyone commits to it.
	SolvedX     float64 `json:"solvedX"`
	SolvedBleed float64 `json:"solvedBleed"`

	Origins map[string]*Origin `json:"origins"`
	Error   string             `json:"error,omitempty"`
}

// TextReport is one text block's measured bounds and anchoring.
type TextReport struct {
	Screen   string     `json:"screen"`
	Kind     string     `json:"kind"`
	Text     string     `json:"text"`
	Box      [4]float64 `json:"box"`
	Anchor   string     `json:"anchor"`
	Lines    []string   `json:"lines"`
	Overflow bool       `json:"overflow"`
	Wrapped  bool       `json:"wrapped"`
	Y        float64    `json:"y"`
	Size     float64    `json:"size"`
	MaxWidth float64    `json:"maxWidth"`

	Origins map[string]*Origin `json:"origins"`
}

// SeamReport describes the two-screen view and how well a shared device lines
// up across the boundary.
type SeamReport struct {
	// Left and Right are the two screenshots either side of this boundary.
	Left    string      `json:"left"`
	Right   string      `json:"right"`
	Matches []SeamMatch `json:"matches"`
}

// SeamMatch is one device that appears in both screens.
//
// A device spanning a seam has to agree with its other half on all three of x,
// y and height. x is the interesting one — it carries the card width and the
// carousel gap — but y and height are the ones that get missed, because they
// look like they are already the same in the YAML. They frequently are not:
// `x` and `y` are additive when a screen overrides a template's single
// `device:`, and not merged at all when the screen declares a `devices:` array,
// so two entries reading the same in source can render 400px apart.
type SeamMatch struct {
	// Screen and Index identify the device that would move to fix this.
	Screen          string `json:"screen"`
	Index           int    `json:"index"`
	NeighbourScreen string `json:"neighbourScreen"`
	NeighbourIndex  int    `json:"neighbourIndex"`
	Source          string `json:"source"`
	// Error is the horizontal continuation error: 0 means the two halves meet.
	Error   float64 `json:"error"`
	SolvedX float64 `json:"solvedX"`
	// ErrorY and HeightDelta must both be 0, since it is meant to be one
	// object. NeighbourY and NeighbourHeight are what to match them to.
	ErrorY          float64 `json:"errorY"`
	HeightDelta     int     `json:"heightDelta"`
	NeighbourY      float64 `json:"neighbourY"`
	NeighbourHeight int     `json:"neighbourHeight"`
	// Continuous is true only when all three agree.
	Continuous bool `json:"continuous"`
}

// State enumerates what can be edited.
func (s *Server) State() StateResponse {
	cfg := s.project.Config()

	langs := append([]string{""}, cfg.Languages...)
	out := StateResponse{
		Config:    s.project.Rel(s.project.ConfigPath),
		ReadOnly:  s.readOnly,
		Languages: langs,
	}

	for _, o := range cfg.Outputs {
		dev := cfg.Devices[o.Device]
		out.Outputs = append(out.Outputs, OutputInfo{
			Device: o.Device, Name: dev.Name,
			Width: dev.Width, Height: dev.Height,
			Screens: o.Screens,
		})
	}
	byTemplate := map[string][]string{}
	for i := range cfg.Screens {
		sc := &cfg.Screens[i]
		devices := DevicesOf(sc)
		out.Screens = append(out.Screens, ScreenInfo{
			ID: sc.ID, Index: i, Template: sc.Template,
			Devices: len(devices),
		})
		if sc.Template != "" {
			byTemplate[sc.Template] = append(byTemplate[sc.Template], sc.ID)
		}
		for j, d := range devices {
			out.Catalog = append(out.Catalog, CatalogDevice{
				Screen: sc.ID, Index: j, Source: d.Source, Template: d.Template,
				AutoCrop: d.AutoCrop, CropThreshold: d.CropThreshold,
				Width: d.Width, Height: d.Height, X: d.X, Y: d.Y,
			})
		}
	}

	for name, tmpl := range cfg.Templates {
		info := TemplateInfo{Name: name, Screens: byTemplate[name]}
		if tmpl.Device != nil {
			info.Devices = 1
		}
		if len(tmpl.Devices) > 0 {
			info.Devices = len(tmpl.Devices)
		}
		out.Templates = append(out.Templates, info)
	}
	sort.Slice(out.Templates, func(i, j int) bool {
		return out.Templates[i].Name < out.Templates[j].Name
	})

	return out
}

// Preview renders the whole row and measures everything on it.
func (s *Server) Preview(req PreviewRequest) (*PreviewResponse, error) {
	started := time.Now()
	cfg := s.project.Config()

	output, device, err := s.resolveOutput(req.Output, req.Screen)
	if err != nil {
		return nil, err
	}
	canvas := image.Pt(device.Width, device.Height)

	ids := output.Screens
	if len(ids) == 0 {
		return nil, fmt.Errorf("output %q renders no screens", output.Device)
	}
	if len(req.Order) > 0 {
		reordered, err := permute(ids, req.Order)
		if err != nil {
			return nil, err
		}
		ids = reordered
	}

	gap := req.Gap
	if gap < 0 {
		gap = geometry.SeamGapFor(canvas.X)
	}

	deviceName := device.ScreenshotPrefix
	if deviceName == "" {
		deviceName = output.Device
	}

	step := canvas.X + int(gap)
	resp := &PreviewResponse{
		Canvas: [2]int{canvas.X, canvas.Y},
		Frame:  [2]int{canvas.X*len(ids) + int(gap)*(len(ids)-1), canvas.Y},
		Gap:    gap,
	}

	// Every screen is built, and they are independent, so build them at once.
	// A seven-screen row is seven frame composites; in series that is a
	// three-second wait before anything appears.
	type built struct {
		layers []Layer
		used   []*cachedImage
		screen *config.Screen
		index  int
		err    error
	}
	out := make([]built, len(ids))

	var wg sync.WaitGroup
	sem := make(chan struct{}, max(2, runtime.NumCPU()/2))
	for i, id := range ids {
		base := cfg.GetScreen(id)
		if base == nil {
			return nil, fmt.Errorf("unknown screen: %s", id)
		}
		wg.Add(1)
		go func(i int, id string, base *config.Screen) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Localise first, then apply the edits: an edit to the title is an
			// edit to what is on screen now, not to the string it came from.
			screen := req.Edits[id].Apply(s.renderer.ScreenForLang(base, req.Lang))
			layers, used, err := s.buildLayers(
				screen, id, device, deviceName, image.Pt(i*step, 0), req.Lang)
			out[i] = built{layers, used, screen, screenIndexOf(cfg, id), err}
		}(i, id, base)
	}
	wg.Wait()

	for i, b := range out {
		if b.err != nil {
			return nil, b.err
		}
		id := ids[i]
		resp.Layers = append(resp.Layers, b.layers...)
		resp.Cards = append(resp.Cards, Card{Screen: id, X: i * step, W: canvas.X, H: canvas.Y})
		resp.Devices = append(resp.Devices, s.reportDevices(b.screen, id, b.used, canvas,
			deviceName, b.index, len(DevicesOf(b.screen))-len(req.Edits[id].added()))...)
		resp.Texts = append(resp.Texts, s.reportTexts(b.screen, id, canvas,
			cfg.GetScreen(id), b.index)...)
	}

	// Every boundary in the row, not just the one next to the selection.
	for i := 0; i+1 < len(out); i++ {
		if r := seamBetween(ids[i], out[i].screen, ids[i+1], out[i+1].screen, canvas.X, gap); r != nil {
			resp.Seams = append(resp.Seams, *r)
		}
	}

	if sel := cfg.GetScreen(req.Screen); sel != nil && sel.Template != "" {
		resp.Template = sel.Template
		resp.TemplateScreens = s.screensUsing(sel.Template)
	}
	if e := req.Edits[req.Screen]; e != nil {
		if i := indexOf(ids, req.Screen); i >= 0 {
			resp.YAML = e.YAML(out[i].screen, req.Screen)
		}
	}
	if resp.YAML == "" {
		resp.YAML = "# nothing changed yet"
	}

	resp.CaptionStrip = captionStripPx
	resp.Scale = req.Zoom
	if resp.Scale <= 0 {
		// Fit the card's height *and* the strip its name is written in. Fitting
		// the card alone makes the row exactly as tall as the window, which puts
		// every caption one strip below the fold.
		resp.Scale = displayScale(canvas.X, canvas.Y,
			req.MaxWidth, req.MaxHeight-captionStripPx)
	}
	resp.Shown = [2]int{
		int(float64(resp.Frame[0]) * resp.Scale),
		int(float64(resp.Frame[1]) * resp.Scale),
	}
	resp.Millis = time.Since(started).Milliseconds()
	return resp, nil
}

// captionStripPx is the height reserved under each screenshot for its name.
const captionStripPx = 30

// seamBetween reports how a device shared by two adjacent screenshots lines up
// across the boundary between them.
//
// A device spanning a seam has to agree with its other half on all three of x,
// y and height. x is the interesting one — it carries the card width and the
// carousel gap — but y and height are the ones that get missed, because they
// look like they are already the same in the YAML. They frequently are not:
// `x` and `y` are added to a template's when a screen overrides `device:`, and
// not merged at all when the screen declares a `devices:` array, so two entries
// reading the same in source can render hundreds of pixels apart.
func seamBetween(leftID string, left *config.Screen, rightID string, right *config.Screen,
	canvasWidth int, gap float64) *SeamReport {

	report := &SeamReport{Left: leftID, Right: rightID}
	for i, d := range DevicesOf(right) {
		for j, o := range DevicesOf(left) {
			if d.Source != o.Source || d.Template != o.Template {
				continue
			}
			m := SeamMatch{
				Screen: rightID, Index: i,
				NeighbourScreen: leftID, NeighbourIndex: j, Source: d.Source,
				Error:           geometry.SeamError(o.X, d.X, canvasWidth, gap, true),
				SolvedX:         geometry.SeamMatchX(o.X, canvasWidth, gap, true),
				ErrorY:          d.Y - o.Y,
				HeightDelta:     d.Height - o.Height,
				NeighbourY:      o.Y,
				NeighbourHeight: o.Height,
			}
			m.Continuous = math.Abs(m.Error) < 0.5 && math.Abs(m.ErrorY) < 0.5 && m.HeightDelta == 0
			report.Matches = append(report.Matches, m)
		}
	}
	if len(report.Matches) == 0 {
		return nil
	}
	return report
}

// permute checks that a requested order is the same set of screens in a
// different arrangement, so a stale row cannot silently drop or duplicate one.
func permute(have, want []string) ([]string, error) {
	if len(have) != len(want) {
		return nil, fmt.Errorf("the row has %d screens, the requested order has %d",
			len(have), len(want))
	}
	seen := map[string]int{}
	for _, id := range have {
		seen[id]++
	}
	for _, id := range want {
		if seen[id] == 0 {
			return nil, fmt.Errorf("screen %q is not in this output", id)
		}
		seen[id]--
	}
	return append([]string(nil), want...), nil
}

// ReorderRequest rewrites the order of an output's screens.
type ReorderRequest struct {
	Output  string   `json:"output"`
	Screens []string `json:"screens"`
}

// Reorder writes a new store order for an output.
func (s *Server) Reorder(req ReorderRequest) (*ApplyResult, error) {
	if s.readOnly {
		return nil, fmt.Errorf("the editor was started with --read-only")
	}

	cfg := s.project.Config()
	var current []string
	for _, o := range cfg.Outputs {
		if o.Device == req.Output {
			current = o.Screens
		}
	}
	if current == nil {
		return nil, fmt.Errorf("unknown output: %s", req.Output)
	}
	if _, err := permute(current, req.Screens); err != nil {
		return nil, err
	}

	node, file := s.project.OutputScreensNode(req.Output)
	if node == nil {
		return nil, fmt.Errorf("could not find the screens list for output %q", req.Output)
	}

	// Express the new arrangement as indices into the old one, which is what
	// lets each entry keep the comments written about it.
	used := make([]bool, len(current))
	order := make([]int, 0, len(req.Screens))
	for _, id := range req.Screens {
		for i, have := range current {
			if have == id && !used[i] {
				used[i] = true
				order = append(order, i)
				break
			}
		}
	}

	src, err := readFile(file)
	if err != nil {
		return nil, err
	}
	out, err := ReorderSequence(src, node, order)
	if err != nil {
		return nil, err
	}
	if err := s.project.WriteFile(file, out); err != nil {
		return nil, err
	}
	s.cache.Reset()

	return &ApplyResult{File: s.project.Rel(file), Field: "screens"}, nil
}

func indexOf(list []string, v string) int {
	for i, s := range list {
		if s == v {
			return i
		}
	}
	return -1
}

// reportDevices measures each device with the geometry model, calibrated by
// what the renderer just did so the overlays are exact.
func (s *Server) reportDevices(screen *config.Screen, screenID string, used []*cachedImage,
	canvas image.Point, deviceName string, screenIndex int, existing int) []DeviceReport {

	cfg := s.project.Config()
	devices := DevicesOf(screen)
	reports := make([]DeviceReport, 0, len(devices))

	for i, d := range devices {
		r := DeviceReport{
			Screen: screenID, Index: i, Source: d.Source, Template: d.Template,
			X: d.X, Y: d.Y, Width: d.Width, Height: d.Height,
			AutoCrop: d.AutoCrop, Added: i >= existing,
		}

		f, err := s.cache.frame(d, cfg.Settings.ScreenshotsDir, deviceName)
		if err != nil {
			r.Error = err.Error()
			reports = append(reports, r)
			continue
		}
		if i < len(used) {
			f = f.Calibrated(&geometry.Calibration{
				ForWidth: d.Width, ForHeight: d.Height,
				Stage: used[i].info.Stage, Crop: used[i].info.Crop,
			})
		}

		r.Placement = geometry.Place(f, d, canvas)
		r.Shadow = geometry.Margins{
			Left:   r.Placement.Body.X0 - r.Placement.Content.X0,
			Right:  r.Placement.Content.X1 - r.Placement.Body.X1,
			Top:    r.Placement.Body.Y0 - r.Placement.Content.Y0,
			Bottom: r.Placement.Content.Y1 - r.Placement.Body.Y1,
		}
		r.SolvedX = geometry.CenterX(f, d, canvas)
		r.SolvedBleed = geometry.BleedBottom(f, d, canvas, 0)
		if !r.Added {
			r.Origins = s.originsFor(screen, screenIndex, i, "device", "x", "y", "height")
		}

		reports = append(reports, r)
	}
	return reports
}

// reportTexts measures the title and subtitle with the renderer's own
// measuring code, so what the overlay claims is what will be drawn.
func (s *Server) reportTexts(screen *config.Screen, screenID string, canvas image.Point,
	base *config.Screen, screenIndex int) []TextReport {
	dc := gg.NewContext(canvas.X, canvas.Y)
	var out []TextReport

	for _, t := range []struct {
		kind string
		cfg  *config.TextConfig
	}{{"title", screen.Title}, {"subtitle", screen.Subtitle}} {
		if t.cfg == nil || t.cfg.Text == "" {
			continue
		}
		m, err := render.MeasureText(dc, t.cfg, s.renderer.FontLoader())
		if err != nil {
			continue
		}
		out = append(out, TextReport{
			Screen: screenID, Kind: t.kind, Text: t.cfg.Text,
			Origins: s.originsFor(base, screenIndex, 0, t.kind,
				"text", "y", "size", "max_width", "font", "color", "align"),
			Box:      [4]float64{m.X0, m.Y0, m.X1, m.Y1},
			Anchor:   m.Anchor,
			Lines:    m.Lines,
			Overflow: m.Overflow,
			Wrapped:  m.Wrapped,
			Y:        t.cfg.Y, Size: t.cfg.Size, MaxWidth: t.cfg.MaxWidth,
		})
	}
	return out
}

// SolveRequest asks for the height, y and x that fit a device's body between a
// given top edge and a given bleed past the bottom.
//
// Unlike centring and bleeding, this one cannot be done by the page: height
// does not move the body one-for-one — it sizes the crop box, and the body is
// a fraction of that — so it takes the model to invert.
type SolveRequest struct {
	// Screen is the screen owning the device, which in a seam view may be the
	// neighbour rather than the one selected in the picker.
	Screen string  `json:"screen"`
	Output string  `json:"output"`
	Device int     `json:"device"`
	Top    float64 `json:"top"`
	Bleed  float64 `json:"bleed"`
	Edits  *Edits  `json:"edits"`
}

// SolveResponse is the solved triple.
type SolveResponse struct {
	Height int     `json:"height"`
	Y      float64 `json:"y"`
	X      float64 `json:"x"`
}

// Solve answers a fit request.
func (s *Server) Solve(req SolveRequest) (*SolveResponse, error) {
	cfg := s.project.Config()
	output, device, err := s.resolveOutput(req.Output, req.Screen)
	if err != nil {
		return nil, err
	}
	base := cfg.GetScreen(req.Screen)
	if base == nil {
		return nil, fmt.Errorf("unknown screen: %s", req.Screen)
	}

	screen := req.Edits.Apply(base)
	devices := DevicesOf(screen)
	if req.Device < 0 || req.Device >= len(devices) {
		return nil, fmt.Errorf("screen %q has no device %d", req.Screen, req.Device)
	}
	d := devices[req.Device]

	deviceName := device.ScreenshotPrefix
	if deviceName == "" {
		deviceName = output.Device
	}
	f, err := s.cache.frame(d, cfg.Settings.ScreenshotsDir, deviceName)
	if err != nil {
		return nil, err
	}

	canvas := image.Pt(device.Width, device.Height)
	solved := *d
	solved.Height, solved.Y = geometry.Fit(f, d, canvas, req.Top, req.Bleed)
	return &SolveResponse{
		Height: solved.Height,
		Y:      solved.Y,
		X:      geometry.CenterX(f, &solved, canvas),
	}, nil
}

// resolveOutput picks the store slot to render for. A named one when asked
// for, otherwise the first that includes the screen.
func (s *Server) resolveOutput(name, screenID string) (*config.Output, config.Device, error) {
	cfg := s.project.Config()
	for i := range cfg.Outputs {
		o := &cfg.Outputs[i]
		if name != "" && o.Device != name {
			continue
		}
		if name == "" && !contains(o.Screens, screenID) {
			continue
		}
		dev, ok := cfg.Devices[o.Device]
		if !ok {
			return nil, config.Device{}, fmt.Errorf("unknown device: %s", o.Device)
		}
		return o, dev, nil
	}
	return nil, config.Device{}, fmt.Errorf("no output renders screen %q", screenID)
}

func screenIndexOf(cfg *config.Config, id string) int {
	for i := range cfg.Screens {
		if cfg.Screens[i].ID == id {
			return i
		}
	}
	return -1
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
