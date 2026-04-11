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

### Rotato 3D Batch Processing

```bash
delivr rotato --template scene.rotato --images ./screenshots --output ./3d-output
```

### Pre-render Rotato Frame (fast path)

```bash
delivr rotato frame --template iphone_front.rotato --output frames --dim 1320x2868
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
- `configs/example-rotato.yaml` — Rotato 3D mockup integration

## License

Apache-2.0
