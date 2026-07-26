package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/philipparndt/delivr/internal/config"
)

// Project is the loaded config, in both the forms the editor needs: the
// resolved struct the renderer works from, and the YAML node tree that knows
// which physical file every value came from.
type Project struct {
	mu sync.RWMutex

	// ConfigPath is the file the editor was pointed at.
	ConfigPath string
	// Dir is the directory config paths resolve against.
	Dir string
	// IsRoot records whether this is a unified delivr.yaml or a standalone
	// generate config, which decides where in the tree things live.
	IsRoot bool

	cfg     *config.Config
	tree    *yaml.Node
	sources config.NodeSources
}

// LoadProject reads a config the same way `generate` does, and additionally
// keeps the YAML tree so values can be traced back to their source file.
func LoadProject(path string) (*Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	p := &Project{ConfigPath: abs, Dir: filepath.Dir(abs)}
	if err := p.reload(); err != nil {
		return nil, err
	}
	return p, nil
}

// Reload re-reads the config from disk. Called after a write-back so the
// editor's idea of the file's contents, and every line number in it, stays
// true.
func (p *Project) Reload() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reload()
}

func (p *Project) reload() error {
	isRoot, err := config.IsRootConfig(p.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var cfg *config.Config
	if isRoot {
		rootCfg, err := config.LoadRootConfig(p.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		if cfg, err = rootCfg.ToGenerateConfig(); err != nil {
			return fmt.Errorf("failed to prepare config: %w", err)
		}
	} else if cfg, err = config.Load(p.ConfigPath); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tree, sources, err := config.LoadNodeWithIncludesFrom(p.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read config structure: %w", err)
	}

	p.IsRoot, p.cfg, p.tree, p.sources = isRoot, cfg, tree, sources
	return nil
}

// Config returns the resolved configuration.
func (p *Project) Config() *config.Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cfg
}

// Rel renders a path for display, relative to the config's directory.
func (p *Project) Rel(path string) string {
	if r, err := filepath.Rel(p.Dir, path); err == nil {
		return r
	}
	return path
}

// generateRoot is the mapping that holds templates/screens: the `generate:`
// section in a unified config, or the document root in a standalone one.
func (p *Project) generateRoot() *yaml.Node {
	if !p.IsRoot {
		return p.tree
	}
	return mapValue(p.tree, "generate")
}

// TemplateDeviceNode returns the mapping for a device inside a named template,
// and the file it lives in.
func (p *Project) TemplateDeviceNode(name string, index int) (*yaml.Node, string) {
	tmpl := mapValue(mapValue(p.generateRoot(), "templates"), name)
	return p.deviceNode(tmpl, index)
}

// ScreenDeviceNode returns the mapping for a device inside a screen, and the
// file it lives in. The screen index matches the resolved config's ordering,
// because resolving an !include of a sequence splices its items in place.
func (p *Project) ScreenDeviceNode(screenIndex, index int) (*yaml.Node, string) {
	screens := mapValue(p.generateRoot(), "screens")
	if screens == nil || screens.Kind != yaml.SequenceNode ||
		screenIndex < 0 || screenIndex >= len(screens.Content) {
		return nil, ""
	}
	return p.deviceNode(screens.Content[screenIndex], index)
}

// ScreenTextNode returns the mapping for a screen's title or subtitle.
func (p *Project) ScreenTextNode(screenIndex int, which string) (*yaml.Node, string) {
	screens := mapValue(p.generateRoot(), "screens")
	if screens == nil || screens.Kind != yaml.SequenceNode ||
		screenIndex < 0 || screenIndex >= len(screens.Content) {
		return nil, ""
	}
	n := mapValue(screens.Content[screenIndex], which)
	return n, p.sources[n]
}

// TranslationNode returns the mapping holding one screen's copy in one
// language, and the file it lives in.
func (p *Project) TranslationNode(lang, screenID string) (*yaml.Node, string) {
	n := mapValue(mapValue(mapValue(p.generateRoot(), "translations"), lang), screenID)
	return n, p.sources[n]
}

// TemplateTextNode returns the mapping for a template's title or subtitle.
func (p *Project) TemplateTextNode(name, which string) (*yaml.Node, string) {
	n := mapValue(mapValue(mapValue(p.generateRoot(), "templates"), name), which)
	return n, p.sources[n]
}

// OutputScreensNode returns the `screens:` sequence of an output, and the file
// it lives in — the list whose order is the store row's order.
func (p *Project) OutputScreensNode(device string) (*yaml.Node, string) {
	outputs := mapValue(p.generateRoot(), "outputs")
	if outputs == nil || outputs.Kind != yaml.SequenceNode {
		return nil, ""
	}
	for _, o := range outputs.Content {
		if d := mapValue(o, "device"); d != nil && d.Value == device {
			n := mapValue(o, "screens")
			return n, p.sources[n]
		}
	}
	return nil, ""
}

// Device forms. Which one a screen is in decides both whether a screen-level
// x or y is a delta and which files are worth writing to at all.
const (
	// FormSingle: the screen's `device:` is merged with the template's by
	// mergeDeviceImage, which ADDS x and y rather than replacing them.
	FormSingle = "single"
	// FormScreenArray: the screen declares `devices:`, and mergeDevicesArray
	// returns it verbatim. The template contributes nothing — not merged, not
	// added — so values here are literal and the template is unwritable.
	FormScreenArray = "screen-array"
	// FormTemplateArray: the template declares `devices:` and the screen does
	// not. The template's list is used as-is and the screen's own `device:`,
	// if any, is ignored entirely.
	FormTemplateArray = "template-array"
)

// DeviceForm reports how a screen's rendered device list was built.
//
// This is the difference that silently corrupts a write: on a `devices:` screen
// the resolved x IS the written x, while on a single-`device:` screen it is the
// template's plus the screen's. Treating the second rule as universal writes a
// value short by the template's, and the device jumps.
func (p *Project) DeviceForm(screenIndex int, templateName string) string {
	screens := mapValue(p.generateRoot(), "screens")
	if screens != nil && screens.Kind == yaml.SequenceNode &&
		screenIndex >= 0 && screenIndex < len(screens.Content) {
		if d := mapValue(screens.Content[screenIndex], "devices"); d != nil &&
			d.Kind == yaml.SequenceNode && len(d.Content) > 0 {
			return FormScreenArray
		}
	}
	if templateName != "" {
		tmpl := mapValue(mapValue(p.generateRoot(), "templates"), templateName)
		if d := mapValue(tmpl, "devices"); d != nil &&
			d.Kind == yaml.SequenceNode && len(d.Content) > 0 {
			return FormTemplateArray
		}
	}
	return FormSingle
}

// deviceNode picks the right device out of a screen or template mapping: the
// single `device:` when index is 0 and there is no array, otherwise the entry
// at that index of `devices:`.
func (p *Project) deviceNode(owner *yaml.Node, index int) (*yaml.Node, string) {
	if owner == nil {
		return nil, ""
	}
	if devices := mapValue(owner, "devices"); devices != nil && devices.Kind == yaml.SequenceNode {
		if index < 0 || index >= len(devices.Content) {
			return nil, ""
		}
		n := devices.Content[index]
		return n, p.sources[n]
	}
	if index != 0 {
		return nil, ""
	}
	n := mapValue(owner, "device")
	return n, p.sources[n]
}

// SourceOf reports which file a node was parsed from.
func (p *Project) SourceOf(n *yaml.Node) string { return p.sources[n] }

// WriteFile replaces a config file's contents and reloads.
func (p *Project) WriteFile(path string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	info, err := os.Stat(path)
	mode := os.FileMode(0644)
	if err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return err
	}
	return p.reload()
}

// mapValue looks a key up in a mapping node, tolerating nils and documents.
func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
