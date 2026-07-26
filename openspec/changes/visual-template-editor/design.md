## Context

`RenderDevice` composites a screenshot into a Rotato frame, optionally crops the
result to its alpha content, scales it, and draws it centred plus an offset:

```go
imgW := img.Bounds().Dx()
x := (float64(dc.Width())-float64(imgW))/2 + deviceCfg.X
dc.DrawImage(img, int(x), int(deviceCfg.Y))
```

Everything confusing about positioning follows from `img` being the device
*plus its shadow*. The editor's job is to show what `img` actually contains and
where the device inside it lands.

### Measured geometry, for `iphone_front` (3840x2160 frame)

| box                              | x          | y         |
|----------------------------------|------------|-----------|
| frame alpha > 8 (the crop box)   | 1501..2615 | 208..2160 |
| screen mask (`mask_path`)        | 1529..2311 | 230..1930 |
| frame alpha > 200 (opaque body)  | 1502..2339 | 209..1951 |

The shadow reaches 304px right and 28px left of the screen mask; the crop box's
bottom is clipped by the frame edge because the shadow runs off it.

## Goals / Non-Goals

**Goals**
- One screen, rendered live by the real pipeline, positioned by dragging.
- The invisible made visible: crop box, body, anchoring mode, margins.
- Solve for values instead of nudging them.
- Sub-500ms per interaction.
- Comment-preserving, opt-in write-back.

**Non-Goals**
- Editing anything other than device x/y/height and title/subtitle text/geometry.
  Backgrounds, gradients and translations stay in YAML.
- Rendering all 28 outputs, or previewing more than two screens at once.
- Changing render behaviour. The editor explains the quirks; it does not fix them.
- Remote access, auth, or multi-user editing.

## Decisions

### 1. The geometry model is pure, and is the single source for overlays *and* solvers

`internal/geometry` takes measured native-pixel boxes (`Frame`) plus a
`config.DeviceImage` and a canvas size, and returns a `Placement`: the total
scale, the drawn image rect, the body rect, and the four margins — all in canvas
coordinates. Overlays are drawn from a `Placement`; solvers invert the same
model. They cannot disagree, because there is only one model.

The model reproduces the pipeline exactly, including the fact that `auto_crop`
scales the frame **twice** (`RenderWithFrame` scales to the target, then
`CropToContent`, then `scaleImageToFit` scales again). Composing the two stages
gives a total scale of `height / contentHeightNative`, which the tests pin
against the numbers in the shipped neon-snare config:

- `height: 2822` → total scale 2822/1952 = 1.4457 → centring x = 1.4457 × 138.5
  = **199.5**, and the config says `x: 200`.
- The same solve at `height: 2400` gives **x ≈ 170**, the value the config
  comment records as measured by hand.

### 2. The "body" comes from the frame mask, with a high-alpha fallback

`mask_path` is opaque exactly over the screen area, which is what the frame JSON
already describes and what the shipped config's own measurements used. Its
bounding box is within a pixel of the opaque-body box (alpha > 200) on every
frame in the sample, because the bezel is symmetric. When there is no mask —
flat images with no `template:` — the body falls back to the alpha > 200 box,
and the UI labels which source it used, because on a device with a chin those
two are not the same thing and the user should know which one they are centring.

### 3. The preview is the real renderer, made fast by caching, not by shortcuts

Measured on the neon-snare iPhone screens: compositing into the frame 288ms,
`CropToContent` 188ms, the second resize 22ms, canvas plus draw 73ms, downscale
to preview size 8ms. Roughly 600ms cold, which misses the target.

The fix is a cache keyed on everything that feeds the *device image* — source,
template, width, height, crop threshold, crop padding — and explicitly not on
x/y. Dragging therefore costs a canvas fill, a `DrawImage` and text: ~100ms.
Only a height change pays the full ~500ms, and the slider debounces.

Rejected: rendering at a fraction of canvas size. It is faster, but alpha
thresholds and font metrics do not scale linearly, so the preview would disagree
with the output — the exact failure this tool exists to prevent.

