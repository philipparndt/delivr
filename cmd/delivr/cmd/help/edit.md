# delivr edit

Position devices and copy against a live preview, instead of editing YAML,
running `generate`, and squinting at 28 PNGs.

The preview is the real renderer — the same code `delivr generate` writes files
with — so what you see is what ships.

## Usage

    delivr edit --config delivr.yaml
    delivr edit --config delivr.yaml --port 8731
    delivr edit --config delivr.yaml --read-only   # measure, never write

The server binds to `127.0.0.1` only, on a free port unless `--port` says
otherwise, and prints a URL carrying a token that the API requires. Ctrl-C
stops it.

| Flag          | Default | Description                                     |
|---------------|---------|-------------------------------------------------|
| `--config`    | —       | Config to edit; root or standalone, as `generate` |
| `--port`      | `0`     | Listen port; 0 picks a free one                 |
| `--open`      | `true`  | Open a browser at the URL                       |
| `--read-only` | `false` | Refuse every write; the YAML can still be copied |

## What it is for

Three render behaviours are load-bearing for existing configs and invisible in
the output. The editor exists to show them.

**`auto_crop` crops to the device *and its drop shadow*.** Rotato renders that
shadow offset to one side — on `iphone_front` it reaches 304px right and 28px
left — and `device.go` centres the cropped image. So **`x: 0` is not centred**:
it parks the phone about 170px left of centre and leaves a wall of background
on the other side. Turn on the **auto_crop box** and **device body** overlays
and the two rectangles do not share a centre, which is the whole story.

**`height` sizes the cropped image, not the device.** `y + height` is not where
the device bottom lands; it can be a couple of hundred pixels out while the
numbers look right. The panel prints both, next to each other.

**`max_width` moves a text block vertically.** `text.go` top-anchors a block
that can wrap and centres one that cannot, so adding a `max_width` to stop a
title overflowing also drops it by half its height — often onto the subtitle.
The text overlay labels which mode each block is in and draws the line where
`y` actually falls.

None of these are fixed here. Shipped configs depend on them.

## Overlays

Each toggles independently, and is drawn from measurements of the real render
rather than from the preview image.

| Overlay        | Shows                                                       |
|----------------|-------------------------------------------------------------|
| auto_crop box  | what `auto_crop` trims to — device plus shadow              |
| device body    | the device itself, from the frame's `mask_path`             |
| centre lines   | the canvas centre and the body's own, with the gap in px    |
| margins        | body edge to canvas edge, each side, in px                  |
| text bounds    | measured box, wrap points, and the anchoring mode           |
| screen quad    | the screen's four corners, showing a rotated frame's tilt    |

The body comes from the frame mask when there is one and from the opaque alpha
when there is not; the panel says which, because on a device with a chin those
are not the same box.

## Positioning

**Click a device to select it**, then drag to set `x` and `y`. Selection
hit-tests the device *body*, not its image — the image includes a drop shadow
reaching hundreds of pixels past anything a person would call the device — and
respects each screenshot's clipping, so a device cropped off the edge is only
selectable where it is actually drawn. In seam view this is how you reach the
other screenshot's devices.

`height` is the slider under the x/y fields. Arrow keys nudge the selection,
with Shift for ten.

The **wheel scrolls the row**, as it does everywhere else; **⌘/Ctrl-wheel
zooms**, anchored on the pointer, as in every other canvas tool. Zooming is a
transform on layers the browser already has, so it costs nothing.

Dragging does not wait on a render. The device's pixels do not change when it
moves, so the preview is sent as layers — background, each device, the text —
and a drag offsets one of them in the browser. The server is asked for an
authoritative render once, on release.

**⌘Z / ⇧⌘Z** undo and redo. Steps are grouped by gesture, so one drag is one
undo — not one per pixel the mouse moved. Undo works on pending edits, which
are not the files; saving is the point where the two meet, and it starts the
history again.

Three actions solve rather than nudge:

**Centre it** puts the body's centre on the canvas centre. This is the value
that is 170 rather than 0.

**Bleed to bottom** runs the body a stated number of pixels past the bottom
edge. Stopping short leaves a band of background under the device, and it reads
as dropped onto the card rather than as part of it.

**Fit** solves `height`, `y` and `x` together from a body top and a bleed —
the three numbers that have to move as a set, because `height` changes the
scale and therefore where the top lands.

## Ruler

A jointed straightedge over the whole row, at any angle, spanning every card.

Drag a **joint** to aim the segment ending at it; drag the **bar** or the centre
grip to slide the whole chain, keeping every angle. **＋ segment** extends the
chain, continuing straight on. The selected segment gets a dashed extension
across the entire row, which is what turns "these two look aligned" into an
answer.

Hold **Shift** while aiming and the angle snaps in 15° steps — **relative to the
previous segment**, not to the page. That is the distinction that makes it
useful on tilted art. Rotated device frames put the alignments that matter on a
diagonal: "perpendicular to that phone's edge" is a question about a 17°
reference, and snapping to an absolute 90° cannot answer it. The **∥** and **⊥**
buttons set exactly 0° and 90° from the predecessor.

Each segment reports its own angle, its length, and its turn from the one
before. The chain persists across screens and reloads.

## The row

The whole store row renders at once — every screenshot in the output, in order,
separated by the gap the App Store leaves between them in its carousel: roughly
**4% of a screenshot's width**, 53px on the 6.9-inch slot. It scrolls
horizontally; the wheel, the zoom slider, ⌘-wheel or **fit** set the scale.
Fit leaves room for the screenshot names written under each card.

One screen at a time was the wrong unit. The questions this tool exists to
answer are about the row — is this device centred *relative to its neighbours*,
does that phone continue across the boundary, does the copy sit at a consistent
height down the page — and a viewport showing one card turns all of them into a
memory test.

