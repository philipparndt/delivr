package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A miniature project with the structure that makes write-back ambiguous: a
// template shared by two screens, one of which also overrides the value, and
// the comments that make a naive re-marshal unacceptable.
func fixture(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()

	write(t, dir, "templates.yaml", `# Reusable geometry.
iphone-front:
  title:
    font: "SF.otf"
    size: 110
    color: "#FFFFFF"
    align: "center"
    y: 35
  device:
    auto_crop: true

    # x is 170 and NOT 0, which is the thing to know before touching it.
    # auto_crop trims to the frame's content, and that content is the phone
    # plus its drop shadow.
    x: 170
    y: 438
    height: 2822
`)

	write(t, dir, "screens.yaml", `# The screens.
- id: "one"
  template: "iphone-front"
  background:
    type: "solid"
    color: "#000000"
  device:
    source: "one.png"
    # A delta on the template, not a replacement.
    x: 30

- id: "two"
  template: "iphone-front"
  background:
    type: "solid"
    color: "#000000"
  subtitle:
    text: "Second screen"
  device:
    source: "two.png"
    height: 2400

# A screen with a devices: LIST. mergeDevicesArray returns this verbatim — the
# template's device block is not merged into it and not added to it — so these
# values are literal, unlike screen "one" where they are deltas.
- id: "pair"
  template: "iphone-front"
  background:
    type: "solid"
    color: "#000000"
  devices:
    - source: "back.png"
      x: 153
      y: 415
    - source: "front.png"
      x: -1025
      y: 298
`)

	write(t, dir, "delivr.yaml", `settings:
  fonts_dir: "."
  screenshots_dir: "."

devices:
  iphone:
    name: "iPhone"
    width: 1320
    height: 2868

generate:
  languages: ["de-DE"]
  translations:
    de-DE:
      two:
        subtitle: "Zweiter Bildschirm"
  templates: !include templates.yaml
  screens: !include screens.yaml
  outputs:
    - device: "iphone"
      screens: ["one", "two", "pair"]
      prefix: "iphone"
`)

	srv, err := New(filepath.Join(dir, "delivr.yaml"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv, dir
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The x on screen "one" is 200 on screen, and that number appears in neither
// file: it is the template's 170 plus the screen's 30.
func TestResolvedValueIsTheSumOfTemplateAndScreen(t *testing.T) {
	srv, _ := fixture(t)
	screen := srv.project.Config().GetScreen("one")
	if got := DevicesOf(screen)[0].X; got != 200 {
		t.Fatalf("resolved x = %v, want 200 (170 from the template plus 30)", got)
	}
}

func TestTargetsSplitTheValueBetweenTemplateAndScreen(t *testing.T) {
	srv, _ := fixture(t)

	targets, err := srv.Targets(TargetsRequest{Screen: "one", Device: 0, Field: "x", Value: 250})
	if err != nil {
		t.Fatal(err)
	}
	byKind := map[string]Target{}
	for _, tg := range targets {
		byKind[tg.Kind] = tg
	}

	// Writing 250 into either file would be wrong: the other one still gets
	// added on top.
	if got := byKind["template"].Write; got != 220 {
		t.Errorf("template write = %v, want 220 (250 less the screen's 30)", got)
	}
	if got := byKind["screen"].Write; got != 80 {
		t.Errorf("screen write = %v, want 80 (250 less the template's 170)", got)
	}
	if !strings.Contains(byKind["screen"].Note, "170") {
		t.Errorf("the screen target should say what it is a delta on: %q", byKind["screen"].Note)
	}
}

func TestTemplateTargetSaysHowManyScreensItMoves(t *testing.T) {
	srv, _ := fixture(t)
	targets, err := srv.Targets(TargetsRequest{Screen: "one", Device: 0, Field: "y", Value: 500})
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range targets {
		if tg.Kind != "template" {
			continue
		}
		if len(tg.Affects) != 3 {
			t.Errorf("template affects %v, want every screen using it", tg.Affects)
		}
		if !strings.Contains(tg.Note, "3 screens") {
			t.Errorf("note should warn about the blast radius, got %q", tg.Note)
		}
		return
	}
	t.Fatal("no template target offered")
}

// height replaces rather than adds, so a screen that sets it makes the template
// unwritable — writing there would silently do nothing.
func TestTemplateTargetIsBlockedWhenTheScreenOverridesIt(t *testing.T) {
	srv, _ := fixture(t)
	targets, err := srv.Targets(TargetsRequest{Screen: "two", Device: 0, Field: "height", Value: 2600})
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range targets {
		if tg.Kind == "template" && tg.Blocked == "" {
			t.Error("the template target should be blocked: the screen sets height itself")
		}
	}
}

func TestApplyWritesToTheTemplateAndKeepsItsComments(t *testing.T) {
	srv, dir := fixture(t)
	templatesBefore := read(t, dir, "templates.yaml")

	res, err := srv.Apply(ApplyRequest{
		Screen: "one", Device: 0, Field: "x", Value: 250, Kind: "template",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Written != 220 {
		t.Errorf("wrote %v, want 220", res.Written)
	}

	out := read(t, dir, "templates.yaml")
	if !strings.Contains(out, "    x: 220\n") {
		t.Errorf("value not written:\n%s", out)
	}
	for _, want := range []string{
		"# x is 170 and NOT 0, which is the thing to know before touching it.",
		"# plus its drop shadow.",
		"# Reusable geometry.",
		`    font: "SF.otf"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q from the file:\n%s", want, out)
		}
	}
	// Everything except the one number is byte-identical: putting 170 back
	// reproduces the original file exactly.
	if restored := strings.Replace(out, "x: 220", "x: 170", 1); restored != templatesBefore {
		t.Errorf("the write touched more than the value:\n--- after ---\n%s", out)
	}
	// And the config now resolves to what was asked for.
	if got := DevicesOf(srv.project.Config().GetScreen("one"))[0].X; got != 250 {
		t.Errorf("after the write the screen resolves to x = %v, want 250", got)
	}
}

func TestApplyWritesToTheScreenAsADelta(t *testing.T) {
	srv, dir := fixture(t)

	if _, err := srv.Apply(ApplyRequest{
		Screen: "one", Device: 0, Field: "x", Value: 250, Kind: "screen",
	}); err != nil {
		t.Fatal(err)
	}

	if out := read(t, dir, "screens.yaml"); !strings.Contains(out, "    x: 80\n") {
		t.Errorf("expected the delta 80 in screens.yaml:\n%s", out)
	}
	if out := read(t, dir, "templates.yaml"); !strings.Contains(out, "    x: 170\n") {
		t.Error("the template should not have been touched")
	}
	if got := DevicesOf(srv.project.Config().GetScreen("one"))[0].X; got != 250 {
		t.Errorf("resolves to x = %v, want 250", got)
	}
}

// The other screen has no x of its own, so writing one means adding a key.
func TestApplyAddsAKeyThatDoesNotExistYet(t *testing.T) {
	srv, dir := fixture(t)

	res, err := srv.Apply(ApplyRequest{
		Screen: "two", Device: 0, Field: "x", Value: 200, Kind: "screen",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Added {
		t.Error("expected the key to be reported as added")
	}
	out := read(t, dir, "screens.yaml")
	if !strings.Contains(out, "    x: 30\n") {
		t.Error("screen one's x should be untouched")
	}
	// 200 on screen less the template's 170, added after the block's last key
	// and at its indentation.
	if !strings.Contains(out, "    height: 2400\n    x: 30\n") {
		t.Errorf("the added key is not where it should be:\n%s", out)
	}
	if got := DevicesOf(srv.project.Config().GetScreen("two"))[0].X; got != 200 {
		t.Errorf("resolves to x = %v, want 200", got)
	}
}

func TestApplyRefusesInReadOnlyMode(t *testing.T) {
	srv, dir := fixture(t)
	srv.readOnly = true

	before := read(t, dir, "templates.yaml")
	if _, err := srv.Apply(ApplyRequest{
		Screen: "one", Device: 0, Field: "x", Value: 250, Kind: "template",
	}); err == nil {
		t.Fatal("expected a refusal")
	}
	if read(t, dir, "templates.yaml") != before {
		t.Error("read-only mode wrote to the file anyway")
	}
}

func TestOriginsPointAtTheFileTheValueLivesIn(t *testing.T) {
	srv, _ := fixture(t)
	screen := srv.project.Config().GetScreen("one")

	origins := srv.originsFor(screen, 0, 0, "device", "x", "y", "height")
	// x is set on the screen, so that is what is reported — it is what was
	// added last, and what the user is most likely to want to change.
	if origins["x"].Kind != "screen" || origins["x"].File != "screens.yaml" {
		t.Errorf("x origin = %+v, want the screen's own value", origins["x"])
	}
	if !origins["x"].Additive {
		t.Error("x should be flagged as additive")
	}
	// y and height come from the template, in the other file.
	for _, field := range []string{"y", "height"} {
		o := origins[field]
		if o == nil || o.Kind != "template" || o.File != "templates.yaml" {
			t.Errorf("%s origin = %+v, want the template", field, o)
		}
	}
}

// Saving several fields at once applies them one at a time, each resolved
// against the file as the previous write left it. This is the sequence the
// Save button runs, and it has to land on the asked-for values even though
// x and y are deltas and the writes go to different files.
func TestSavingSeveralFieldsInSequence(t *testing.T) {
	srv, dir := fixture(t)

	batch := []ApplyRequest{
		{Screen: "one", Device: 0, Field: "x", Value: 250, Kind: "template"},
		{Screen: "one", Device: 0, Field: "y", Value: 500, Kind: "screen"},
		{Screen: "one", Device: 0, Field: "height", Value: 2600, Kind: "template"},
	}
	for _, req := range batch {
		if _, err := srv.Apply(req); err != nil {
			t.Fatalf("%s: %v", req.Field, err)
		}
	}

	d := DevicesOf(srv.project.Config().GetScreen("one"))[0]
	if d.X != 250 || d.Y != 500 || d.Height != 2600 {
		t.Errorf("after saving the batch the screen resolves to x=%v y=%v height=%v, "+
			"want 250/500/2600", d.X, d.Y, d.Height)
	}

	// The comments are still there after three separate writes to two files.
	if out := read(t, dir, "templates.yaml"); !strings.Contains(out,
		"# x is 170 and NOT 0, which is the thing to know before touching it.") {
		t.Errorf("comments lost across a batch:\n%s", out)
	}
	if out := read(t, dir, "screens.yaml"); !strings.Contains(out,
		"# A delta on the template, not a replacement.") {
		t.Errorf("comments lost in screens.yaml:\n%s", out)
	}
}

// Writing the same field twice — nudge, save, nudge again, save — has to be
// idempotent rather than compounding, since the second write is computed
// against a config that already contains the first.
func TestSavingTheSameFieldTwiceDoesNotCompound(t *testing.T) {
	srv, _ := fixture(t)

	for range 2 {
		if _, err := srv.Apply(ApplyRequest{
			Screen: "one", Device: 0, Field: "x", Value: 250, Kind: "template",
		}); err != nil {
			t.Fatal(err)
		}
		if got := DevicesOf(srv.project.Config().GetScreen("one"))[0].X; got != 250 {
			t.Fatalf("resolved x = %v, want 250", got)
		}
	}
}

func TestApplyRejectsAnUnknownTarget(t *testing.T) {
	srv, _ := fixture(t)
	if _, err := srv.Apply(ApplyRequest{
		Screen: "one", Device: 0, Field: "x", Value: 1, Kind: "nonsense",
	}); err == nil {
		t.Error("expected an error for an unknown target kind")
	}
}

func TestEditsYAMLShowsOnlyWhatChanged(t *testing.T) {
	srv, _ := fixture(t)
	screen := srv.project.Config().GetScreen("one")

	x, h := 250.0, 2400
	e := &Edits{Devices: map[int]*DeviceEdit{0: {X: &x, Height: &h}}}
	out := e.YAML(screen, "one")

	for _, want := range []string{"# one", "device:", "height: 2400", "x: 250"} {
		if !strings.Contains(out, want) {
			t.Errorf("YAML missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "y:") {
		t.Errorf("y was not changed and should not appear:\n%s", out)
	}
}

// Copy is editable too, and its values live in the same template/screen split
// as the geometry — with one extra home for a localised string.
func TestTextTargetsFollowTheSameSplit(t *testing.T) {
	srv, _ := fixture(t)

	// The title's size comes from the template, and no screen overrides it.
	targets, err := srv.Targets(TargetsRequest{Screen: "one", Field: "title.size", Value: 96})
	if err != nil {
		t.Fatal(err)
	}
	var tmpl *Target
	for i := range targets {
		if targets[i].Kind == "template" {
			tmpl = &targets[i]
		}
	}
	if tmpl == nil || tmpl.Blocked != "" {
		t.Fatalf("expected a writable template target, got %+v", targets)
	}
	// Text fields replace rather than add, so no delta arithmetic.
	if tmpl.Write != 96 {
		t.Errorf("template write = %v, want 96 — text fields are not additive", tmpl.Write)
	}
}

func TestApplyWritesATextSizeToTheTemplate(t *testing.T) {
	srv, dir := fixture(t)

	if _, err := srv.Apply(ApplyRequest{
		Screen: "one", Field: "title.size", Value: 96, Kind: "template",
	}); err != nil {
		t.Fatal(err)
	}
	out := read(t, dir, "templates.yaml")
	if !strings.Contains(out, "    size: 96\n") {
		t.Errorf("size not written:\n%s", out)
	}
	if !strings.Contains(out, `    font: "SF.otf"`) {
		t.Error("the rest of the title block should be untouched")
	}
	if got := srv.project.Config().GetScreen("one").Title.Size; got != 96 {
		t.Errorf("resolved size = %v, want 96", got)
	}
}

// Localised copy belongs in the translation file. Writing it to the screen
// would change the base string, and writing it to the template would change
// every language at once — so both are refused while a language is selected.
func TestLocalisedCopyGoesToTheTranslationFile(t *testing.T) {
	srv, dir := fixture(t)

	targets, err := srv.Targets(TargetsRequest{
		Screen: "two", Field: "subtitle.text", Text: "Zweites Bild", Lang: "de-DE",
	})
	if err != nil {
		t.Fatal(err)
	}
	byKind := map[string]Target{}
	for _, tg := range targets {
		byKind[tg.Kind] = tg
	}
	if _, ok := byKind["translation"]; !ok {
		t.Fatalf("no translation target offered: %+v", targets)
	}
	if byKind["translation"].Blocked != "" {
		t.Errorf("translation target blocked: %s", byKind["translation"].Blocked)
	}
	if byKind["screen"].Blocked == "" {
		t.Error("writing German copy to the screen should be refused")
	}

	if _, err := srv.Apply(ApplyRequest{
		Screen: "two", Field: "subtitle.text",
		Text: "Zweites Bild", Lang: "de-DE", Kind: "translation",
	}); err != nil {
		t.Fatal(err)
	}
	if out := read(t, dir, "delivr.yaml"); !strings.Contains(out, `subtitle: "Zweites Bild"`) {
		t.Errorf("translation not written:\n%s", out)
	}
	// The base-language string is untouched.
	if out := read(t, dir, "screens.yaml"); !strings.Contains(out, `text: "Second screen"`) {
		t.Error("the base copy should not have changed")
	}
}

// Adding a device is a pending edit that renders, but is not written back.
func TestAddedDeviceRendersAndIsEmittedAsYAML(t *testing.T) {
	srv, _ := fixture(t)
	screen := srv.project.Config().GetScreen("one")

	e := &Edits{Added: []*NewDevice{{
		Source: "two.png", AutoCrop: true, CropThreshold: 8, Height: 2400, X: -1025, Y: 300,
	}}}
	applied := e.Apply(screen)

	devices := DevicesOf(applied)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
	// The existing device keeps index 0, so nothing already addressed moves.
	if devices[0].Source != "one.png" {
		t.Errorf("device 0 = %q, want the original", devices[0].Source)
	}
	if devices[1].Source != "two.png" || devices[1].X != -1025 {
		t.Errorf("added device = %+v", devices[1])
	}
	// And the original config is untouched.
	if len(DevicesOf(srv.project.Config().GetScreen("one"))) != 1 {
		t.Error("Apply mutated the loaded config")
	}

	out := e.YAML(applied, "one")
	for _, want := range []string{"devices:", "source: \"two.png\"", "x: -1025", "auto_crop: true"} {
		if !strings.Contains(out, want) {
			t.Errorf("YAML missing %q:\n%s", want, out)
		}
	}
}

// The regression that moved a device on save.
//
// `x` and `y` are additive only on the single-`device:` path, where
// mergeDeviceImage runs. A screen with a `devices:` list never reaches it:
// mergeDevicesArray returns the list untouched, so the value on screen IS the
// value in the file. Subtracting the template's anyway writes a number short by
// exactly the template's y, and the device jumps up by that much.
func TestDevicesListValuesAreLiteralNotDeltas(t *testing.T) {
	srv, _ := fixture(t)

	// The template says y: 438; the list says 415. The list wins outright.
	if got := DevicesOf(srv.project.Config().GetScreen("pair"))[0].Y; got != 415 {
		t.Fatalf("resolved y = %v, want the list's 415 with no template merge", got)
	}

	targets, err := srv.Targets(TargetsRequest{Screen: "pair", Device: 0, Field: "y", Value: 500})
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range targets {
		switch tg.Kind {
		case "screen":
			if tg.Blocked != "" {
				t.Fatalf("the screen's own list must be writable: %s", tg.Blocked)
			}
			if tg.Write != 500 {
				t.Errorf("screen write = %v, want 500 — a devices: entry is literal, "+
					"not a delta on the template's 438", tg.Write)
			}
		case "template":
			if tg.Blocked == "" {
				t.Error("writing to the template must be refused: a devices: list " +
					"ignores it entirely, so the write would silently do nothing")
			}
		}
	}
}

func TestSavingToADevicesListLandsOnTheAskedForValue(t *testing.T) {
	srv, dir := fixture(t)

	for _, tc := range []struct {
		field string
		value float64
	}{{"y", 712}, {"x", -854}} {
		if _, err := srv.Apply(ApplyRequest{
			Screen: "pair", Device: 1, Field: tc.field, Value: tc.value, Kind: "screen",
		}); err != nil {
			t.Fatalf("%s: %v", tc.field, err)
		}
	}

	d := DevicesOf(srv.project.Config().GetScreen("pair"))[1]
	if d.X != -854 || d.Y != 712 {
		t.Errorf("after saving, the device renders at x=%v y=%v, want -854/712 — "+
			"it moved when it should have stayed put", d.X, d.Y)
	}
	// The other entry in the list is untouched.
	if other := DevicesOf(srv.project.Config().GetScreen("pair"))[0]; other.X != 153 || other.Y != 415 {
		t.Errorf("the sibling entry moved: %+v", other)
	}
	if out := read(t, dir, "screens.yaml"); !strings.Contains(out, "      y: 712\n") {
		t.Errorf("expected the literal value in the list:\n%s", out)
	}
}

// Saving must be idempotent for every device form: what is on screen before the
// save is what is on screen after it.
func TestSavingNeverMovesAnything(t *testing.T) {
	for _, tc := range []struct {
		screen string
		device int
		kind   string
	}{
		{"one", 0, "screen"}, {"one", 0, "template"},
		{"pair", 0, "screen"}, {"pair", 1, "screen"},
	} {
		t.Run(tc.screen+"/"+tc.kind, func(t *testing.T) {
			srv, _ := fixture(t)
			before := *DevicesOf(srv.project.Config().GetScreen(tc.screen))[tc.device]

			// Save the values exactly as they already are.
			for _, f := range []struct {
				field string
				value float64
			}{{"x", before.X}, {"y", before.Y}} {
				if _, err := srv.Apply(ApplyRequest{
					Screen: tc.screen, Device: tc.device,
					Field: f.field, Value: f.value, Kind: tc.kind,
				}); err != nil {
					t.Fatalf("%s: %v", f.field, err)
				}
			}

			after := *DevicesOf(srv.project.Config().GetScreen(tc.screen))[tc.device]
			if after.X != before.X || after.Y != before.Y {
				t.Errorf("saving unchanged values moved the device: %v,%v -> %v,%v",
					before.X, before.Y, after.X, after.Y)
			}
		})
	}
}

// Reordering the store row is the one structural edit the editor makes. The
// list it rewrites is usually annotated — per-entry notes explaining why a
// particular screenshot leads — so each entry has to take its comments with it.
func TestReorderKeepsEachEntrysComments(t *testing.T) {
	dir := t.TempDir()
	src := `# iPhone — the full arc of the store video, in order.
- device: "iphone"
  screens:
    # Leads, because it carries the wordmark.
    - "one"
    # The mechanic that names the game.
    - "two"
    - "pair"   # the overlapping pair
  prefix: "iphone"
`
	write(t, dir, "outputs.yaml", src)

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	seq := nodeAt(t, doc.Content[0].Content[0], "screens")

	// Move the last entry to the front: [pair, one, two].
	out, err := ReorderSequence([]byte(src), seq, []int{2, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	want := `# iPhone — the full arc of the store video, in order.
- device: "iphone"
  screens:
    - "pair"   # the overlapping pair
    # Leads, because it carries the wordmark.
    - "one"
    # The mechanic that names the game.
    - "two"
  prefix: "iphone"
`
	if got != want {
		t.Errorf("reorder produced:\n%s\nwant:\n%s", got, want)
	}
}

// The compact form is common and must work too.
func TestReorderHandlesAFlowSequence(t *testing.T) {
	src := "outputs:\n  - device: \"iphone\"\n    screens: [\"one\", \"two\", \"pair\"]\n"
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	seq := nodeAt(t, doc.Content[0], "outputs").Content[0]
	seq = nodeAt(t, seq, "screens")

	out, err := ReorderSequence([]byte(src), seq, []int{2, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	want := "outputs:\n  - device: \"iphone\"\n    screens: [\"pair\", \"one\", \"two\"]\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestReorderRefusesANonPermutation(t *testing.T) {
	src := "- a\n- b\n- c\n"
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	seq := doc.Content[0]

	for name, order := range map[string][]int{
		"duplicate":    {0, 0, 1},
		"out of range": {0, 1, 9},
		"too short":    {0, 1},
	} {
		if _, err := ReorderSequence([]byte(src), seq, order); err == nil {
			t.Errorf("%s: expected a refusal", name)
		}
	}
}

// The whole point is that the row's order is what the store shows.
func TestReorderChangesWhatTheOutputRenders(t *testing.T) {
	srv, dir := fixture(t)

	before := srv.project.Config().Outputs[0].Screens
	if before[0] != "one" {
		t.Fatalf("fixture order changed: %v", before)
	}

	if _, err := srv.Reorder(ReorderRequest{
		Output: "iphone", Screens: []string{"pair", "two", "one"},
	}); err != nil {
		t.Fatal(err)
	}

	after := srv.project.Config().Outputs[0].Screens
	if strings.Join(after, ",") != "pair,two,one" {
		t.Errorf("output order = %v, want pair,two,one", after)
	}
	// The rest of the file is untouched.
	if out := read(t, dir, "delivr.yaml"); !strings.Contains(out, `prefix: "iphone"`) {
		t.Error("the output block lost its prefix")
	}
}

func TestReorderRefusedInReadOnlyMode(t *testing.T) {
	srv, _ := fixture(t)
	srv.readOnly = true
	if _, err := srv.Reorder(ReorderRequest{
		Output: "iphone", Screens: []string{"two", "one", "pair"},
	}); err == nil {
		t.Error("expected a refusal")
	}
}
