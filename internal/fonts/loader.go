package fonts

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// Loader manages font loading and caching.
// Parsed fonts are cached and shared (they are immutable).
// Faces are created fresh per call since they are not thread-safe.
type Loader struct {
	fontsDir  string
	fontCache map[string]*opentype.Font
	mu        sync.RWMutex
}

// NewLoader creates a new font loader
func NewLoader(fontsDir string) *Loader {
	return &Loader{
		fontsDir:  fontsDir,
		fontCache: make(map[string]*opentype.Font),
	}
}

// Load loads a font at the specified size.
// Returns a new face each time (faces are not thread-safe).
// Supports:
//   - Simple filenames: "SF-Pro-Display-Bold.otf" (resolved relative to fontsDir)
//   - Absolute paths: "/System/Library/Fonts/Hiragino Sans GB.ttc"
//   - TTC collections with index: "font.ttc:0" or "/path/to/font.ttc:2"
func (l *Loader) Load(filename string, size float64) (font.Face, error) {
	fnt, err := l.getFont(filename)
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create font face: %w", err)
	}
	return face, nil
}

func (l *Loader) getFont(filename string) (*opentype.Font, error) {
	l.mu.RLock()
	fnt, ok := l.fontCache[filename]
	l.mu.RUnlock()
	if ok {
		return fnt, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock
	if fnt, ok := l.fontCache[filename]; ok {
		return fnt, nil
	}

	fnt, err := l.loadFont(filename)
	if err != nil {
		return nil, err
	}
	l.fontCache[filename] = fnt
	return fnt, nil
}

// parseFontPath splits a font specifier into path and optional TTC index.
// Examples: "font.otf" -> ("font.otf", -1), "font.ttc:2" -> ("font.ttc", 2)
func parseFontPath(filename string) (string, int) {
	// Check for TTC index suffix (e.g., "font.ttc:0")
	if idx := strings.LastIndex(filename, ":"); idx > 0 {
		suffix := filename[idx+1:]
		if n, err := strconv.Atoi(suffix); err == nil {
			return filename[:idx], n
		}
	}
	return filename, -1
}

func (l *Loader) loadFont(filename string) (*opentype.Font, error) {
	rawPath, ttcIndex := parseFontPath(filename)

	// Resolve path: absolute paths used as-is, relative resolved against fontsDir
	var path string
	if filepath.IsAbs(rawPath) {
		path = rawPath
	} else {
		path = l.fontsDir + "/" + rawPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read font file %s: %w", path, err)
	}

	// Handle TTC (TrueType Collection) files
	ext := strings.ToLower(filepath.Ext(rawPath))
	if ext == ".ttc" {
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse font collection %s: %w", path, err)
		}
		if ttcIndex < 0 {
			ttcIndex = 0 // default to first font in collection
		}
		fnt, err := collection.Font(ttcIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to get font index %d from %s: %w", ttcIndex, path, err)
		}
		return fnt, nil
	}

	fnt, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font file %s: %w", path, err)
	}

	return fnt, nil
}

// Close releases cached resources
func (l *Loader) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fontCache = make(map[string]*opentype.Font)
}
