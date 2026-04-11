package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type deviceOption struct {
	Slug     string
	Name     string
	Width    int
	Height   int
	Platform string
	Display  string
}

var knownDevices = []deviceOption{
	{Slug: "iphone-6.9", Name: "iPhone 17 Pro Max", Width: 1320, Height: 2868, Platform: "ios", Display: "APP_IPHONE_67"},
	{Slug: "iphone-6.3", Name: "iPhone 17 Pro", Width: 1206, Height: 2622, Platform: "ios", Display: "APP_IPHONE_61"},
	{Slug: "iphone-6.1", Name: "iPhone 17", Width: 1206, Height: 2622, Platform: "ios", Display: "APP_IPHONE_61"},
	{Slug: "ipad-13", Name: "iPad Pro 13-inch (M4)", Width: 2064, Height: 2752, Platform: "ios", Display: "APP_IPAD_PRO_3GEN_129"},
	{Slug: "ipad-air-13", Name: "iPad Air 13-inch (M3)", Width: 2064, Height: 2752, Platform: "ios", Display: "APP_IPAD_PRO_3GEN_129"},
	{Slug: "ipad-air-11", Name: "iPad Air 11-inch (M3)", Width: 1668, Height: 2388, Platform: "ios", Display: "APP_IPAD_PRO_3GEN_11"},
	{Slug: "macos", Name: "macOS", Width: 2560, Height: 1600, Platform: "macos", Display: "APP_DESKTOP"},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a delivr project with example configs",
	Long:  GetHelp("init"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit() error {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	fmt.Println(titleStyle.Render("delivr project init"))
	fmt.Println()

	// Device selection
	fmt.Println("Select devices (enter numbers separated by commas):")
	fmt.Println()
	for i, d := range knownDevices {
		fmt.Printf("  %d. %s (%s)\n", i+1, d.Name, d.Platform)
	}
	fmt.Println()
	fmt.Print("Devices [1,2,4]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		input = "1,2,4"
	}

	var selectedDevices []deviceOption
	for _, s := range strings.Split(input, ",") {
		s = strings.TrimSpace(s)
		idx := 0
		fmt.Sscanf(s, "%d", &idx)
		if idx >= 1 && idx <= len(knownDevices) {
			selectedDevices = append(selectedDevices, knownDevices[idx-1])
		}
	}

	if len(selectedDevices) == 0 {
		return fmt.Errorf("no devices selected")
	}

	fmt.Printf("\nSelected: %d devices\n\n", len(selectedDevices))

	// Create directory structure
	dirs := []string{
		"screens",
		"metadata/en-US",
		".claude/commands",
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0755)
	}

	// Generate devices.yaml
	writeIfNotExists("devices.yaml", generateDevicesYAML(selectedDevices))

	// Generate templates.yaml
	writeIfNotExists("templates.yaml", generateTemplatesYAML())

	// Generate example screens
	writeIfNotExists("screens/example.yaml", generateExampleScreensYAML(selectedDevices))

	// Generate outputs.yaml
	writeIfNotExists("outputs.yaml", generateOutputsYAML(selectedDevices))

	// Generate delivr.yaml
	writeIfNotExists("delivr.yaml", generateRootYAML())

	// Generate metadata files
	writeIfNotExists("metadata/en-US/description.md", "# App Name\n\nDescribe your app here.\n")
	writeIfNotExists("metadata/en-US/promotional_text.md", "Short promotional text for the App Store.\n")
	writeIfNotExists("metadata/en-US/release_notes.md", "- Bug fixes and improvements\n")

	// Generate Claude commands
	writeIfNotExists(".claude/commands/changelog.md", generateChangelogCommand())
	writeIfNotExists(".claude/commands/update-description.md", generateUpdateDescriptionCommand())
	writeIfNotExists(".claude/commands/translate.md", generateTranslateCommand())

	fmt.Println(doneStyle.Render("Project initialized!"))
	fmt.Println()
	fmt.Println("Created files:")
	fmt.Println("  delivr.yaml              Root configuration")
	fmt.Println("  devices.yaml             Device definitions")
	fmt.Println("  templates.yaml           Screen templates")
	fmt.Println("  screens/example.yaml     Example screens")
	fmt.Println("  outputs.yaml             Output mappings")
	fmt.Println("  metadata/en-US/          App Store text templates")
	fmt.Println("  .claude/commands/        AI helper commands")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit delivr.yaml to set your Xcode project path")
	fmt.Println("  2. Edit screens/ to define your screenshot compositions")
	fmt.Println("  3. Run: delivr capture --config delivr.yaml")
	fmt.Println("  4. Run: delivr generate --config delivr.yaml")

	return nil
}

func writeIfNotExists(path string, content string) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("  skip %s (already exists)\n", path)
		return
	}
	dir := filepath.Dir(path)
	if dir != "." {
		os.MkdirAll(dir, 0755)
	}
	os.WriteFile(path, []byte(content), 0644)
	fmt.Printf("  created %s\n", path)
}

