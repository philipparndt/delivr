package editor

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A miniature of the real thing: the value being edited is buried in the prose
// that explains it, which is exactly the content a re-marshal would destroy.
const commented = `# ============================================================
# Templates. The game art is the selling point.
# ============================================================

iphone-front:
  title:
    size: 110

  device:
    auto_crop: true

    # x is 170 and NOT 0, which is the thing to know before touching it.
    # auto_crop trims to the frame's content, and that content is the phone
    # plus its drop shadow.
    x: 170        # measured, not guessed
    y: 438
    height: 2822
`

// nodeAt walks a resolved tree to a value node by key path.
func nodeAt(t *testing.T, root *yaml.Node, path ...string) *yaml.Node {
	t.Helper()
	n := root
	if n.Kind == yaml.DocumentNode {
		n = n.Content[0]
	}
	for _, key := range path {
		if n.Kind != yaml.MappingNode {
			t.Fatalf("%q: not a mapping", key)
		}
		found := false
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				n, found = n.Content[i+1], true
				break
			}
		}
		if !found {
			t.Fatalf("key %q not found", key)
		}
	}
	return n
}

func parse(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	return &doc
}

func TestSetScalarLeavesEverythingElseAlone(t *testing.T) {
	root := parse(t, commented)
	x := nodeAt(t, root, "iphone-front", "device", "x")

	got, err := SetScalar([]byte(commented), x.Line, x.Column, "200")
	if err != nil {
		t.Fatal(err)
	}

	// Byte-for-byte identical except for the one token.
	want := strings.Replace(commented, "x: 170  ", "x: 200  ", 1)
	if string(got) != want {
		t.Errorf("edit changed more than the value:\n--- got ---\n%s", got)
	}
}

func TestSetScalarKeepsTheCommentsThatExplainTheValue(t *testing.T) {
	root := parse(t, commented)
	x := nodeAt(t, root, "iphone-front", "device", "x")

	got, err := SetScalar([]byte(commented), x.Line, x.Column, "200")
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)

	for _, want := range []string{
		"# x is 170 and NOT 0, which is the thing to know before touching it.",
		"# plus its drop shadow.",
		"# measured, not guessed",
		"# ============================================================",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("comment lost: %q", want)
		}
	}
	// Blank lines are structure too — they are what separates the comment
	// blocks from each other.
	if strings.Count(out, "\n\n") != strings.Count(commented, "\n\n") {
		t.Error("blank line structure changed")
	}
}

// The comparison that makes the case for a text edit over a re-marshal.
func TestReMarshallingWouldHaveDestroyedTheFile(t *testing.T) {
	root := parse(t, commented)
	out, err := yaml.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(out), "\n\n") == strings.Count(commented, "\n\n") {
		t.Skip("yaml.v3 now preserves blank lines; the text edit is no longer " +
			"the only option, though it is still the smaller diff")
	}
}

func TestSetScalarRoundTripsThroughTheParser(t *testing.T) {
	root := parse(t, commented)
	h := nodeAt(t, root, "iphone-front", "device", "height")

	got, err := SetScalar([]byte(commented), h.Line, h.Column, "2400")
	if err != nil {
		t.Fatal(err)
	}
	if v := nodeAt(t, parse(t, string(got)), "iphone-front", "device", "height").Value; v != "2400" {
		t.Errorf("re-parsed height = %q, want 2400", v)
	}
}

