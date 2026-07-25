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

	// Backdrop darkens what is behind the text so it stays legible over busy
	// imagery. Without it, copy laid over a full-bleed screenshot competes with
	// whatever happens to be underneath — a bright sprite behind one word is
	// enough to make a headline unreadable.
	Backdrop *TextBackdrop `yaml:"backdrop,omitempty"`
}

// TextBackdrop is drawn behind a text block for contrast.
type TextBackdrop struct {
	// "scrim" (default) fades from Color at the nearest edge to transparent,
	// which keeps a full-bleed image looking full-bleed. "panel" draws a
	// rounded box tight around the text instead.
	Type string `yaml:"type,omitempty"`

	// Fill colour, alpha included: "#000000B0" is a common scrim.
	Color string `yaml:"color"`

	// Panel only: space between the text and the box edge, and the corner
	// radius of that box.
	Padding float64 `yaml:"padding,omitempty"`
	Radius  float64 `yaml:"radius,omitempty"`

	// Scrim only: how far the fade reaches from the edge. Defaults to just past
	// the text block, so the copy always sits in the opaque part.
	Height float64 `yaml:"height,omitempty"`
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

// VideoConfig drives App Store preview videos: recording them off a simulator,
// cutting them to Apple's spec, and uploading them.
//
// Apple's constraints are tight and unforgiving — 15 to 30 seconds, one of a
// fixed set of resolutions per device family, H.264 or ProRes — and a preview
// that misses any of them is rejected after upload rather than before. The
// per-device Spec below exists so those numbers live in config next to the
// screenshot sizes, instead of in a shell script.
type VideoConfig struct {
	// Output directory for the finished previews.
	Output string `yaml:"output,omitempty"`

	// Recording: how to get the app into the state worth filming.
	Record *VideoRecordConfig `yaml:"record,omitempty"`

	// One entry per device family to publish a preview for. Keys match the
	// device keys in devices.yaml.
	Devices map[string]VideoDeviceConfig `yaml:"devices,omitempty"`
}

// VideoRecordConfig describes how to drive the app while filming.
type VideoRecordConfig struct {
	// Built .app to install. Supports the same shell-free path expansion as the
	// rest of the config.
	App string `yaml:"app,omitempty"`
	// Xcode project and scheme, if delivr should build before recording.
	Project string `yaml:"project,omitempty"`
	Scheme  string `yaml:"scheme,omitempty"`

	// Launch arguments that put the app into its scripted sequence.
	LaunchArgs []string `yaml:"launch_args,omitempty"`

	// Seconds to record. Should comfortably exceed the sequence so the trim
	// below has material to cut from.
	Duration float64 `yaml:"duration,omitempty"`

	// Run the sequence once before recording, discarding the result. The first
	// run of a scene pays for shader and texture caches, and that hitching is
	// otherwise the first thing a viewer sees.
	WarmUp bool `yaml:"warm_up,omitempty"`
}

// VideoDeviceConfig is the per-device output spec.
type VideoDeviceConfig struct {
	// Simulator to record on. Defaults to the devices.yaml entry's name.
	Simulator string `yaml:"simulator,omitempty"`

	// Built .app for this device, overriding video.record.app. Usually
	// required rather than optional: a project targeting both iOS and tvOS
	// builds a separate bundle per platform, so one shared path cannot serve
	// both.
	App string `yaml:"app,omitempty"`

	// Final pixel size. Must be one Apple accepts for this device family.
	Width  int `yaml:"width"`
	Height int `yaml:"height"`

	// Seconds of finished video. Apple requires 15-30.
	Duration float64 `yaml:"duration,omitempty"`

	// Where to start the cut. When AutoTrim is set this is measured from the
	// detected start instead of from the head of the recording.
	Start float64 `yaml:"start,omitempty"`

	// Find the first frame after the opening black by luminance, and cut from
	// there. Simulator boot and launch take a variable amount of time, so a
	// fixed offset drifts between runs and the cut lands somewhere different
	// every time.
	AutoTrim bool `yaml:"auto_trim,omitempty"`

	// Poster frame, in seconds from the start of the finished video. Apple
	// wants a timecode; this is the friendlier form of it.
	PosterFrame float64 `yaml:"poster_frame,omitempty"`

	// ASC preview type, e.g. "APP_IPHONE_67" or "APP_APPLE_TV". Defaults to the
	// devices.yaml display_type, which uses the same vocabulary.
	PreviewType string `yaml:"preview_type,omitempty"`
}
