package frames

// DeviceEntry describes a downloadable Apple device bezel.
type DeviceEntry struct {
	Name string
	URL  string
}

// DeviceCatalog maps device slugs to their Apple CDN download URLs.
// Update this map when Apple releases new devices.
var DeviceCatalog = map[string]DeviceEntry{
	"iphone-17": {
		Name: "iPhone 17",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-iPhone-17.dmg",
	},
	"iphone-16": {
		Name: "iPhone 16",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-iPhone-16.dmg",
	},
	"ipad": {
		Name: "iPad",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-iPad.dmg",
	},
	"ipad-air-m2": {
		Name: "iPad Air M2",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-iPad-Air-M2.dmg",
	},
	"ipad-mini": {
		Name: "iPad mini",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-iPad-mini.dmg",
	},
	"ipad-pro-m4": {
		Name: "iPad Pro M4",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-iPad-Pro-M4.dmg",
	},
	"apple-tv": {
		Name: "Apple TV",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-Apple-TV.dmg",
	},
	"apple-watch-ultra-3": {
		Name: "Apple Watch Ultra 3 (2025)",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-Apple-Watch-Ultra-3-2025.dmg",
	},
	"apple-watch-ultra-2": {
		Name: "Apple Watch Ultra 2 (2024)",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-Apple-Watch-Ultra-2-2024.dmg",
	},
	"apple-watch-series-11": {
		Name: "Apple Watch Series 11 (2025)",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-Apple-Watch-Series-11-2025.dmg",
	},
	"imac-m4": {
		Name: "iMac M4",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-iMac-M4.dmg",
	},
	"studio-displays": {
		Name: "Studio Displays",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-Studio-Displays.dmg",
	},
	"macbook-air-m5": {
		Name: "MacBook Air M5",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-MacBook-Air-M5.dmg",
	},
	"macbook-pro-m5": {
		Name: "MacBook Pro M5",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-MacBook-Pro-M5.dmg",
	},
	"macbook-neo": {
		Name: "MacBook Neo",
		URL:  "https://devimages-cdn.apple.com/design/resources/download/Bezel-MacBook-Neo.dmg",
	},
}

// CatalogSlugs returns all device slugs in sorted order.
func CatalogSlugs() []string {
	slugs := make([]string, 0, len(DeviceCatalog))
	for slug := range DeviceCatalog {
		slugs = append(slugs, slug)
	}
	// Sort for consistent output
	sortStrings(slugs)
	return slugs
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
