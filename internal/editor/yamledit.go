package editor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Writing a value back into a delivr config is a text edit, not a re-marshal.
//
// These files are mostly prose. templates.yaml opens with a fifteen-line
// comment explaining why the title is 110pt, and the one number this editor is
// most likely to change sits under a paragraph explaining that it is 170 and
// not 0 and what happens if you assume otherwise. Marshalling a yaml.Node back
// out preserves comments but not the blank lines between them, not the quoting
// style, and not the ordering of a map — so a one-number change would arrive as
// a diff touching every line in the file, and the reason the number is what it
// is would be the first casualty.
//
// So: find the exact token, replace the exact token, leave every other byte
// where it was.

// SetScalar replaces the scalar at (line, column) — 1-based, as yaml.Node
// reports them — with a new value.
//
// The existing token's quoting is preserved: a value that was quoted stays
// quoted, a plain one stays plain unless the new text needs quoting. Trailing
// comments on the same line survive, because they are frequently the
// explanation for the number being replaced.
func SetScalar(src []byte, line, column int, value string) ([]byte, error) {
	lines, ending := splitLines(src)
	if line < 1 || line > len(lines) {
		return nil, fmt.Errorf("line %d is outside the file (%d lines)", line, len(lines))
	}

	row := lines[line-1]
	start := column - 1
	if start < 0 || start > len(row) {
		return nil, fmt.Errorf("column %d is outside line %d", column, line)
	}

	end, quote, err := scalarExtent(row, start)
	if err != nil {
		return nil, fmt.Errorf("line %d: %w", line, err)
	}

	lines[line-1] = row[:start] + renderScalar(value, quote) + row[end:]
	return []byte(strings.Join(lines, ending)), nil
}

// scalarExtent finds where the scalar starting at `start` ends, and how it was
// quoted.
func scalarExtent(row string, start int) (end int, quote byte, err error) {
	if start >= len(row) {
		// An empty value — "x:" with nothing after it. Nothing to replace.
		return start, 0, nil
	}

	switch row[start] {
	case '\'', '"':
		q := row[start]
		for i := start + 1; i < len(row); i++ {
			if row[i] == '\\' && q == '"' {
				i++
				continue
			}
			if row[i] == q {
				// A doubled single quote is an escaped quote, not the end.
				if q == '\'' && i+1 < len(row) && row[i+1] == '\'' {
					i++
					continue
				}
				return i + 1, q, nil
			}
		}
		return 0, 0, fmt.Errorf("unterminated %c-quoted value", q)

	case '|', '>':
		// Block scalars span lines; replacing one in place is not a single
		// token edit and this refuses rather than mangling it.
		return 0, 0, fmt.Errorf("block scalars cannot be edited in place")
	}

	// A plain scalar runs to the end of the line, minus any trailing comment.
	// " #" starts a comment; a bare "#" inside a word does not.
	rest := row[start:]
	if i := commentStart(rest); i >= 0 {
		rest = rest[:i]
	}
	return start + len(strings.TrimRight(rest, " \t")), 0, nil
}

// commentStart returns the index of a trailing comment in a plain scalar, or -1.
func commentStart(s string) int {
	for i := range len(s) {
		if s[i] == '#' && i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
			return i
		}
	}
	return -1
}

// plainSafe matches values that can be written without quotes: numbers, and
// simple words that YAML will not reinterpret.
var plainSafe = regexp.MustCompile(`^-?(\d+\.?\d*|\.\d+)$|^[A-Za-z][\w .\-]*$`)

// yamlReserved are plain words YAML reads as something other than a string.
var yamlReserved = map[string]bool{
	"true": true, "false": true, "yes": true, "no": true, "on": true,
	"off": true, "null": true, "~": true, "y": true, "n": true,
}

func renderScalar(value string, quote byte) string {
	switch quote {
	case '\'':
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	case '"':
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
	}
	if value != "" && plainSafe.MatchString(value) && !yamlReserved[strings.ToLower(value)] {
		return value
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

// InsertKey adds `key: value` to an existing mapping, after its last entry and
// at its indentation.
//
// This only ever grows a mapping that is already there. Creating the mapping
// itself — a `device:` block on a screen that has none — means deciding where
// it goes among the comments, and getting that wrong produces a file whose
// prose no longer describes the thing underneath it. The editor refuses and
// offers the YAML to paste instead.
func InsertKey(src []byte, mapping *yaml.Node, key, value string) ([]byte, error) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("cannot add %q: there is no mapping to add it to", key)
	}
	if len(mapping.Content) == 0 {
		return nil, fmt.Errorf("cannot add %q to an empty mapping", key)
	}
	if mapping.Style == yaml.FlowStyle {
		return nil, fmt.Errorf("cannot add %q to a flow mapping", key)
	}

	lines, ending := splitLines(src)

	// Insert after the last line of the mapping's last value, so a nested
	// block does not get a sibling wedged into the middle of it.
	last := lastLine(mapping)
	if last < 1 || last > len(lines) {
		return nil, fmt.Errorf("mapping ends at line %d, outside the file", last)
	}

	indent := strings.Repeat(" ", mapping.Content[0].Column-1)
	entry := indent + key + ": " + renderScalar(value, 0)

	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:last]...)
	out = append(out, entry)
	out = append(out, lines[last:]...)
	return []byte(strings.Join(out, ending)), nil
}

