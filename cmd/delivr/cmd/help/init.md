Initialize a new delivr project with example configuration files.

Interactively selects devices and generates a complete project scaffold
including the root config, device definitions, screen templates, example
screens, App Store metadata templates, and Claude Code helper commands.

## Usage

```bash
delivr init
```

## Generated Files

```
delivr.yaml              Root configuration with !include references
devices.yaml             Selected device definitions
templates.yaml           Reusable screen templates (fonts, sizes, positioning)
screens/example.yaml     Example screenshot screen definitions
outputs.yaml             Device → screen output mappings
metadata/en-US/          App Store text templates (description, promo, release notes)
.claude/commands/        AI helper commands for changelog, descriptions, translations
```

## Next Steps

```bash
# 1. Edit delivr.yaml to set your Xcode project path
# 2. Define your screenshot screens in screens/
# 3. Capture screenshots from simulators
delivr capture --config delivr.yaml

# 4. Generate App Store screenshots
delivr generate --config delivr.yaml

# 5. Upload to App Store Connect
delivr deliver --config delivr.yaml
```

## Claude Code Commands

The generated `.claude/commands/` include:
- **changelog** — Generate release notes from git history
- **update-description** — Update App Store description
- **translate** — Translate metadata to configured languages
