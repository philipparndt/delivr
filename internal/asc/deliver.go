package asc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// Deliver uploads metadata and screenshots to App Store Connect.
// platformForDisplayType returns "IOS" or "MAC_OS" based on the display type.
// platformForDisplayType maps a screenshot display type or preview type to the
// App Store Connect platform whose version owns it.
//
// This matters because assets attach to an appStoreVersion, and an app with
// both an iOS and a tvOS build has one version per platform. Everything
// non-desktop used to fall through to IOS, which quietly filed Apple TV
// screenshots against the iOS version.
func platformForDisplayType(dt string) string {
	switch {
	case dt == "APP_DESKTOP":
		return "MAC_OS"
	case dt == "APP_APPLE_TV":
		return "TV_OS"
	case dt == "APP_APPLE_VISION_PRO":
		return "VISION_OS"
	case strings.HasPrefix(dt, "APP_WATCH"):
		// Watch assets belong to the iOS version — watchOS is not a separate
		// platform in App Store Connect.
		return "IOS"
	default:
		return "IOS"
	}
}

func (c *Client) Deliver(cfg DeliverConfig) error {
	// 1. Find the app
	appID, err := c.findApp(cfg.BundleID)
	if err != nil {
		return fmt.Errorf("find app: %w", err)
	}
	fmt.Printf("Found app %s (ID: %s)\n", cfg.BundleID, appID)

	// 2. Determine which platforms we need (from screenshot display types)
	platforms := map[string]bool{}
	if cfg.Screenshots != nil {
		for _, displayTypes := range cfg.Screenshots {
			for dt := range displayTypes {
				platforms[platformForDisplayType(dt)] = true
			}
		}
	}
	// Metadata goes to all platforms; if no screenshots, default to both
	if len(platforms) == 0 {
		platforms["IOS"] = true
		platforms["MAC_OS"] = true
	}

	// 3. Process each platform
	for platform := range platforms {
		fmt.Printf("\n=== Platform: %s ===\n", platform)

		versionID, versionString, err := c.findEditableVersionForPlatform(appID, platform)
		if err != nil {
			fmt.Printf("  Skipping %s: %v\n", platform, err)
			continue
		}
		fmt.Printf("Found editable version %s (ID: %s)\n", versionString, versionID)

		localizations, err := c.getLocalizations(versionID)
		if err != nil {
			return fmt.Errorf("get localizations for %s: %w", platform, err)
		}
		fmt.Printf("Found %d localizations\n", len(localizations))

		// Update metadata
		if cfg.Metadata != nil {
			wasVerbose := c.Verbose
			c.Verbose = false

			metaBar := progress.New(
				progress.WithGradient("#1A6B5A", "#0F8B6E"),
				progress.WithWidth(40),
				progress.WithoutPercentage(),
			)
			metaDim := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

			locales := sortedKeys(cfg.Metadata)
			// Count locales that have a matching localization
			total := 0
			for _, locale := range locales {
				if _, ok := localizations[locale]; ok {
					total++
				}
			}
			done := 0

			for _, locale := range locales {
				meta := cfg.Metadata[locale]
				locID, ok := localizations[locale]
				if !ok {
					continue
				}
				if err := c.updateLocalization(locID, meta); err != nil {
					fmt.Println()
					c.Verbose = wasVerbose
					return fmt.Errorf("update %s/%s: %w", platform, locale, err)
				}
				done++
				pct := float64(done) / float64(total)
				fmt.Printf("\r  %s %s %s",
					metaDim.Render("metadata"),
					metaBar.ViewAs(pct),
					metaDim.Render(fmt.Sprintf("%d/%d", done, total)),
				)
			}
			fmt.Println()
			c.Verbose = wasVerbose
		}

		// Upload screenshots belonging to this platform
		if cfg.Screenshots != nil {
			// Suppress verbose HTTP logging during uploads
			wasVerbose := c.Verbose
			c.Verbose = false

			bar := progress.New(
				progress.WithGradient("#1A6B5A", "#0F8B6E"),
				progress.WithWidth(40),
				progress.WithoutPercentage(),
			)
			dim := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

			// Find longest locale name for alignment
			maxLen := 0
			for locale := range cfg.Screenshots {
				if len(locale) > maxLen {
					maxLen = len(locale)
				}
			}

			screenshotLocales := sortedKeys(cfg.Screenshots)
			for _, locale := range screenshotLocales {
				displayTypes := cfg.Screenshots[locale]
				locID, ok := localizations[locale]
				if !ok {
					continue
				}

				// Count total files for this locale. Previews count too, or the
				// progress bar reaches 100% and then keeps uploading.
				var totalFiles int
				for displayType, files := range displayTypes {
					if platformForDisplayType(displayType) != platform {
						continue
					}
					totalFiles += len(files)
				}
				for previewType, files := range cfg.Previews[locale] {
					if platformForDisplayType(previewType) != platform {
						continue
					}
					totalFiles += len(files)
				}
				if totalFiles == 0 {
					continue
				}

				uploaded := 0
				label := fmt.Sprintf("%-*s", maxLen, locale)
				printProgress := func() {
					pct := float64(uploaded) / float64(totalFiles)
					fmt.Printf("\r  %s %s %s",
						dim.Render(label),
						bar.ViewAs(pct),
						dim.Render(fmt.Sprintf("%d/%d", uploaded, totalFiles)),
					)
				}
				printProgress()

				for displayType, files := range displayTypes {
					if platformForDisplayType(displayType) != platform {
						continue
					}
					if err := c.uploadScreenshotSet(locID, displayType, files, func() {
						uploaded++
						printProgress()
					}); err != nil {
						fmt.Println()
						c.Verbose = wasVerbose
						return fmt.Errorf("screenshots %s/%s: %w", locale, displayType, err)
					}
				}

				// Previews ride the same localization and the same platform
				// filter. previewType and displayType use the same vocabulary,
				// so the platform can be derived the same way.
				for previewType, files := range cfg.Previews[locale] {
					if platformForDisplayType(previewType) != platform {
						continue
					}
					if err := c.UploadPreviewSet(locID, previewType, files,
						cfg.PreviewPosterFrame, func() {
							uploaded++
							printProgress()
						}); err != nil {
						fmt.Println()
						c.Verbose = wasVerbose
						return fmt.Errorf("previews %s/%s: %w", locale, previewType, err)
					}
				}
				fmt.Println() // newline after completed locale
			}

			c.Verbose = wasVerbose
		}
	}

	fmt.Println("\nDone!")
	return nil
}

