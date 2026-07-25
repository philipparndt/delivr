package asc

import "encoding/json"

// JSON:API response types

type listResponse struct {
	Data []resource `json:"data"`
}

type singleResponse struct {
	Data resource `json:"data"`
}

type resource struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

// Localization attributes (for reading)
type localizationAttrs struct {
	Locale string `json:"locale"`
}

// Screenshot attributes (response from reserve)
type screenshotAttrs struct {
	UploadOperations []uploadOperation `json:"uploadOperations,omitempty"`
}

type uploadOperation struct {
	Method         string          `json:"method"`
	URL            string          `json:"url"`
	Length         int             `json:"length"`
	Offset         int             `json:"offset"`
	RequestHeaders []requestHeader `json:"requestHeaders"`
}

type requestHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Request payloads

type patchLocalization struct {
	Data patchLocalizationData `json:"data"`
}

type patchLocalizationData struct {
	Type       string                      `json:"type"`
	ID         string                      `json:"id"`
	Attributes patchLocalizationAttributes `json:"attributes"`
}

type patchLocalizationAttributes struct {
	Description     *string `json:"description,omitempty"`
	PromotionalText *string `json:"promotionalText,omitempty"`
	WhatsNew        *string `json:"whatsNew,omitempty"`
}

type createScreenshotSet struct {
	Data createScreenshotSetData `json:"data"`
}

type createScreenshotSetData struct {
	Type          string                        `json:"type"`
	Attributes    createScreenshotSetAttributes `json:"attributes"`
	Relationships screenshotSetRelationships    `json:"relationships"`
}

type createScreenshotSetAttributes struct {
	ScreenshotDisplayType string `json:"screenshotDisplayType"`
}

type screenshotSetRelationships struct {
	AppStoreVersionLocalization relationship `json:"appStoreVersionLocalization"`
}

type relationship struct {
	Data relationshipData `json:"data"`
}

type relationshipData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type createScreenshot struct {
	Data createScreenshotData `json:"data"`
}

type createScreenshotData struct {
	Type          string                  `json:"type"`
	Attributes    createScreenshotAttrs   `json:"attributes"`
	Relationships screenshotRelationships `json:"relationships"`
}

type createScreenshotAttrs struct {
	FileName string `json:"fileName"`
	FileSize int64  `json:"fileSize"`
}

type screenshotRelationships struct {
	AppScreenshotSet relationship `json:"appScreenshotSet"`
}

type commitScreenshot struct {
	Data commitScreenshotData `json:"data"`
}

type commitScreenshotData struct {
	Type       string                `json:"type"`
	ID         string                `json:"id"`
	Attributes commitScreenshotAttrs `json:"attributes"`
}

type commitScreenshotAttrs struct {
	Uploaded           bool   `json:"uploaded"`
	SourceFileChecksum string `json:"sourceFileChecksum"`
}

// DeliverConfig holds all data needed for a deliver operation.
type DeliverConfig struct {
	BundleID    string
	Metadata    map[string]*LocaleMetadata     // locale -> metadata
	Screenshots map[string]map[string][]string // locale -> displayType -> file paths
	// Previews are keyed the same way, by previewType rather than
	// displayType — the two vocabularies happen to overlap (APP_IPHONE_67,
	// APP_APPLE_TV), but they are separate fields in the API.
	Previews map[string]map[string][]string // locale -> previewType -> file paths
	// Poster frame for every uploaded preview, in seconds. One value rather
	// than per-file: a set of previews for one device shares a shape, and
	// nobody wants to tune this per locale.
	PreviewPosterFrame float64
}

// LocaleMetadata holds the text fields for a single locale.
type LocaleMetadata struct {
	Description     string
	PromotionalText string
	WhatsNew        string
}
