package config

// Config is the root configuration structure
type Config struct {
	Settings     Settings                                  `yaml:"settings"`
	Devices      map[string]Device                         `yaml:"devices"`
	Templates    map[string]ScreenTemplate                 `yaml:"templates,omitempty"`
	Screens      []Screen                                  `yaml:"screens"`
	Outputs      []Output                                  `yaml:"outputs"`
	Languages     []string                                  `yaml:"languages,omitempty"`      // e.g. ["en-US", "de-DE", ...]
	Translations  map[string]map[string]ScreenTranslation   `yaml:"translations,omitempty"`   // lang -> screenID -> translation
	LanguageFonts map[string]LanguageFontConfig              `yaml:"language_fonts,omitempty"` // lang -> font overrides
}

// LanguageFontConfig specifies font and size overrides for a language
type LanguageFontConfig struct {
	TitleFont    string  `yaml:"title_font,omitempty"`
	SubtitleFont string  `yaml:"subtitle_font,omitempty"`
	TitleSize    float64 `yaml:"title_size,omitempty"`
	SubtitleSize float64 `yaml:"subtitle_size,omitempty"`
}

// ScreenTranslation holds translated title/subtitle text for a screen
type ScreenTranslation struct {
	Title    string `yaml:"title,omitempty"`
	Subtitle string `yaml:"subtitle,omitempty"`
}

// ScreenTemplate defines reusable screen properties
type ScreenTemplate struct {
	Title      *TextConfig   `yaml:"title,omitempty"`
	Subtitle   *TextConfig   `yaml:"subtitle,omitempty"`
	Background *Background   `yaml:"background,omitempty"`
	Device     *DeviceImage  `yaml:"device,omitempty"`  // single device
	Devices    []DeviceImage `yaml:"devices,omitempty"` // multiple devices
}

// Settings contains global settings
type Settings struct {
	FontsDir       string `yaml:"fonts_dir"`
	ScreenshotsDir string `yaml:"screenshots_dir"`
	BundleID       string `yaml:"bundle_id,omitempty"`
}

// Device defines a target device's dimensions
type Device struct {
	Name             string `yaml:"name"`
	Width            int    `yaml:"width"`
	Height           int    `yaml:"height"`
	ScreenshotPrefix string `yaml:"screenshot_prefix,omitempty"` // prefix for screenshot filenames
	DisplayType      string `yaml:"display_type,omitempty"`      // App Store Connect screenshot display type
}

// Screen defines a single screenshot configuration
type Screen struct {
	ID         string        `yaml:"id"`
	Template   string        `yaml:"template,omitempty"` // reference to a template
	Title      *TextConfig   `yaml:"title,omitempty"`
	Subtitle   *TextConfig   `yaml:"subtitle,omitempty"`
	Background *Background   `yaml:"background,omitempty"`
	Device     *DeviceImage  `yaml:"device,omitempty"`  // single device (backwards compatible)
	Devices    []DeviceImage `yaml:"devices,omitempty"` // multiple devices (rendered in order, first = back)
}

// TextConfig defines text rendering settings
type TextConfig struct {
	Text     string  `yaml:"text"`
	Font     string  `yaml:"font"`
	Size     float64 `yaml:"size"`
	Color    string  `yaml:"color"`
	Align    string  `yaml:"align"` // "left", "center", "right"
	Y        float64 `yaml:"y"`
	X        float64 `yaml:"x,omitempty"`         // optional X offset from aligned position
	MaxWidth float64 `yaml:"max_width,omitempty"` // max width for word wrapping (0 = no wrap)
}

// Background defines the background style
type Background struct {
	Type  string         `yaml:"type"` // "solid" or "gradient"
	Color string         `yaml:"color,omitempty"`
	Angle float64        `yaml:"angle,omitempty"` // degrees: 0=top-down, 90=left-right
	Stops []GradientStop `yaml:"stops,omitempty"`
}

// GradientStop defines a color stop in a gradient
type GradientStop struct {
	Pos   float64 `yaml:"pos"`   // 0.0 to 1.0
	Color string  `yaml:"color"` // hex color
}

// DeviceImage defines how to render the device screenshot
type DeviceImage struct {
	Source        string  `yaml:"source"`                   // path pattern with {device} placeholder
	Width         int     `yaml:"width,omitempty"`          // 0 = auto from height
	Height        int     `yaml:"height,omitempty"`         // target height
	X             float64 `yaml:"x,omitempty"`              // offset from center
	Y             float64 `yaml:"y,omitempty"`              // Y position
	Template      string  `yaml:"template,omitempty"`       // path to .frame.json device template
	AutoCrop      bool    `yaml:"auto_crop,omitempty"`      // crop to content bounds (removes transparent padding)
	CropThreshold int     `yaml:"crop_threshold,omitempty"` // alpha threshold 0-255 (default 10, lower includes more shadow)
	CropPadding   int     `yaml:"crop_padding,omitempty"`   // padding around detected content bounds
}

// Output defines which screens to render for which device
type Output struct {
	Device  string   `yaml:"device"`
	Screens []string `yaml:"screens"`
	Prefix  string   `yaml:"prefix"`
}
