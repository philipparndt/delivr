# delivr

A lightweight App Store screenshot generator and delivery tool — a fastlane alternative for screenshot automation.

## Features

- Generate professional App Store screenshots from YAML configuration
- Multi-device support (iPhone, iPad) with configurable dimensions
- Gradient and solid color backgrounds with text overlays
- 3D device mockups via [Rotato](https://rotato.app) integration (macOS)
- Upload screenshots and metadata to App Store Connect
- Multi-language/localization support
- Parallel rendering for fast iteration
- Shell completion (bash, zsh, fish, powershell)

## Installation

### Homebrew

```bash
brew install philipparndt/delivr/delivr
```

### Binary Download

Download the latest release from [GitHub Releases](https://github.com/philipparndt/delivr/releases).

### Build from Source

```bash
git clone https://github.com/philipparndt/delivr.git
cd delivr
make build
```

## Usage

### Generate Screenshots

```bash
delivr generate --config config.yaml --output ./output --verbose

# Shorthand (generate is the default command)
delivr --config config.yaml
```

### Generate Device Templates from Rotato

Generate device template sets (frame + mask + metadata) from `.rotato` files.
This is a one-time setup step — templates are then referenced in your screenshot config.

```bash
delivr rotato --input ./rotato-files --output ./frames

# Custom placeholder dimensions (e.g., for iPad)
delivr rotato --input ./rotato-files --output ./frames --dim 2064x2752
```

### Deliver to App Store Connect

```bash
delivr deliver --config appstore.yaml --screenshots ./output
```

### List Display Types

```bash
delivr deliver list-display-types --config appstore.yaml
```

### Shell Completion

```bash
# Bash
source <(delivr completion bash)

# Zsh
delivr completion zsh > "${fpath[1]}/_delivr"

# Fish
delivr completion fish | source
```

### Version

```bash
delivr version
```

## Configuration

Screenshots are defined in YAML config files. See `configs/` for examples:

- `configs/example.yaml` — Basic flat screenshots
- `configs/example-rotato.yaml` — Device templates with 3D Rotato mockups

## License

Apache-2.0
