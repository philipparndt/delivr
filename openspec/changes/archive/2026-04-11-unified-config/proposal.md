## Why

The current configuration is fragmented across multiple disconnected files: a capture config (`capture.yaml`), a screenshot generation config (`appstore.yaml` with devices, templates, screens, translations, outputs — often 1000+ lines), separate markdown files for App Store texts, and an encrypted credentials config. There's no single entry point, and configs can't include/reference other files, forcing everything into one monolithic YAML or requiring manual coordination between files.

A unified `delivr.yaml` root config with YAML file includes would let users organize their configuration cleanly: devices in one place, screen templates in another, iPhone screens separate from iPad screens, App Store texts as referenced markdown files — all composed into a single logical config through includes.

## What Changes

- Add a root `delivr.yaml` config format that serves as the single entry point for all delivr commands
- Support `!include <path>` directive in YAML files to compose configs from multiple files
- The root config defines: project settings, devices, and references to sub-configs (capture, screens, delivery)
- `delivr generate`, `delivr capture`, and `delivr deliver` all read from the same root config (or their relevant section)
- Screens, templates, and translations can be split across multiple included files (e.g., `screens/iphone.yaml`, `screens/ipad.yaml`)
- App Store texts (description, promo, release notes) remain as markdown files, referenced by path in the delivery config
- **BREAKING**: Existing standalone config files still work but the new unified format is the recommended approach

## Capabilities

### New Capabilities
- `yaml-includes`: Support for `!include` directive in YAML config files to compose configs from multiple files
- `unified-config`: Root `delivr.yaml` format that ties together capture, generate, and deliver configs with shared device definitions

### Modified Capabilities

## Impact

- **Code**: New YAML loader with include support in `internal/config/`, updated config types to support unified format, all commands updated to accept root config
- **Config**: New `delivr.yaml` format; existing per-command configs remain supported
- **CLI**: Commands can accept `--config delivr.yaml` and extract their relevant section, or continue using standalone configs
- **Users**: Can migrate to unified config incrementally — existing configs keep working
