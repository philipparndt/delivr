## Why

Positioning a device in a delivr layout means editing YAML, running `delivr
generate` (28 images, ~40s), opening the PNGs, and guessing at the difference.
The loop is slow enough that people tune by eye, and three render behaviours
are invisible in the output until someone measures them:

1. **`auto_crop` crops to the device *plus its drop shadow*, which is wildly
   asymmetric.** For `iphone_front` the alpha content box is x=1501..2615 while
   the phone body is x=1529..2311 — 304px of shadow to the right, 28px to the
   left. `device.go` centres the *cropped image*, so `x: 0` parks the phone
   ~170px left of centre. A shipped project ran for weeks with the phone flush
   against the left edge and 350px of dead background on the right.
2. **`height` sizes the cropped image, not the device**, so `y + height` is not
   where the device bottom lands.
3. **`max_width` on a text block silently moves it vertically.** `text.go` uses
   `DrawStringWrapped` (top-anchored) when `MaxWidth > 0` and
   `DrawStringAnchored` (centred on Y) when it is 0, so adding `max_width` to
   stop a title overflowing drops it by half its height onto the subtitle.

Every number needed to get these right is already on disk — the body silhouette
in the frame's `mask_path`, the crop box in the frame PNG's alpha — and nothing
surfaces any of it.

## What Changes

- Add `delivr edit --config delivr.yaml`: a localhost-only page that renders one
  screen live with the real render pipeline and lets you position it by dragging.
- Add `internal/geometry`: a pure model of where a device *body* lands on the
  canvas given a config, and solvers that invert it ("centre it", "bleed to
  bottom", "fit body", "match across the seam").
- Overlays over the live render: crop box vs body silhouette, canvas and body
  centre lines, text bounds annotated with their anchoring mode, and body-to-edge
  margins in px.
- Seam preview: two adjacent screens side by side with a configurable gap,
  defaulting to 4% of screenshot width.
- Live title/subtitle editing with immediate reflow.
- Changed fields are emitted as YAML, and can optionally be written back to the
  file they actually came from, as a surgical single-token edit that leaves the
  rest of the file byte-identical.
- Behaviour-preserving extractions in `internal/render` so the editor calls the
  same code `generate` does rather than a second implementation.

Existing render behaviour is unchanged. The three quirks above are load-bearing
for shipped configs; this change makes them **visible**, it does not fix them.

## Capabilities

### New Capabilities
- `visual-editor`: `delivr edit` serves a live positioning UI for one screen.
- `device-geometry`: a measured, pure model of device placement, with solvers.

### Modified Capabilities
- `cobra-commands`: adds the `edit` command.
- `markdown-help`: adds `help/edit.md`.

## Impact

- **Code**: new `internal/geometry`, new `internal/editor`, new
  `cmd/delivr/cmd/edit.go`, new `cmd/delivr/cmd/help/edit.md`. Refactors in
  `internal/render/{renderer,device,text}.go` that move code without changing
  what it does.
- **Dependencies**: none. `net/http` and `embed` from the standard library; the
  page is one embedded HTML file with inline JS and no build step.
- **Network**: binds to `127.0.0.1` on an ephemeral port by default.
- **Risk**: write-back touches the user's source files. Mitigated by making it
  opt-in per apply, by editing the exact scalar token rather than re-marshalling,
  and by `--read-only`.
