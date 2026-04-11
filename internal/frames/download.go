package frames

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckHdiutil verifies that hdiutil is available (macOS only).
func CheckHdiutil() error {
	_, err := exec.LookPath("hdiutil")
	if err != nil {
		return fmt.Errorf("hdiutil not found — DMG extraction requires macOS")
	}
	return nil
}

// DownloadDMG downloads a DMG file from the given URL to a temporary file.
// The caller is responsible for removing the temp file.
func DownloadDMG(url string, verbose bool) (string, error) {
	if verbose {
		fmt.Printf("  Downloading %s\n", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "delivr-bezel-*.dmg")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("download failed: %w", err)
	}
	tmpFile.Close()

	return tmpFile.Name(), nil
}

// ExtractPNGs mounts a DMG, copies all PNG files to outputDir, and unmounts.
func ExtractPNGs(dmgPath, outputDir string, verbose bool) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Mount the DMG — pipe "Y\n" to stdin to accept any EULA/license agreement.
	// Apple's design resource DMGs include a license that hdiutil prompts for.
	// We omit -quiet so the license text flows through stdout without blocking.
	cmd := exec.Command("hdiutil", "attach", "-nobrowse", "-noverify", "-plist", dmgPath)
	cmd.Stdin = strings.NewReader("Y\n")
	mountOutput, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to mount DMG: %w", err)
	}

	// Parse mount point — the output contains the license text followed by plist XML.
	// parseMountPoint scans for the /Volumes/ path in the plist portion.
	mountPoint := parseMountPoint(string(mountOutput))
	if mountPoint == "" {
		return nil, fmt.Errorf("could not determine mount point from hdiutil output")
	}

	if verbose {
		fmt.Printf("  Mounted at %s\n", mountPoint)
	}

	// Find and copy all PNG files
	var copied []string
	err = filepath.Walk(mountPoint, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors in walk
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".png") {
			return nil
		}

		destPath := filepath.Join(outputDir, info.Name())
		if err := copyFile(path, destPath); err != nil {
			return fmt.Errorf("failed to copy %s: %w", info.Name(), err)
		}
		copied = append(copied, destPath)
		if verbose {
			fmt.Printf("  Extracted: %s\n", info.Name())
		}
		return nil
	})

	// Always unmount, even if walk had errors
	unmountErr := exec.Command("hdiutil", "detach", "-quiet", mountPoint).Run()

	if err != nil {
		return copied, err
	}
	if unmountErr != nil {
		return copied, fmt.Errorf("failed to unmount DMG: %w", unmountErr)
	}

	return copied, nil
}

// HasExistingPNGs checks if any PNG files with the given slug prefix exist in outputDir.
func HasExistingPNGs(outputDir, slug string) bool {
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return false
	}

	// Check for any file matching the device name pattern
	entry, ok := DeviceCatalog[slug]
	if !ok {
		return false
	}

	// Derive expected filename prefix from the DMG URL
	dmgBase := filepath.Base(entry.URL)
	prefix := strings.TrimSuffix(dmgBase, ".dmg")

	matches, _ := filepath.Glob(filepath.Join(outputDir, prefix+"*"))
	return len(matches) > 0
}

// DownloadAndExtract downloads a DMG from url, extracts PNGs to outputDir, and cleans up.
func DownloadAndExtract(url, outputDir string, verbose bool) ([]string, error) {
	dmgPath, err := DownloadDMG(url, verbose)
	if err != nil {
		return nil, err
	}
	defer os.Remove(dmgPath)

	return ExtractPNGs(dmgPath, outputDir, verbose)
}

// parseMountPoint extracts the /Volumes/... path from hdiutil plist output.
func parseMountPoint(plistOutput string) string {
	// Look for lines containing /Volumes/
	for _, line := range strings.Split(plistOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		// In plist XML, mount points appear as <string>/Volumes/...</string>
		if strings.HasPrefix(trimmed, "<string>/Volumes/") {
			path := strings.TrimPrefix(trimmed, "<string>")
			path = strings.TrimSuffix(path, "</string>")
			return path
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