func TestSetScalarPreservesQuoting(t *testing.T) {
	src := "a: 'single'\nb: \"double\"\nc: plain\n"
	root := parse(t, src)

	for _, tc := range []struct{ key, value, want string }{
		{"a", "new", "a: 'new'"},
		{"b", "new", `b: "new"`},
		{"c", "new", "c: new"},
		// A value that would change meaning unquoted gets quoted.
		{"c", "true", `c: "true"`},
		{"c", "12: 30", `c: "12: 30"`},
		{"c", "", `c: ""`},
		// An apostrophe inside a single-quoted scalar is doubled.
		{"a", "it's", "a: 'it''s'"},
	} {
		n := nodeAt(t, root, tc.key)
		got, err := SetScalar([]byte(src), n.Line, n.Column, tc.value)
		if err != nil {
			t.Fatalf("%s=%q: %v", tc.key, tc.value, err)
		}
		if !strings.Contains(string(got), tc.want) {
			t.Errorf("%s=%q produced:\n%s\nwant a line %q", tc.key, tc.value, got, tc.want)
		}
		// Whatever the quoting, it must parse back to the value asked for.
		if v := nodeAt(t, parse(t, string(got)), tc.key).Value; v != tc.value {
			t.Errorf("%s=%q re-parsed as %q", tc.key, tc.value, v)
		}
	}
}

func TestSetScalarHandlesTextWithAHash(t *testing.T) {
	src := "title: Level #1 begins   # the real comment\n"
	root := parse(t, src)
	n := nodeAt(t, root, "title")

	got, err := SetScalar([]byte(src), n.Line, n.Column, "Level #2 begins")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "# the real comment") {
		t.Errorf("trailing comment lost: %s", got)
	}
	if v := nodeAt(t, parse(t, string(got)), "title").Value; v != "Level #2 begins" {
		t.Errorf("re-parsed as %q", v)
	}
}

func TestSetScalarRefusesBlockScalars(t *testing.T) {
	src := "text: |\n  one\n  two\n"
	root := parse(t, src)
	n := nodeAt(t, root, "text")
	if _, err := SetScalar([]byte(src), n.Line, n.Column, "x"); err == nil {
		t.Error("expected a refusal for a block scalar")
	}
}

func TestSetScalarRejectsAPositionOutsideTheFile(t *testing.T) {
	if _, err := SetScalar([]byte("a: 1\n"), 99, 4, "2"); err == nil {
		t.Error("expected an error for a line past the end")
	}
}

func TestInsertKeyAppendsAtTheMappingsIndentation(t *testing.T) {
	src := "screen:\n  device:\n    source: shot.png\n    height: 2400\n"
	root := parse(t, src)
	dev := nodeAt(t, root, "screen", "device")

	got, err := InsertKey([]byte(src), dev, "x", "170")
	if err != nil {
		t.Fatal(err)
	}
	want := "screen:\n  device:\n    source: shot.png\n    height: 2400\n    x: 170\n"
	if string(got) != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// A new key must not land inside the nested block that precedes it.
func TestInsertKeyGoesAfterANestedBlock(t *testing.T) {
	src := "device:\n  source: shot.png\n  crop:\n    threshold: 8\n    padding: 2\n"
	root := parse(t, src)
	dev := nodeAt(t, root, "device")

	got, err := InsertKey([]byte(src), dev, "x", "170")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(got), "    padding: 2\n  x: 170\n") {
		t.Errorf("key landed in the wrong place:\n%s", got)
	}
	if v := nodeAt(t, parse(t, string(got)), "device", "x").Value; v != "170" {
		t.Errorf("re-parsed x = %q", v)
	}
}

func TestInsertKeyRefusesWhenThereIsNoMapping(t *testing.T) {
	if _, err := InsertKey([]byte("a: 1\n"), nil, "x", "1"); err == nil {
		t.Error("expected a refusal when the parent mapping does not exist")
	}
	src := "a: {b: 1}\n"
	flow := nodeAt(t, parse(t, src), "a")
	if _, err := InsertKey([]byte(src), flow, "x", "1"); err == nil {
		t.Error("expected a refusal for a flow mapping")
	}
}

func TestEditsSurviveCRLF(t *testing.T) {
	src := "a: 1\r\nb: 2\r\n"
	n := nodeAt(t, parse(t, src), "a")
	got, err := SetScalar([]byte(src), n.Line, n.Column, "9")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "a: 9\r\nb: 2\r\n" {
		t.Errorf("line endings changed: %q", got)
	}
}
