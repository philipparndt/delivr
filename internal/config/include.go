package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadWithIncludes reads a YAML file and resolves all !include directives
// recursively. Returns the processed YAML bytes ready for unmarshaling.
func LoadWithIncludes(path string) ([]byte, error) {
	visited := make(map[string]bool)
	node, err := loadNode(path, visited, nil)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(node)
}

// LoadNodeWithIncludes reads a YAML file, resolves includes, and returns the node tree.
func LoadNodeWithIncludes(path string) (*yaml.Node, error) {
	visited := make(map[string]bool)
	return loadNode(path, visited, nil)
}

// NodeSources maps each node in a resolved tree to the absolute path of the
// file it was parsed from.
//
// Resolving an !include splices another document's nodes into this one, which
// makes the tree convenient to read and useless for writing back: a value that
// appears under `generate.templates` may physically live in templates.yaml,
// three files away. Each node's Line and Column are relative to its own file,
// so file plus position is enough to edit the exact token it came from.
type NodeSources map[*yaml.Node]string

// LoadNodeWithIncludesFrom is LoadNodeWithIncludes, also reporting which file
// each node came from.
func LoadNodeWithIncludesFrom(path string) (*yaml.Node, NodeSources, error) {
	visited := make(map[string]bool)
	sources := make(NodeSources)
	node, err := loadNode(path, visited, sources)
	if err != nil {
		return nil, nil, err
	}
	return node, sources, nil
}

func loadNode(path string, visited map[string]bool, sources NodeSources) (*yaml.Node, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	if visited[absPath] {
		return nil, fmt.Errorf("circular include detected: %s", absPath)
	}
	visited[absPath] = true
	defer func() { delete(visited, absPath) }()

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if doc.Kind == 0 {
		return &doc, nil
	}

	// Tag before resolving includes, so the nodes spliced in below keep the
	// file they were parsed from rather than picking up this one.
	if sources != nil {
		tagSource(doc.Content[0], absPath, sources)
	}

	baseDir := filepath.Dir(absPath)
	if err := resolveIncludes(doc.Content[0], baseDir, visited, sources); err != nil {
		return nil, fmt.Errorf("in %s: %w", path, err)
	}

	return doc.Content[0], nil
}

func tagSource(node *yaml.Node, path string, sources NodeSources) {
	if node == nil {
		return
	}
	sources[node] = path
	for _, child := range node.Content {
		tagSource(child, path, sources)
	}
}

func resolveIncludes(node *yaml.Node, baseDir string, visited map[string]bool, sources NodeSources) error {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.MappingNode:
		// Process key-value pairs
		for i := 0; i < len(node.Content)-1; i += 2 {
			valueNode := node.Content[i+1]

			if valueNode.Tag == "!include" {
				// Replace with included file contents
				includePath := filepath.Join(baseDir, valueNode.Value)
				included, err := loadNode(includePath, visited, sources)
				if err != nil {
					return fmt.Errorf("include %s: %w", valueNode.Value, err)
				}
				node.Content[i+1] = included
			} else {
				if err := resolveIncludes(valueNode, baseDir, visited, sources); err != nil {
					return err
				}
			}
		}

	case yaml.SequenceNode:
		// Process sequence items — flatten included sequences
		var newContent []*yaml.Node
		for _, item := range node.Content {
			if item.Tag == "!include" {
				includePath := filepath.Join(baseDir, item.Value)
				included, err := loadNode(includePath, visited, sources)
				if err != nil {
					return fmt.Errorf("include %s: %w", item.Value, err)
				}

				// Flatten: if included node is a sequence, merge its items
				if included.Kind == yaml.SequenceNode {
					newContent = append(newContent, included.Content...)
				} else {
					newContent = append(newContent, included)
				}
			} else {
				if err := resolveIncludes(item, baseDir, visited, sources); err != nil {
					return err
				}
				newContent = append(newContent, item)
			}
		}
		node.Content = newContent

	case yaml.ScalarNode:
		// Nothing to do for scalars (unless tagged, handled by parent)

	case yaml.AliasNode:
		// Aliases are resolved by yaml.v3 automatically
	}

	return nil
}