// lastLine is the highest line number anywhere in a node's subtree.
func lastLine(n *yaml.Node) int {
	line := n.Line
	for _, c := range n.Content {
		if l := lastLine(c); l > line {
			line = l
		}
	}
	return line
}

// splitLines splits on the file's own line ending so the rejoin reproduces it.
func splitLines(src []byte) ([]string, string) {
	if bytes.Contains(src, []byte("\r\n")) {
		return strings.Split(string(src), "\r\n"), "\r\n"
	}
	return strings.Split(string(src), "\n"), "\n"
}

// ReorderSequence permutes a block sequence of scalars in place, moving each
// item's own comments with it and leaving every other byte of the file alone.
//
// Reordering the store row is the one edit that is genuinely structural rather
// than a changed value, and it is still worth doing as a text edit: an output's
// `screens:` list is usually annotated ("the full arc of the store video, in
// order"), and per-item notes explain why a particular screenshot leads. Those
// belong to the item, so they travel with it.
//
// order is the new arrangement expressed as indices into the current one:
// order[0] is the item that should end up first.
func ReorderSequence(src []byte, seq *yaml.Node, order []int) ([]byte, error) {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("not a sequence")
	}
	if len(order) != len(seq.Content) {
		return nil, fmt.Errorf("expected %d entries, got %d", len(seq.Content), len(order))
	}

	if err := checkPermutation(order); err != nil {
		return nil, err
	}
	if seq.Style == yaml.FlowStyle {
		return reorderFlow(src, seq, order)
	}

	lines, ending := splitLines(src)

	// Each item spans from the first line of its own leading comment to its
	// last line. Anything a comment sits above is a note about that item.
	type block struct{ start, end int } // 1-based, inclusive
	blocks := make([]block, len(seq.Content))
	prevEnd := 0
	for i, item := range seq.Content {
		start := item.Line - countLines(item.HeadComment)
		end := lastLine(item)
		if start <= prevEnd || end < start || end > len(lines) {
			return nil, fmt.Errorf("entry %d does not sit on its own lines; reorder it by hand", i)
		}
		blocks[i] = block{start, end}
		prevEnd = end
	}

	// Everything before the first entry and after the last is untouched; only
	// the span between them is rewritten.
	first, last := blocks[0].start, blocks[len(blocks)-1].end
	out := make([]string, 0, len(lines))
	out = append(out, lines[:first-1]...)
	for n, from := range order {
		b := blocks[from]
		out = append(out, lines[b.start-1:b.end]...)
		// Preserve any blank line that separated this entry from the next one
		// in the original, so a spaced-out list stays spaced out.
		if n < len(order)-1 && b.end < len(lines) && strings.TrimSpace(lines[b.end]) == "" {
			out = append(out, "")
		}
	}
	out = append(out, lines[last:]...)

	return []byte(strings.Join(out, ending)), nil
}

func checkPermutation(order []int) error {
	seen := make([]bool, len(order))
	for _, from := range order {
		if from < 0 || from >= len(order) || seen[from] {
			return fmt.Errorf("the new order is not a permutation of the old one")
		}
		seen[from] = true
	}
	return nil
}

// reorderFlow handles the compact form, `screens: ["a", "b", "c"]`, by
// rewriting that one line. Each entry's own text is carried over, so quoting
// survives, and the separator is whatever the file already used.
func reorderFlow(src []byte, seq *yaml.Node, order []int) ([]byte, error) {
	lines, ending := splitLines(src)

	line := seq.Content[0].Line
	for _, item := range seq.Content {
		if item.Line != line {
			return nil, fmt.Errorf("cannot reorder a flow sequence spread over several lines")
		}
	}
	if line < 1 || line > len(lines) {
		return nil, fmt.Errorf("line %d is outside the file", line)
	}
	row := lines[line-1]

	type span struct{ start, end int }
	spans := make([]span, len(seq.Content))
	for i, item := range seq.Content {
		start := item.Column - 1
		end, _, err := scalarExtent(row, start)
		if err != nil {
			return nil, err
		}
		if start < 0 || end > len(row) || (i > 0 && start < spans[i-1].end) {
			return nil, fmt.Errorf("entry %d is not where the parser said it was", i)
		}
		spans[i] = span{start, end}
	}

	sep := ", "
	if len(spans) > 1 {
		sep = row[spans[0].end:spans[1].start]
	}

	parts := make([]string, 0, len(order))
	for _, from := range order {
		parts = append(parts, row[spans[from].start:spans[from].end])
	}

	first, last := spans[0].start, spans[len(spans)-1].end
	lines[line-1] = row[:first] + strings.Join(parts, sep) + row[last:]
	return []byte(strings.Join(lines, ending)), nil
}

func countLines(comment string) int {
	if comment == "" {
		return 0
	}
	return strings.Count(comment, "\n") + 1
}