// ListDisplayTypes probes which screenshot display types are allowed for this app
// by attempting to create (and immediately deleting) a set for each known type.
func (c *Client) ListDisplayTypes(bundleID string) error {
	appID, err := c.findApp(bundleID)
	if err != nil {
		return err
	}

	// All known non-iMessage display types
	candidates := []string{
		"APP_IPHONE_35", "APP_IPHONE_40", "APP_IPHONE_47", "APP_IPHONE_55",
		"APP_IPHONE_58", "APP_IPHONE_61", "APP_IPHONE_65", "APP_IPHONE_67",
		"APP_IPAD_97", "APP_IPAD_105",
		"APP_IPAD_PRO_129", "APP_IPAD_PRO_3GEN_129", "APP_IPAD_PRO_3GEN_11",
		"APP_DESKTOP",
		"APP_APPLE_TV", "APP_APPLE_VISION_PRO",
		"APP_WATCH_SERIES_3", "APP_WATCH_SERIES_4", "APP_WATCH_SERIES_7",
		"APP_WATCH_SERIES_10", "APP_WATCH_ULTRA",
	}

	for _, platform := range []string{"IOS", "MAC_OS"} {
		versionID, ver, err := c.findEditableVersionForPlatform(appID, platform)
		if err != nil {
			fmt.Printf("\n%s: no editable version found\n", platform)
			continue
		}
		fmt.Printf("\n%s — version %s\n", platform, ver)

		localizations, err := c.getLocalizations(versionID)
		if err != nil {
			return err
		}
		var locID string
		for _, id := range localizations {
			locID = id
			break
		}

		fmt.Printf("Probing display types...\n")
		var allowed []string
		for _, dt := range candidates {
			setID, err := c.createScreenshotSetForLocalization(locID, dt)
			if err != nil {
				continue
			}
			_ = c.del(fmt.Sprintf("/appScreenshotSets/%s", setID))
			allowed = append(allowed, dt)
		}

		fmt.Printf("Allowed:\n")
		for _, dt := range allowed {
			fmt.Printf("  %s\n", dt)
		}
	}
	return nil
}

