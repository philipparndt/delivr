## Context

Current state across iot-panels-marketing and mqtt-analyzer-marketing:
- `appstore.yaml` (1064 lines): devices, templates, screens, translations, outputs, languages — all in one file
- `capture-mqtt-analyzer.yaml`: separate capture config (project, scheme, devices, appearances)
- `appstore-config.yaml`: encrypted ASC credentials (SOPS)
- `appstore-text*.md`, `appstore-promo*.md`, `news*.md`: per-language markdown files

Problem: devices are defined separately in capture and generate configs. Templates, screens, and translations bloat a single file. No way to share common definitions.

## Goals / Non-Goals

**Goals:**
- Single `delivr.yaml` root config that all commands can read
- `!include path.yaml` directive to compose configs from multiple files
- Shared `devices` section used by both capture and generate
- Clean separation: devices, templates, screens (per device class), translations, delivery settings
- Support multiple include files for screens (e.g., `screens/iphone.yaml`, `screens/ipad.yaml`)
- Backward compatible — standalone capture/generate configs still work

**Non-Goals:**
- YAML anchors/aliases (Go's yaml.v3 already supports these)
- Remote includes (HTTP URLs)
- Config validation beyond what exists today
- Merging/overlaying multiple root configs

## Decisions

### 1. `!include` as a custom YAML tag

Use a custom YAML unmarshaler that processes `!include` tags before normal parsing. The include path is relative to the file containing the `!include` directive.

```yaml
# delivr.yaml
devices: !include devices.yaml
templates: !include templates.yaml
screens:
  - !include screens/iphone.yaml
  - !include screens/ipad.yaml
  - !include screens/macos.yaml
```

The `!include` tag works at any level — it can include a whole file as a mapping, a sequence, or even a scalar. When used in a sequence, included files that contain sequences are flattened into the parent.

### 2. Root config format

```yaml
# delivr.yaml
settings:
  fonts_dir: /Library/Fonts
  bundle_id: de.rnd7.MQTTAnalyzer

devices: !include devices.yaml

capture:
  project: ../src/MQTTAnalyzer.xcodeproj
  scheme: MQTTAnalyzer
  test_target: MQTTAnalyzerUITests/ScreenshotTests
  appearances: [dark, light]
  status_bar:
    time: "9:41"
    battery_level: 100
  parallel: true

generate:
  templates: !include templates.yaml
  screens:
    - !include screens/iphone.yaml
    - !include screens/ipad.yaml
  outputs: !include outputs.yaml
  languages: [en-US, de-DE, ja]
  translations: !include translations.yaml
  language_fonts: !include language-fonts.yaml

deliver:
  metadata_dir: ./metadata
  screenshots_dir: ./output/appstore
```

### 3. Devices shared between capture and generate

The `devices` section at the root level is shared. Capture uses the device names + platform. Generate uses the device names + dimensions + display_type. Both are defined together:

```yaml
# devices.yaml
iphone-6.9:
  name: "iPhone 17 Pro Max"
  width: 1320
  height: 2868
  platform: ios
  screenshot_prefix: "iPhone 17 Pro Max"
  display_type: APP_IPHONE_67
```

The `platform` field is new — used by capture, ignored by generate.

### 4. Command config resolution

Each command checks for its section in the root config:
- `delivr capture --config delivr.yaml` → reads `capture` section + `devices`
- `delivr generate --config delivr.yaml` → reads `generate` section + `devices` + `settings`
- `delivr deliver --config delivr.yaml` → reads `deliver` section + `devices` + `settings`

Auto-detection: if the config has a `capture`/`generate`/`deliver` key, it's a root config. If it has `project`/`scheme`, it's a standalone capture config. If it has `screens`/`outputs`, it's a standalone generate config.

### 5. Include implementation

Process includes in two passes:
1. Read the YAML file as raw bytes
2. Walk the YAML node tree, replacing `!include` tagged nodes with the contents of the referenced file (recursively)
3. Unmarshal the resolved tree into the target struct

This uses `gopkg.in/yaml.v3`'s `yaml.Node` tree for processing before final unmarshal.

### 6. Sequence flattening for screens

When `!include` is used inside a YAML sequence and the included file contains a sequence, the items are flattened into the parent:

```yaml
# delivr.yaml
screens:
  - !include screens/iphone.yaml   # contains 9 screens
  - !include screens/ipad.yaml     # contains 9 screens
# Result: 18 screens in a flat list
```

## Risks / Trade-offs

- **Include cycles** → Mitigated by tracking visited files and erroring on cycles
- **Error messages** → Include errors need to show which file and line caused the issue. Mitigated by wrapping errors with the include path.
- **Backward compatibility** → Existing configs work unchanged since they don't use `!include` or root-level keys
