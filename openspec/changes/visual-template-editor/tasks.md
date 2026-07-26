## 1. Geometry model

- [x] 1.1 Create `internal/geometry/frame.go`: `Frame` (native size, content box, body box, body source, screen quad) and `Measure*` helpers that read a frame JSON or a plain image
- [x] 1.2 Create `internal/geometry/place.go`: the pure `Place()` model reproducing the crop/scale/centre pipeline, including the double scale in the `auto_crop` path
- [x] 1.3 Create `internal/geometry/solve.go`: `CenterX`, `BleedBottom`, `FitBody`, `SeamMatchX`
- [x] 1.4 Create `internal/geometry/place_test.go` and `solve_test.go`: shadow asymmetry, `height` ≠ body bottom, the shipped neon-snare numbers (170 at 2400, 200 at 2822, 2822/438 from top 470 + bleed 60), the seam value -1025
- [x] 1.5 Add a test that renders a synthetic asymmetric frame through the real `render` pipeline and asserts the model predicts where the body lands

## 2. Render extractions (behaviour-preserving)

- [x] 2.1 Split `RenderDevice` into `PrepareDeviceImage` (load/composite/crop/scale) and `DrawDeviceImage` (centre + draw); keep `RenderDevice` as the composition of the two
- [x] 2.2 Add `PaintScreen(dc, screen, fonts, loadDevice)` and rewrite `renderScreen` in terms of it
- [x] 2.3 Export text measuring as `MeasureText` returning box, anchor mode, wrapped lines and overflow, reusing the existing `textBounds`
- [x] 2.4 Export `Renderer.ScreenForLang` so the editor applies translations the same way
- [x] 2.5 Confirm `delivr generate` output is byte-identical before and after

## 3. Config provenance

- [x] 3.1 Add `config.LoadNodeWithIncludesFrom` returning the resolved node tree plus a node→source-file map
- [x] 3.2 Add a test that a value from an included file reports that file, not the includer

## 4. Editor server

- [x] 4.1 `internal/editor/server.go`: loopback listener, session token, routes, graceful shutdown
- [x] 4.2 `internal/editor/state.go`: enumerate screens/outputs/devices/languages, resolve a screen with overrides applied
- [x] 4.3 `internal/editor/render.go`: preview rendering with the device-image cache, canvas→preview downscale, seam composition
- [x] 4.4 `internal/editor/geometry.go`: build the overlay payload (placements, text metrics, margins) from the geometry model
- [x] 4.5 `internal/editor/page.html`: single embedded page — picker, drag, height control, overlay toggles, seam view, text fields, YAML panel, apply buttons
- [x] 4.6 Confirm the drag path stays under 500ms on the neon-snare iPhone screens

## 5. Write-back