func generateDevicesYAML(devices []deviceOption) string {
	var sb strings.Builder
	for _, d := range devices {
		sb.WriteString(fmt.Sprintf("%s:\n", d.Slug))
		sb.WriteString(fmt.Sprintf("  name: %q\n", d.Name))
		sb.WriteString(fmt.Sprintf("  width: %d\n", d.Width))
		sb.WriteString(fmt.Sprintf("  height: %d\n", d.Height))
		sb.WriteString(fmt.Sprintf("  platform: %s\n", d.Platform))
		sb.WriteString(fmt.Sprintf("  screenshot_prefix: %q\n", d.Name))
		if d.Display != "" {
			sb.WriteString(fmt.Sprintf("  display_type: %q\n", d.Display))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func generateTemplatesYAML() string {
	return `default:
  title:
    font: "SF-Pro-Display-Bold.otf"
    size: 120
    color: "#FFFFFF"
    align: "center"
    y: 180
  subtitle:
    font: "SF-Pro-Display-Regular.otf"
    size: 56
    color: "#FFFFFFCC"
    align: "center"
    y: 280
  device:
    height: 2200
    y: 450
`
}

func generateExampleScreensYAML(devices []deviceOption) string {
	return `- id: "screen-1"
  template: "default"
  title:
    text: "Welcome"
  subtitle:
    text: "Your app tagline here"
  background:
    type: "gradient"
    angle: 180
    stops:
      - pos: 0.0
        color: "#3498DB"
      - pos: 1.0
        color: "#2980B9"
  device:
    source: "{device}-Screenshot 1.png"

- id: "screen-2"
  template: "default"
  title:
    text: "Feature"
  subtitle:
    text: "Describe a key feature"
  background:
    type: "gradient"
    angle: 180
    stops:
      - pos: 0.0
        color: "#9B59B6"
      - pos: 1.0
        color: "#8E44AD"
  device:
    source: "{device}-Screenshot 2.png"
`
}

func generateOutputsYAML(devices []deviceOption) string {
	var sb strings.Builder
	for _, d := range devices {
		sb.WriteString(fmt.Sprintf("- device: %q\n", d.Slug))
		sb.WriteString("  screens:\n")
		sb.WriteString("    - \"screen-1\"\n")
		sb.WriteString("    - \"screen-2\"\n")
		sb.WriteString(fmt.Sprintf("  prefix: %q\n\n", d.Name))
	}
	return sb.String()
}

func generateRootYAML() string {
	return `settings:
  fonts_dir: "/Library/Fonts"
  # bundle_id: "com.example.myapp"

devices: !include devices.yaml

# capture:
#   project: ../src/MyApp.xcodeproj
#   scheme: MyApp
#   test_target: MyAppUITests/ScreenshotTests
#   output: ./screenshots
#   appearances: [dark, light]
#   status_bar:
#     time: "9:41"
#     battery_level: 100
#   parallel: true

generate:
  screenshots_dir: ./screenshots/dark
  output: ./output/appstore
  templates: !include templates.yaml
  screens: !include screens/example.yaml
  outputs: !include outputs.yaml
  # languages: [en-US, de-DE, ja]
  # translations: !include translations.yaml
  # language_fonts: !include language-fonts.yaml

# deliver:
#   metadata_dir: ./metadata
#   screenshots_dir: ./output/appstore
`
}

func generateChangelogCommand() string {
	return `Generate a changelog from recent git commits.

Read the git log since the last tag and create a user-friendly changelog
suitable for App Store release notes.

Steps:
1. Run: git log $(git describe --tags --abbrev=0)..HEAD --oneline
2. Group changes by type (features, fixes, improvements)
3. Write the changelog to metadata/en-US/release_notes.md
4. Keep it concise — App Store limits to 4000 characters
`
}

func generateUpdateDescriptionCommand() string {
	return `Update the App Store description based on the current app features.

Read the current description from metadata/en-US/description.md and
update it to reflect the latest features and improvements.

Guidelines:
- Keep it under 4000 characters
- Lead with the most compelling features
- Use bullet points for feature lists
- Include relevant keywords naturally
- End with a call to action
`
}

func generateTranslateCommand() string {
	return `Translate App Store metadata to all configured languages.

Read the English metadata files from metadata/en-US/ and translate them
to the languages defined in delivr.yaml.

For each language:
1. Read metadata/en-US/description.md → Write to metadata/{lang}/description.md
2. Read metadata/en-US/promotional_text.md → Write to metadata/{lang}/promotional_text.md
3. Read metadata/en-US/release_notes.md → Write to metadata/{lang}/release_notes.md

Translation guidelines:
- Maintain the same structure and formatting
- Keep app-specific terms (brand names, feature names) untranslated
- Adapt idioms naturally — don't translate literally
- Respect character limits for each field
`
}
