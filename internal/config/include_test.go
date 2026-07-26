package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBasicInclude(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "devices.yaml", `
iphone:
  name: "iPhone 17 Pro Max"
  width: 1320
`)

	writeFile(t, dir, "main.yaml", `
settings:
  fonts_dir: /Library/Fonts
devices: !include devices.yaml
`)

	node, err := LoadNodeWithIncludes(filepath.Join(dir, "main.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	data, _ := yaml.Marshal(node)
	result := string(data)

	if !strings.Contains(result, "iPhone 17 Pro Max") {
		t.Errorf("expected included device name, got:\n%s", result)
	}
}

func TestNestedInclude(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)

	writeFile(t, sub, "font.yaml", `
title_font: SF-Pro-Bold.otf
`)

	writeFile(t, dir, "templates.yaml", `
default:
  fonts: !include sub/font.yaml
`)

	writeFile(t, dir, "main.yaml", `
templates: !include templates.yaml
`)

	node, err := LoadNodeWithIncludes(filepath.Join(dir, "main.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	data, _ := yaml.Marshal(node)
	result := string(data)

	if !strings.Contains(result, "SF-Pro-Bold.otf") {
		t.Errorf("expected nested included font, got:\n%s", result)
	}
}

func TestSequenceFlattening(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "iphone.yaml", `
- id: screen1
  title: "Screen 1"
- id: screen2
  title: "Screen 2"
`)

	writeFile(t, dir, "ipad.yaml", `
- id: screen3
  title: "Screen 3"
`)

	writeFile(t, dir, "main.yaml", `
screens:
  - !include iphone.yaml
  - !include ipad.yaml
`)

	node, err := LoadNodeWithIncludes(filepath.Join(dir, "main.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	data, _ := yaml.Marshal(node)
	result := string(data)

	// All 3 screens should be flattened into one list
	if strings.Count(result, "id:") != 3 {
		t.Errorf("expected 3 screens flattened, got:\n%s", result)
	}
}

func TestCycleDetection(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "a.yaml", `
ref: !include b.yaml
`)

	writeFile(t, dir, "b.yaml", `
ref: !include a.yaml
`)

	_, err := LoadNodeWithIncludes(filepath.Join(dir, "a.yaml"))
	if err == nil {
		t.Fatal("expected cycle detection error")
	}

	if !strings.Contains(err.Error(), "circular include") {
		t.Errorf("expected circular include error, got: %v", err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// Writing a value back means editing the file it actually lives in, which is
// not the file that included it.
func TestIncludeTracksTheSourceFile(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "templates.yaml", `
iphone-front:
  device:
    x: 170
`)
	writeFile(t, dir, "main.yaml", `
settings:
  fonts_dir: /Library/Fonts
templates: !include templates.yaml
`)

	root, sources, err := LoadNodeWithIncludesFrom(filepath.Join(dir, "main.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	x := findValue(t, root, "templates", "iphone-front", "device", "x")
	if got, want := sources[x], filepath.Join(dir, "templates.yaml"); got != want {
		t.Errorf("x came from %q, want %q", got, want)
	}
	// And its position is relative to that file, not to the includer.
	if x.Line != 4 {
		t.Errorf("x is at line %d of templates.yaml, want 4", x.Line)
	}

	fonts := findValue(t, root, "settings", "fonts_dir")
	if got, want := sources[fonts], filepath.Join(dir, "main.yaml"); got != want {
		t.Errorf("fonts_dir came from %q, want %q", got, want)
	}
}

func findValue(t *testing.T, n *yaml.Node, path ...string) *yaml.Node {
	t.Helper()
	for _, key := range path {
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