- [x] 5.1 `internal/editor/yamledit.go`: `SetScalar` (replace the token at a node's line/column), `InsertKey` (append into an existing mapping at its indentation), both leaving everything else byte-identical
- [x] 5.2 Resolve an edit to a target: template value vs screen override, with the affected-screen count
- [x] 5.3 `/api/apply` honouring `--read-only`, re-parsing after the write to prove the value took
- [x] 5.4 Tests: comments and blank lines survive, additive `x`/`y` deltas are computed correctly, structural inserts are refused

## 6. CLI and docs

- [x] 6.1 `cmd/delivr/cmd/edit.go` with `--config`, `--port`, `--open`, `--read-only`
- [x] 6.2 `cmd/delivr/cmd/help/edit.md`
- [x] 6.3 Embed and case in `cmd/delivr/cmd/help.go`
- [x] 6.4 README section

## 7. Verification against a real project

- [x] 7.1 Point at `~/dev/neon-snare-marketing`; set `iphone-front` device `x` to 0 and confirm the overlay makes the miscentring obvious
- [x] 7.2 Confirm "centre it" recovers x ≈ 170 at height 2400
- [x] 7.3 Check the seam preview on `iphone-01-title` / `iphone-02-loop`. It reports them as **not** continuous at their committed values, and it is right: the shared phone renders at x 519 / y 712 on screen 1 and x -1025 / y 298 on screen 2, so it is 171px too far left and 414px too high. The config's own arithmetic (`348 - 1373 = -1025`) used screen 1's *raw* x, but `mergeDeviceImage` adds the template's `x: 171` and `y: 414` on top — while screen 2's `devices:` array bypasses the template merge entirely. Applying the solved x -854 and y 712 makes it continuous. Reported to the user; the config is theirs to change.
- [x] 7.4 `go build ./...`, `go vet ./...`, `go test ./...`

## 8. Follow-ups from review

- [x] 8.1 Seam neighbour defaults to the adjacent screen in the output's store order, with a manual override and an `auto` reset
- [x] 8.2 Seam checks `y` and `height` as well as `x`, since a device spanning the boundary must agree on all three
- [x] 8.3 Undo/redo over the pending-edit buffer, coalesced per gesture, with ⌘Z / ⇧⌘Z
- [x] 8.4 Explicit Save (⌘S) writing all pending changes in sequence, each to a chosen destination
- [x] 8.5 Cache the background gradient (162ms/frame) and bound every cache; drag 285ms → ~160ms, height 1000ms → ~240ms
- [x] 8.6 Template panel naming the template and the screens it drives
- [x] 8.7 Editable and saveable copy properties, with localised text routed to the translation file
- [x] 8.8 Add a device by copying an existing one, seam-placed; emitted as YAML rather than written structurally

## 9. Second round of follow-ups

- [x] 9.1 Serve the preview as content-addressed layers (background / devices / text) instead of one flattened PNG; drag becomes a CSS offset with no request. Layers are clipped per screenshot card so an overflowing device cannot appear on the card next door
- [x] 9.2 Test pinning layer position against what `DrawDeviceImage` actually draws, and that cache keys change when and only when the pixels do
- [x] 9.3 Make `x`/`y` nudges and arrow keys local too; debounce the height re-render until the scroll stops
- [x] 9.4 Make the seam's neighbour fully editable — its own edit buffer, selectable and draggable devices, saveable copy
- [x] 9.5 Global ruler: movable, rotatable, Shift-snapping, extending across both cards, persisted
- [x] 9.6 Click a device in the preview to select it, hit-testing the body and respecting each card's clipping — this is what actually made the seam's other screenshot editable
- [x] 9.7 Ruler: filled, screen-sized hit targets (the centre was an unfilled ring, clickable only on its 2px stroke); the whole bar moves it, the ends aim it

## 10. Third round of follow-ups

- [x] 10.1 **Bug: saving moved a device.** `x`/`y` are additive only on the single-`device:` path; a screen with a `devices:` list never reaches mergeDeviceImage, so its values are literal. Subtracting the template's anyway wrote a number short by exactly the template's y, and the device jumped. Fixed via `Project.DeviceForm`, which also blocks writes to files the merge would ignore. Regression tests included, verified to fail against the old code
- [x] 10.2 Render the whole store row rather than one screen (or two), built in parallel; every screen selectable and editable; seam checks on every boundary
- [x] 10.3 Horizontal scroll, zoom slider and fit; click a screenshot or device to select it
- [x] 10.4 Ruler becomes a jointed polyline: segments chain and Shift snaps each relative to its predecessor, with ∥ and ⊥ buttons
- [x] 10.5 Overlay labels and strokes sized in screen pixels, not frame pixels — with a seven-canvas row they had grown 7× and swamped the preview
- [x] 10.6 Reserve the caption strip in the fit scale, so screenshot names are not one strip below the fold; clamp overlay labels that annotate boxes running past the card
- [x] 10.7 The wheel scrolls the row instead of resizing the selected device; ⌘/Ctrl-wheel zooms about the pointer, applied as a local transform; `height` moves to a slider
- [x] 10.8 Output and language selectors move above the screen picker, and the screen picker lists only the screens that output renders, in store order
- [x] 10.9 Drag a screenshot's name to reorder the row; pending like any other edit, undoable, written on Save to the output's `screens:` list (block and flow forms), carrying each entry's comments with it
- [x] 10.10 The reorder handle is the whole caption strip under a card, not the name's glyphs — an SVG `<text>` is hit-tested on the letterforms, so grabbing it meant hitting a few-pixel stroke and missing dragged the device instead (9x the hit area)
- [x] 10.11 Progress panel while a row is built. Switching output or language clears the stale screenshots and shows it immediately with an elapsed timer (cold row 2.7s, device switch 1.9s); same-row re-renders wait 200ms first so a ~150ms drag resync never flashes
