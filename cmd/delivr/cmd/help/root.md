App Store screenshot generator and delivery tool.

Generate professional App Store screenshots from YAML configuration,
create 3D device mockups with Rotato, and deliver to App Store Connect.

## Commands

| Command    | Description                                    |
|------------|------------------------------------------------|
| `generate` | Generate App Store screenshots from config     |
| `rotato`   | Batch process images through Rotato 3D         |
| `deliver`  | Upload metadata and screenshots to App Store   |
| `version`  | Print version information                      |

## Quick Start

```bash
# Generate screenshots
delivr generate --config config.yaml --output ./output

# Or use the shorthand (generate is the default)
delivr --config config.yaml
```

## Configuration

The YAML config file defines devices, templates, screens, and outputs.
See `configs/example.yaml` for a basic example.
