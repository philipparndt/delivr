Generate App Store screenshots from a YAML configuration file.

## Usage

```bash
delivr generate --config <config.yaml> [--output <dir>] [--verbose]
```

## Configuration

The YAML config file defines devices, templates, screens, and outputs.

| Section     | Description                                         |
|-------------|-----------------------------------------------------|
| `settings`  | Global settings (fonts_dir, screenshots_dir)        |
| `devices`   | Target device dimensions (e.g., iPhone: 1290x2796)  |
| `templates` | Reusable screen properties (fonts, positions, sizes) |
| `screens`   | Individual screenshot definitions                    |
| `outputs`   | Which screens to render for which devices            |

## Device Modes

| Mode              | Description                                       |
|-------------------|---------------------------------------------------|
| `image`           | Flat screenshot image (default)                   |
| `rotato-cli`      | 3D render via Rotato app (requires macOS + Rotato)|
| `rotato-template` | Composite into pre-exported Rotato frame PNG      |

## Examples

```bash
delivr generate --config configs/example.yaml
delivr generate --config configs/example.yaml --output ./export --verbose
```
