## Context

Currently the rendering pipeline supports 4 device modes: `image` (flat PNG), `rotato-cli` (live macOS Rotato automation), `rotato-template` (pre-rendered frame compositing), and `rotato-image` (pre-rendered Rotato output). The `rotato-template` mode already uses the target architecture — frame PNG + mask PNG + JSON metadata — and is the fastest path. The goal is to make this the only path, and move all Rotato-specific work into a separate one-time generation command.

The existing frame detection, perspective warp, and mask compositing code (`internal/rotato/frame.go`, `edges.go`, `warp.go`) is solid and stays. The AppleScript UI automation (`rotato.go` RenderWithCLI) stays but is only used by the `delivr rotato` command, never by the renderer.

## Goals / Non-Goals

**Goals:**
- Single device rendering path: load template JSON → load frame + mask PNGs → composite screenshot via perspective warp or rectangle blit
- `delivr rotato` command: takes a directory of `.rotato` files, generates template sets (frame + mask + JSON) for each
- Simplified config: devices reference a template JSON path, no more `mode` field
- Support for flat device frames (Apple bezels, custom PNGs) via the same template format — user manually creates frame + mask + JSON, or we provide tooling later
- Config fields like `auto_crop`, positioning (`x`, `y`, `width`, `height`) continue to work on the composited result

**Non-Goals:**
- Building a tool to generate templates from flat Apple device images (future work — for now users can manually create template JSON)
- Changing the frame metadata format (the existing JSON format is sufficient)
- Modifying the perspective warp or edge detection algorithms

## Decisions

### 1. Remove device mode enum, use template path

Currently: `device.mode` selects between 4 rendering paths.
New: `device.template` points to a `.frame.json` file. If omitted, screenshot is rendered flat (no device frame) — this replaces the old `image` mode for cases where no device bezel is needed.

Alternative: keep `mode` with just two values (`template` and `flat`). Rejected — the presence/absence of a `template` field is sufficient and simpler.

### 2. `delivr rotato` reworked to generate templates

Current `delivr rotato` processes screenshots through Rotato (batch). Current `delivr rotato frame` pre-renders one template.

New: `delivr rotato` takes a directory (or list) of `.rotato` files and generates template sets for each. Essentially it does what `rotato frame` does today, but for multiple files at once. The old batch-screenshot-through-Rotato workflow is removed — users now generate templates once, then use `delivr generate` for screenshots.

```bash
delivr rotato --input <dir-with-rotato-files> --output <templates-dir> [--dim WxH] [--verbose]
```

### 3. Keep RenderWithCLI in the rotato package

The AppleScript automation code stays in `internal/rotato/` — the `delivr rotato` command needs it to render the magenta placeholder through each .rotato file. It's just no longer called from the rendering pipeline.

### 4. Renderer uses RenderWithFrame exclusively

`internal/render/device.go` simplifies to: load template JSON → call `rotato.RenderWithFrame()` → apply auto-crop/scaling/positioning. The function `RenderWithFrame` already handles both rectangular and perspective templates.

### 5. Config migration

Old config:
```yaml
device:
  mode: "rotato-template"
  source: "{device}-Tree View.png"
  template: "frames/iphone-front.frame.json"
  template_rect: [185, 520, 720, 1560]
  height: 2000
```

New config:
```yaml
device:
  source: "{device}-Tree View.png"
  template: "frames/iphone-front.frame.json"
  height: 2000
```

The `template_rect` is no longer needed in config — it's stored in the frame JSON metadata. The `mode` and `rotato_file` fields are removed.

## Risks / Trade-offs

- **Breaking config change** → Users must update their YAML configs. Acceptable since this is pre-release.
- **Two-step workflow** → Users must run `delivr rotato` before `delivr generate`. Mitigated by Makefile targets that chain the steps. This is actually better — template generation is a one-time cost.
- **Flat device frames require manual JSON** → Until a tool for importing Apple device bezels is built, users manually create the JSON for flat frames. This is acceptable as a starting point.