func (c *Client) findApp(bundleID string) (string, error) {
	data, err := c.get(fmt.Sprintf("/apps?filter[bundleId]=%s&fields[apps]=bundleId", bundleID))
	if err != nil {
		return "", err
	}
	var resp listResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no app found with bundle ID %s", bundleID)
	}
	return resp.Data[0].ID, nil
}

func (c *Client) findEditableVersion(appID string) (string, string, error) {
	return c.findEditableVersionForPlatform(appID, "")
}

func (c *Client) findEditableVersionForPlatform(appID, platform string) (string, string, error) {
	// Look for versions in editable states
	states := []string{
		"PREPARE_FOR_SUBMISSION",
		"DEVELOPER_REJECTED",
		"REJECTED",
		"WAITING_FOR_REVIEW",
		"IN_REVIEW",
	}
	filter := strings.Join(states, ",")
	url := fmt.Sprintf("/apps/%s/appStoreVersions?filter[appStoreState]=%s&fields[appStoreVersions]=versionString,appStoreState,platform&limit=5", appID, filter)
	if platform != "" {
		url += "&filter[platform]=" + platform
	}
	data, err := c.get(url)
	if err != nil {
		return "", "", err
	}
	var resp listResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", "", err
	}
	if len(resp.Data) == 0 {
		return "", "", fmt.Errorf("no editable app store version found (states: %s)", filter)
	}

	var attrs struct {
		VersionString string `json:"versionString"`
		AppStoreState string `json:"appStoreState"`
		Platform      string `json:"platform"`
	}
	if err := json.Unmarshal(resp.Data[0].Attributes, &attrs); err != nil {
		return "", "", err
	}
	return resp.Data[0].ID, attrs.VersionString, nil
}

func (c *Client) getLocalizations(versionID string) (map[string]string, error) {
	data, err := c.get(fmt.Sprintf("/appStoreVersions/%s/appStoreVersionLocalizations?fields[appStoreVersionLocalizations]=locale", versionID))
	if err != nil {
		return nil, err
	}
	var resp listResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, r := range resp.Data {
		var attrs localizationAttrs
		if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
			continue
		}
		result[attrs.Locale] = r.ID
	}
	return result, nil
}

func (c *Client) updateLocalization(localizationID string, meta *LocaleMetadata) error {
	attrs := patchLocalizationAttributes{}
	if meta.Description != "" {
		attrs.Description = &meta.Description
	}
	if meta.PromotionalText != "" {
		attrs.PromotionalText = &meta.PromotionalText
	}
	if meta.WhatsNew != "" {
		attrs.WhatsNew = &meta.WhatsNew
	}

	err := c.patch(fmt.Sprintf("/appStoreVersionLocalizations/%s", localizationID), patchLocalization{
		Data: patchLocalizationData{
			Type:       "appStoreVersionLocalizations",
			ID:         localizationID,
			Attributes: attrs,
		},
	})

	// If whatsNew was rejected (first version, or not yet editable), retry without it
	if err != nil && attrs.WhatsNew != nil && strings.Contains(err.Error(), "whatsNew") {
		fmt.Printf("  Note: whatsNew not editable, updating without it\n")
		attrs.WhatsNew = nil
		err = c.patch(fmt.Sprintf("/appStoreVersionLocalizations/%s", localizationID), patchLocalization{
			Data: patchLocalizationData{
				Type:       "appStoreVersionLocalizations",
				ID:         localizationID,
				Attributes: attrs,
			},
		})
	}

	return err
}