To make that cache possible without a second copy of the pipeline,
`internal/render` gains `PrepareDeviceImage` (load, composite, crop, scale),
`DrawDeviceImage` (the centring formula and the draw) and `PaintScreen`
(background, devices, title, subtitle in order, with an injectable device-image
loader). `renderScreen` is rewritten in terms of them and does exactly what it
did before.

### 4. Overlays are DOM, not pixels

The endpoint returns the rendered PNG plus a JSON description of every box in
canvas coordinates. The browser scales them into an SVG layer over the image.
Toggling an overlay costs nothing, the lines stay crisp at any zoom, and the PNG
stays a faithful copy of what `generate` would write.

### 5. Text bounds come from `text.go`'s own measuring code

`textBounds` already exists — the backdrop uses it — and already encodes the
`MaxWidth > 0` anchoring split. It is exported as `MeasureText`, which returns
the box, the anchor mode (`top` for wrapped, `middle` for unwrapped), the
wrapped lines, and whether the box escapes the canvas. The overlay labels the
anchor mode, so quirk 3 reads as a visible property of the block rather than as
a surprise.

### 6. Write-back: yes, but explicitly targeted, and as a token edit

The alternative — emit values, let the user paste — was seriously considered,
because these configs carry comments that are the only record of *why* a number
is what it is ("x is 170 and NOT 0, because…"), and because of two ambiguities
the tool genuinely cannot resolve on its own:

- **A dragged value has two plausible homes.** `iphone-03-blackhole` gets its
  device geometry from the `iphone-front` template. Writing there also moves
  screens 04 through 07.
- **`x` and `y` are additive in `mergeDeviceImage`.** A screen-level `x` is a
  delta on the template's, not a replacement, so "write 170" means different
  things in the two files.

Neither is a reason to refuse to write; both are reasons not to write
*silently*. So:

- The UI shows, for every editable value, where it resolved from — file, line,
  and whether it is a template value or a screen delta.
- Applying is a separate, explicit action, and offers the target: the template
  (labelled with how many screens that moves) or a screen-level override.
- The write is surgical. The scalar's `Line`/`Column` from `yaml.Node` locate
  the token, and only that token's text is replaced; the rest of the file comes
  out byte-identical. Marshalling a `yaml.Node` back out would preserve comments
  but not blank lines or quoting style, and would produce an unreadable diff on
  files that are mostly prose.
- Inserting a key that does not exist yet is supported only into a mapping that
  does, at that mapping's indentation. Anything more structural refuses and
  tells the user to paste.
- `--read-only` disables the apply endpoint entirely.

Provenance needs the source file per node, which `LoadWithIncludes` currently
discards when it splices included documents together. `LoadNodeWithIncludesFrom`
returns the same tree plus a `map[*yaml.Node]string` of node to source file.

### 7. The seam gap is a first-class input

Two screens are rendered side by side with a gap that defaults to 4% of the
screenshot width (53px at 1320 wide — the number the neon-snare config arrived
at by hand). A device that appears in both screens gets a readout of its
continuation error, and `SeamMatchX` solves for the partner x:
`xRight = xLeft - (canvasWidth + gap)`. That gap term is what makes a
hand-aligned seam jump by ~53px, so it is shown as a number and not left implicit.

## Risks / Trade-offs

- **The crop box is measured on the native frame and scaled, not re-measured on
  the resampled image.** Lanczos ringing can move an alpha-threshold edge by a
  pixel, so the modelled box can be ±1px from the one `CropToContent` finds.
  That is far below what anyone is positioning to, and a test renders a
  synthetic frame through the real pipeline and asserts the model predicts where
  the body actually lands, so the error stays bounded rather than assumed.
- **Write-back can still move six screens at once**, if the user picks the
  template target. That is a legitimate thing to want; the mitigation is that
  the count of affected screens is on the button.
- **The server has no auth.** It binds to `127.0.0.1` and holds a per-run token
  that the page is served with; requests without it are refused, so another
  local process cannot drive it blind.

## Migration Plan

Purely additive. No config changes, no output changes, nothing to migrate.

## Open Questions

None blocking.