**Drag the strip under a screenshot to reorder the row** — the whole band, not
just the name written in it. The order is a pending change
like any other — it previews immediately and is written on Save, to the
`screens:` list of the output it belongs to, entry by entry so the notes
written about each one travel with it.

**Every screen is live.** Click any screenshot to select it, or any device on
it to select that. The panel follows the pointer rather than a dropdown
somewhere else. Selection hit-tests the device *body*, not its image — the
image includes a drop shadow reaching hundreds of pixels past anything a person
would call the device — and respects each screenshot's clipping, so a device
cropped off the edge is only selectable where it is actually drawn.

Every boundary with a device spanning it is checked, on all three of `x`, `y`
and `height`, and each mismatch gets a button that fixes it:

    x = xNeighbour - (canvasWidth + gap)

`x` is the one people compute. `y` and `height` are the ones that get missed,
because two entries that read the same in YAML need not resolve the same: `x`
and `y` are **added** to a template's when a screen overrides `device:`, and
ignored entirely when the screen declares a `devices:` array. Two halves of one
phone can therefore be hundreds of pixels apart while both files look right.

Without the gap term the device jumps backwards by ~53px across the boundary —
small enough to look like a rendering artefact, large enough to notice.

## Output and language

The output picks the store slot — iPhone, iPad, Apple TV — and with it the row:
the screen list shows only the screenshots that output actually renders, in
store order. Selecting an Apple TV screen while previewing the iPhone row was
never meaningful, so it is no longer offered.

## Templates

Most geometry and type lives in a template, not on the screen, and the panel
says which one drives the current screen and how many others share it. Editing
any inherited value edits it *through* the screen: you position what you can
see, and choose at save time whether the number belongs to the template — which
moves every screen using it — or to this screen alone.

## Copy

Title and subtitle are editable in place, reflowing immediately, so an overflow
or a single word stranded on the last line shows up while the copy is being
written rather than after a full generate. Size, `y` and `max_width` are
editable alongside the text.

With a language selected, the text itself saves to the **translation** file
rather than to the screen: the screen holds the base string, so writing a
German headline there would change every language at once. The editor refuses
that and offers the translation entry instead.

## Adding a device

**Add** copies a device that already exists elsewhere in the project onto this
screen. That is the shape the overlapping pair needs — the device screen two
must show is the one screen one already has — so the picker lists existing
devices rather than asking for a path, and if the two screens are neighbours in
the store order it places the copy at the seam-continuation `x` straight away.

Added devices render immediately but are **not** written back automatically.
Adding one means creating a `devices:` block, or turning an existing `device:`
into a list — a structural rewrite that would have to guess where the new block
goes among the comments describing the old one. The YAML panel emits the whole
entry to paste instead.

## Saving

Nothing is written until you press **Save** (⌘S). Dragging, solving and typing
all stay in memory, which is what makes undo meaningful and what lets you leave
without having touched anything.

The **Changes** panel lists what has moved, and gives each value a destination
to be written to. Save then applies them one at a time — sequentially, because
each write reloads the config and the next one's line numbers and additive
deltas are computed against the file as it now stands.

The destination is a choice rather than a default, because a dragged value has
two plausible homes and delivr cannot pick between them:

- The geometry usually lives in a **template**, which several screens share.
  Writing there moves all of them — sometimes exactly right, sometimes a
  surprise. The option says how many screens it moves.
- `x` and `y` are **additive**: a screen-level `x` is a delta on the template's,
  not a replacement. Writing 200 to the wrong one of the two files does not
  produce 200. The editor computes the delta and shows it.

Each field defaults to the file it already resolves from, so saving leaves a
value where somebody already chose to put it.

A write replaces the one scalar token in the file and nothing else. Comments,
blank lines, quoting and key order come out byte-identical — which matters,
because in these files the paragraph above a number is frequently the only
record of why it is that number. Adding a key that does not exist yet is
supported inside a mapping that does; anything more structural is refused with
a message, and the YAML panel has the fragment to paste.

`--read-only` disables writing entirely.

## Notes

**Speed.** A cold screen costs about a second: ~350ms compositing a 3840x2160
frame, ~190ms scanning its alpha and rescaling, ~160ms filling the background
gradient, plus encoding. Each is cached against exactly what it depends on — the
composite on the source and frame but not the size, the sized image on the size
but not on `x`/`y`, the background on the background config.

After that, moving anything costs nothing: layers are served by content address
and cached by the browser for good, and `x`/`y` are applied as a CSS offset. A
height change is the only interaction that still renders, at about 250ms, and it
waits for the scroll to stop. Caches are bounded, because a composited frame is
33MB and a filled canvas 15MB.

**Loading.** Switching output or language rebuilds the whole row, which is a
second or two of frame compositing. The previous slot's screenshots are cleared
and a progress panel takes their place with an elapsed timer — leaving the old
device on screen while the new one renders reads as "the click did nothing".
Re-renders of the *same* row show nothing for the first 200ms, so a drag
resync, at ~150ms, never flashes.

**Fidelity.** The layers are the renderer's own pixels, positioned with the same
integer arithmetic `DrawDeviceImage` uses and clipped to each screenshot's card
exactly as drawing onto a canvas of that size clips them. Only the compositing
happens in the browser, and every gesture ends with a real render, so any
difference would last one frame. A test pins the layer position against what the
renderer actually draws, for negative, fractional and off-canvas offsets.

**Accuracy.** Overlays use the crop the renderer actually took, not a predicted
one, so they are exact. The **Fit** solver predicts, because it is asking about
a height that has not been rendered yet; re-running it after the render agrees
to the pixel.

## See also

`delivr generate` renders every screen for real.
