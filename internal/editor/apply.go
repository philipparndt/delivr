package editor

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/philipparndt/delivr/internal/config"
)

// A dragged value has more than one plausible home, and the editor cannot pick
// between them on the user's behalf.
//
// Screen iphone-03-blackhole takes its device geometry from the iphone-front
// template, which four other screens also use. Writing the new x there is
// sometimes exactly right — the whole point of a template is that its screens
// move together — and sometimes it silently moves five screenshots when one was
// meant. So both targets are offered, the template one labelled with how many
// screens it moves, and nothing is written until one is chosen.
//
// The second trap is that mergeDeviceImage adds x and y rather than replacing
// them: a screen-level `x: 30` on a template with `x: 170` renders at 200. A
// value written to the wrong one of those two places is not just in the wrong
// file, it is the wrong number. Target computes the delta.

// Origin is where a resolved value came from, for display.
type Origin struct {
	Kind     string  `json:"kind"` // "template" or "screen"
	Template string  `json:"template,omitempty"`
	File     string  `json:"file"`
	Line     int     `json:"line"`
	Value    float64 `json:"value"`
	Text     string  `json:"text,omitempty"`
	Additive bool    `json:"additive"`
}

// Target is one place a value could be written.
type Target struct {
	Kind     string `json:"kind"`
	Template string `json:"template,omitempty"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	// Write is the value that goes in the file, which for an additive field
	// is not the value on screen. Text carries it instead for prose fields.
	Write float64 `json:"write"`
	Text  string  `json:"text,omitempty"`
	// Affects lists the screens this write moves.
	Affects []string `json:"affects"`
	// Note explains anything surprising about the write.
	Note string `json:"note,omitempty"`
	// Insert is set when the key does not exist yet and would be added.
	Insert bool `json:"insert"`
	// Blocked explains why this target cannot be written to, if it cannot.
	Blocked string `json:"blocked,omitempty"`
}

// additiveCandidates are the fields mergeDeviceImage adds rather than replaces.
// Every other merge in mergeDeviceImage and mergeTextConfig replaces.
//
// "Candidates" because the addition only happens on the single-`device:` path.
// A screen with a `devices:` list never reaches mergeDeviceImage at all, so its
// values are literal — see Project.DeviceForm.
var additiveCandidates = map[string]bool{"device.x": true, "device.y": true}

// mergeRules says, for one block on one screen, whether a screen-level value is
// a delta and which of the two files writing to would actually change anything.
type mergeRules struct {
	additive          bool
	templateEffective bool
	screenEffective   bool
	why               string
}

func (s *Server) mergeRules(screen *config.Screen, screenIndex int, block, key string) mergeRules {
	if block != "device" {
		// Title and subtitle always go through mergeTextConfig, which replaces.
		return mergeRules{templateEffective: true, screenEffective: true}
	}

	switch s.project.DeviceForm(screenIndex, screen.Template) {
	case FormScreenArray:
		// mergeDevicesArray returns the screen's list untouched: the template's
		// device block is not merged, not added, not read.
		return mergeRules{
			screenEffective: true,
			why: "this screen has a devices: list, which replaces the template's " +
				"device block outright rather than merging with it",
		}
	case FormTemplateArray:
		// The template's list wins and the screen's own device: is ignored.
		return mergeRules{
			templateEffective: true,
			why:               "the template has a devices: list, which this screen's device: cannot override",
		}
	default:
		return mergeRules{
			additive:          additiveCandidates[block+"."+key],
			templateEffective: true,
			screenEffective:   true,
		}
	}
}

// editPath splits a field name into the block it lives in and the key inside
// that block. A bare name is a device field, since that is most of them.
//
//	"x"          -> "device",   "x"
//	"title.y"    -> "title",    "y"
//	"subtitle.max_width" -> "subtitle", "max_width"
func editPath(field string) (block, key string) {
	if i := strings.IndexByte(field, '.'); i >= 0 {
		return field[:i], field[i+1:]
	}
	return "device", field
}

// blockNodes returns the template-level and screen-level mappings for a block,
// with the files they live in.
func (s *Server) blockNodes(screen *config.Screen, screenIndex, deviceIndex int, block string) (
	tmplNode *yaml.Node, tmplFile string, screenNode *yaml.Node, screenFile string) {

	if block == "device" {
		if screen.Template != "" {
			tmplNode, tmplFile = s.project.TemplateDeviceNode(screen.Template, deviceIndex)
		}
		screenNode, screenFile = s.project.ScreenDeviceNode(screenIndex, deviceIndex)
		return
	}

	if screen.Template != "" {
		tmplNode, tmplFile = s.project.TemplateTextNode(screen.Template, block)
	}
	screenNode, screenFile = s.project.ScreenTextNode(screenIndex, block)
	return
}

// isTextValue reports whether a field holds prose rather than a number, which
// decides how it is written and whether a translation is a possible home.
func isTextValue(block, key string) bool {
	return block != "device" && key == "text"
}

// originsFor reports where each of a block's values resolved from.
func (s *Server) originsFor(screen *config.Screen, screenIndex, deviceIndex int,
	block string, keys ...string) map[string]*Origin {

	out := make(map[string]*Origin)
	for _, key := range keys {
		if o := s.originOf(screen, screenIndex, deviceIndex, block, key); o != nil {
			out[key] = o
		}
	}
	return out
}

// originOf prefers the screen's own value, since that is what wins (or, for an
// additive field, what was added last).
func (s *Server) originOf(screen *config.Screen, screenIndex, deviceIndex int,
	block, key string) *Origin {

	tmplNode, tmplFile, screenNode, screenFile :=
		s.blockNodes(screen, screenIndex, deviceIndex, block)
	rules := s.mergeRules(screen, screenIndex, block, key)
	additive := rules.additive
	if !rules.templateEffective {
		tmplNode = nil
	}
	if !rules.screenEffective {
		screenNode = nil
	}

	if v := mapValue(screenNode, key); v != nil {
		return &Origin{
			Kind: "screen", File: s.project.Rel(screenFile), Line: v.Line,
			Value: parseNum(v.Value), Text: v.Value, Additive: additive,
		}
	}
	if v := mapValue(tmplNode, key); v != nil {
		return &Origin{
			Kind: "template", Template: screen.Template,
			File: s.project.Rel(tmplFile), Line: v.Line,
			Value: parseNum(v.Value), Text: v.Value, Additive: additive,
		}
	}
	return nil
}

// TargetsRequest asks where a value could be written.
type TargetsRequest struct {
	Screen string `json:"screen"`
	Device int    `json:"device"`
	Field  string `json:"field"`
	// Value is the number to write; Text is used instead for prose fields.
	Value float64 `json:"value"`
	Text  string  `json:"text"`
	// Lang, when set, makes the translation file a possible home for copy.
	Lang string `json:"lang"`
}

// writeText renders the value a target will write, for display and for the
// actual edit.
func (t TargetsRequest) writeText(block, key string, write float64) string {
	if isTextValue(block, key) {
		return t.Text
	}
	if key == "height" || key == "width" {
		return strconv.FormatInt(int64(write), 10)
	}
	return num(write)
}

// Targets lists the places a value could go, with the consequences of each.
func (s *Server) Targets(req TargetsRequest) ([]Target, error) {
	cfg := s.project.Config()
	screen := cfg.GetScreen(req.Screen)
	if screen == nil {
		return nil, fmt.Errorf("unknown screen: %s", req.Screen)
	}
	idx := screenIndexOf(cfg, req.Screen)
	block, key := editPath(req.Field)

	tmplNode, tmplFile, screenNode, screenFile := s.blockNodes(screen, idx, req.Device, block)
	rules := s.mergeRules(screen, idx, block, key)
	tmplValue := valueOf(tmplNode, key)
	screenValue := valueOf(screenNode, key)
	additive := rules.additive
	if !rules.templateEffective {
		tmplValue = 0
	}
	if !rules.screenEffective {
		screenValue = 0
	}

	var targets []Target

	// Localised copy belongs in the translation file, not in the screen: the
	// screen holds the base-language string, and writing a German headline
	// there would change every language at once.
	if req.Lang != "" && isTextValue(block, key) {
		t := Target{
			Kind: "translation", File: "", Text: req.Text,
			Affects: []string{req.Screen},
			Note:    fmt.Sprintf("%s copy for %s", block, req.Lang),
		}
		node, file := s.project.TranslationNode(req.Lang, req.Screen)
		if node == nil {
			t.Blocked = fmt.Sprintf(
				"no translations entry for %s / %s yet — paste it instead", req.Lang, req.Screen)
		} else {
			t.File = s.project.Rel(file)
			if v := mapValue(node, block); v != nil {
				t.Line = v.Line
			} else {
				t.Insert = true
			}
		}
		targets = append(targets, t)
	}

	if tmplNode != nil {
		t := Target{
			Kind: "template", Template: screen.Template,
			File: s.project.Rel(tmplFile), Write: req.Value, Text: req.Text,
			Affects: s.screensUsing(screen.Template),
		}
		if additive {
			// The screen's own value still gets added on top, so the template
			// has to carry the remainder.
			t.Write = req.Value - screenValue
			if screenValue != 0 {
				t.Note = fmt.Sprintf("%s adds the screen's own %s, so the template takes %s",
					key, num(screenValue), num(t.Write))
			}
		} else if !rules.templateEffective {
			t.Blocked = rules.why
		} else if screenNode != nil && mapValue(screenNode, key) != nil {
			t.Blocked = fmt.Sprintf("the screen sets %s itself, which overrides the template", key)
		} else if req.Lang != "" && isTextValue(block, key) {
			t.Blocked = fmt.Sprintf(
				"this is the %s copy — writing it to the template would change every language", req.Lang)
		}
		if v := mapValue(tmplNode, key); v != nil {
			t.Line = v.Line
		} else {
			t.Insert = true
		}
		if len(t.Affects) > 1 && t.Blocked == "" {
			t.Note = joinNotes(t.Note, fmt.Sprintf("moves %d screens", len(t.Affects)))
		}
		targets = append(targets, t)
	}

	if screenNode != nil {
		t := Target{
			Kind: "screen", File: s.project.Rel(screenFile),
			Write: req.Value, Text: req.Text, Affects: []string{req.Screen},
		}
		if !rules.screenEffective {
			t.Blocked = rules.why
		} else if additive {
			t.Write = req.Value - tmplValue
			if tmplValue != 0 {
				t.Note = fmt.Sprintf("a delta on the template's %s of %s", key, num(tmplValue))
			}
		} else if req.Lang != "" && isTextValue(block, key) {
			t.Blocked = fmt.Sprintf(
				"this is the %s copy — the screen holds the base text", req.Lang)
		} else if !additive && t.Write == 0 && !isTextValue(block, key) && tmplNode != nil {
			// mergeTextConfig and mergeDeviceImage both treat a zero as "not
			// set", so a screen cannot override a template back down to zero.
			t.Blocked = fmt.Sprintf(
				"a screen-level %s of 0 is read as unset, so the template's %s would win",
				key, num(tmplValue))
		}
		if v := mapValue(screenNode, key); v != nil {
			t.Line = v.Line
		} else {
			t.Insert = true
		}
		targets = append(targets, t)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("screen %q has no %s block to write into — paste the YAML instead",
			req.Screen, block)
	}
	return targets, nil
}

// ApplyRequest writes one value to one chosen target.
type ApplyRequest struct {
	Screen string  `json:"screen"`
	Device int     `json:"device"`
	Field  string  `json:"field"`
	Value  float64 `json:"value"`
	Text   string  `json:"text"`
	Lang   string  `json:"lang"`
	Kind   string  `json:"kind"` // "template", "screen" or "translation"
}

// ApplyResult reports what was written.
type ApplyResult struct {
	File    string  `json:"file"`
	Field   string  `json:"field"`
	Written float64 `json:"written"`
	Line    int     `json:"line"`
	Added   bool    `json:"added"`
}

// Apply writes a value back to the file it came from, as a single-token edit.
func (s *Server) Apply(req ApplyRequest) (*ApplyResult, error) {
	if s.readOnly {
		return nil, fmt.Errorf("the editor was started with --read-only")
	}

	ask := TargetsRequest{
		Screen: req.Screen, Device: req.Device, Field: req.Field,
		Value: req.Value, Text: req.Text, Lang: req.Lang,
	}
	targets, err := s.Targets(ask)
	if err != nil {
		return nil, err
	}

	var chosen *Target
	for i := range targets {
		if targets[i].Kind == req.Kind {
			chosen = &targets[i]
			break
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("no %s target for %s", req.Kind, req.Field)
	}
	if chosen.Blocked != "" {
		return nil, fmt.Errorf("%s", chosen.Blocked)
	}

	cfg := s.project.Config()
	screen := cfg.GetScreen(req.Screen)
	idx := screenIndexOf(cfg, req.Screen)
	block, key := editPath(req.Field)

	var node *yaml.Node
	var file string
	switch req.Kind {
	case "template":
		node, file, _, _ = s.blockNodes(screen, idx, req.Device, block)
	case "screen":
		_, _, node, file = s.blockNodes(screen, idx, req.Device, block)
	case "translation":
		// The translation file keys copy by screen and then by block, with no
		// nesting below that: `de-DE: { screen-id: { title: "..." } }`.
		node, file = s.project.TranslationNode(req.Lang, req.Screen)
		key = block
	}
	if node == nil {
		return nil, fmt.Errorf("could not locate the %s %s block", req.Kind, block)
	}

	src, err := readFile(file)
	if err != nil {
		return nil, err
	}

	text := ask.writeText(block, key, chosen.Write)
	if req.Kind == "translation" {
		text = req.Text
	}

	var out []byte
	value := mapValue(node, key)
	if value != nil {
		out, err = SetScalar(src, value.Line, value.Column, text)
	} else {
		out, err = InsertKey(src, node, key, text)
	}
	if err != nil {
		return nil, err
	}

	if err := s.project.WriteFile(file, out); err != nil {
		return nil, err
	}
	s.cache.Reset()

	return &ApplyResult{
		File: s.project.Rel(file), Field: req.Field,
		Written: chosen.Write, Line: chosen.Line, Added: value == nil,
	}, nil
}

// screensUsing lists the screens a template moves.
func (s *Server) screensUsing(template string) []string {
	var out []string
	for _, sc := range s.project.Config().Screens {
		if sc.Template == template {
			out = append(out, sc.ID)
		}
	}
	return out
}

func valueOf(mapping *yaml.Node, field string) float64 {
	if v := mapValue(mapping, field); v != nil {
		return parseNum(v.Value)
	}
	return 0
}

func parseNum(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func joinNotes(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}