func (c *Client) uploadScreenshotSet(localizationID, displayType string, files []string, onFileUploaded func()) error {
	// 1. Get existing screenshot sets for this localization
	existingSetID, err := c.findScreenshotSet(localizationID, displayType)
	if err != nil {
		return err
	}

	// 2. Delete existing screenshots in the set if it exists
	if existingSetID != "" {
		if err := c.deleteScreenshotsInSet(existingSetID); err != nil {
			return fmt.Errorf("delete existing: %w", err)
		}
	} else {
		// Create a new set
		existingSetID, err = c.createScreenshotSetForLocalization(localizationID, displayType)
		if err != nil {
			return fmt.Errorf("create set: %w", err)
		}
	}

	// 3. Upload each file
	for _, file := range files {
		if err := c.uploadScreenshot(existingSetID, file); err != nil {
			return fmt.Errorf("upload %s: %w", filepath.Base(file), err)
		}
		if onFileUploaded != nil {
			onFileUploaded()
		}
	}
	return nil
}

func (c *Client) findScreenshotSet(localizationID, displayType string) (string, error) {
	data, err := c.get(fmt.Sprintf("/appStoreVersionLocalizations/%s/appScreenshotSets?fields[appScreenshotSets]=screenshotDisplayType", localizationID))
	if err != nil {
		return "", err
	}
	var resp listResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	// Match by display type explicitly — the API filter may not work reliably
	for _, r := range resp.Data {
		var attrs struct {
			ScreenshotDisplayType string `json:"screenshotDisplayType"`
		}
		if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
			continue
		}
		if attrs.ScreenshotDisplayType == displayType {
			return r.ID, nil
		}
	}
	return "", nil
}

func (c *Client) deleteScreenshotsInSet(setID string) error {
	data, err := c.get(fmt.Sprintf("/appScreenshotSets/%s/appScreenshots?fields[appScreenshots]=fileName", setID))
	if err != nil {
		return err
	}
	var resp listResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	for _, r := range resp.Data {
		if err := c.del(fmt.Sprintf("/appScreenshots/%s", r.ID)); err != nil {
			return fmt.Errorf("delete screenshot %s: %w", r.ID, err)
		}
	}
	return nil
}

func (c *Client) createScreenshotSetForLocalization(localizationID, displayType string) (string, error) {
	data, err := c.post("/appScreenshotSets", createScreenshotSet{
		Data: createScreenshotSetData{
			Type:       "appScreenshotSets",
			Attributes: createScreenshotSetAttributes{ScreenshotDisplayType: displayType},
			Relationships: screenshotSetRelationships{
				AppStoreVersionLocalization: relationship{
					Data: relationshipData{Type: "appStoreVersionLocalizations", ID: localizationID},
				},
			},
		},
	})
	if err != nil {
		return "", err
	}
	var resp singleResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.Data.ID, nil
}

func (c *Client) uploadScreenshot(setID, filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	// 1. Reserve
	reserveData, err := c.post("/appScreenshots", createScreenshot{
		Data: createScreenshotData{
			Type: "appScreenshots",
			Attributes: createScreenshotAttrs{
				FileName: filepath.Base(filePath),
				FileSize: info.Size(),
			},
			Relationships: screenshotRelationships{
				AppScreenshotSet: relationship{
					Data: relationshipData{Type: "appScreenshotSets", ID: setID},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("reserve: %w", err)
	}

	var reserveResp singleResponse
	if err := json.Unmarshal(reserveData, &reserveResp); err != nil {
		return err
	}

	var attrs screenshotAttrs
	if err := json.Unmarshal(reserveResp.Data.Attributes, &attrs); err != nil {
		return err
	}

	// 2. Read the file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// 3. Upload each part
	for _, op := range attrs.UploadOperations {
		end := op.Offset + op.Length
		if end > len(fileData) {
			end = len(fileData)
		}
		chunk := fileData[op.Offset:end]

		headers := make(map[string]string)
		for _, h := range op.RequestHeaders {
			headers[h.Name] = h.Value
		}

		if err := c.uploadRaw(op.Method, op.URL, chunk, headers); err != nil {
			return fmt.Errorf("upload part at offset %d: %w", op.Offset, err)
		}
	}

	// 4. Commit
	checksum, err := fileMD5(filePath)
	if err != nil {
		return err
	}

	return c.patch(fmt.Sprintf("/appScreenshots/%s", reserveResp.Data.ID), commitScreenshot{
		Data: commitScreenshotData{
			Type: "appScreenshots",
			ID:   reserveResp.Data.ID,
			Attributes: commitScreenshotAttrs{
				Uploaded:           true,
				SourceFileChecksum: checksum,
			},
		},
	})
}

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
