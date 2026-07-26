## ADDED Requirements

### Requirement: The edit command serves a local editor
The CLI SHALL provide `delivr edit --config <config.yaml>` which starts an HTTP server bound to the loopback interface and prints its URL. The command SHALL accept `--port` (0 for an ephemeral port), `--open` and `--read-only`.

#### Scenario: Starting the editor
- **WHEN** the user runs `delivr edit --config delivr.yaml`
- **THEN** a server listens on `127.0.0.1`, the URL is printed, and the page loads a screen from the config

#### Scenario: Not reachable off the machine
- **WHEN** the server starts
- **THEN** it binds only to the loopback address, and requests that do not carry the session token issued at start-up are refused

### Requirement: Live preview by the real render pipeline
The editor SHALL render the selected screen with the same code path `delivr generate` uses, at the configured canvas size, and SHALL display it scaled to fit the window.

#### Scenario: Preview matches output
- **WHEN** a screen is previewed with unmodified values
- **THEN** the rendered pixels are those `generate` would write for that screen, device and language

#### Scenario: Interactive edits are fast
- **WHEN** the device is dragged
- **THEN** the re-render completes in well under 500ms, because the prepared device image is cached across changes that do not affect it

### Requirement: Screen, device and language selection
The editor SHALL let the user choose any screen in the config, any device output slot it is rendered for, any device within a screen that declares several, and any configured language.

#### Scenario: Multiple devices on one screen
- **WHEN** the selected screen declares a `devices:` array
- **THEN** each entry is individually selectable and the selected one is the one that drags

#### Scenario: Language switch
- **WHEN** a language is selected
- **THEN** the preview applies that language's translations and font overrides

### Requirement: Direct manipulation
The editor SHALL set the selected device's `x` and `y` by dragging it in the preview, and its `height` by a slider or by scrolling over the preview.

#### Scenario: Dragging
- **WHEN** the user drags the device by 100 preview pixels
- **THEN** `x` and `y` change by the equivalent number of canvas pixels and the preview re-renders

### Requirement: Geometry overlays
The editor SHALL draw, over the preview, each of the following as an independently toggleable overlay: the `auto_crop` content box and the device body silhouette drawn distinctly from one another; the canvas centre line and the body's own centre line; the measured bounds of each text block, labelled with its anchoring mode; and the margin in canvas pixels from each body edge to the corresponding canvas edge.

#### Scenario: Shadow asymmetry is visible
- **WHEN** an `auto_crop` device whose frame has an asymmetric shadow is shown with the crop and body overlays on
- **THEN** the crop box is visibly wider on the shadow side than the body, and the body centre line is visibly offset from the canvas centre line

#### Scenario: Anchoring mode is visible
- **WHEN** a text block has `max_width` set
- **THEN** its overlay is labelled as top-anchored, and a block without `max_width` is labelled as centred on its y

### Requirement: Solved actions
The editor SHALL offer actions that solve for values rather than nudging them: centring the body horizontally, bleeding the body past the bottom edge by a given amount, and fitting the body between a given top edge and a given bleed.

#### Scenario: Centre it
- **WHEN** the user sets `x` to 0 on a shadow-asymmetric device and then invokes centring
- **THEN** `x` becomes the value that puts the body's centre on the canvas centre, and the body and canvas centre lines coincide

### Requirement: Seam preview
The editor SHALL render two adjacent screens side by side separated by a configurable gap defaulting to 4% of the screenshot width, and SHALL report, for a device appearing in both, whether it continues across the boundary.

#### Scenario: Default neighbour follows the store order
- **WHEN** seam view is turned on
- **THEN** the neighbour defaults to the next screen in the selected output's screen list, or the previous one when the selected screen is last, and the side is set to match

#### Scenario: Manual choice is kept
- **WHEN** the user picks a different neighbour or side
- **THEN** that choice survives changing the selected screen until the user asks for the automatic pick again

#### Scenario: Checking a device that spans the seam
- **WHEN** two screens that share a device are shown in seam view at the store gap
- **THEN** the device's horizontal continuation error, its `y` difference and its `height` difference are each reported, and it is called continuous only when all three are zero

#### Scenario: Aligning across the seam
- **WHEN** the user invokes the fix offered for a mismatched `x`, `y` or `height`
- **THEN** that value is set to the one that matches the neighbour

### Requirement: Undo and redo
The editor SHALL maintain an undo history of pending edits, reachable by button and by the platform undo and redo shortcuts. History steps SHALL be grouped by interaction, so that a single drag, scroll or burst of typing is one step.

#### Scenario: Undoing a drag
- **WHEN** the user drags a device across the canvas and then undoes
- **THEN** the device returns to where it was before that drag, in one step rather than one step per mouse movement

#### Scenario: Redo after undo
- **WHEN** the user undoes and then redoes
- **THEN** the edit is reapplied, and making a new edit after undoing discards the redo branch

#### Scenario: History is about edits, not files
- **WHEN** pending edits are saved
- **THEN** the history starts again from the saved state, because the values are now part of the config rather than pending changes

### Requirement: Live text editing
The editor SHALL let the user edit the title and subtitle text and see the result reflowed immediately, including where the text wraps and whether it overflows the canvas.

#### Scenario: Overflow while typing
- **WHEN** a title is typed that is too long for its `max_width`
- **THEN** the wrapped line breaks and the resulting box are shown as they will render, and an overflow past the canvas is flagged

### Requirement: YAML output
The editor SHALL display the changed fields as YAML, showing only what differs from the loaded config, and SHALL show for each value the file and line it resolved from.

#### Scenario: Copying values out
- **WHEN** the user has changed `x`, `y` and `height`
- **THEN** the YAML panel shows exactly those three keys under the right path, ready to paste

### Requirement: Explicit save
The editor SHALL hold every edit in memory until the user explicitly saves, by button or by the platform save shortcut. Saving SHALL apply each changed value in sequence, so that each write is resolved against the config as the previous write left it.

#### Scenario: Nothing is written implicitly
- **WHEN** the user drags, solves and types without saving
- **THEN** no configuration file is modified

#### Scenario: Saving several values at once
- **WHEN** several fields have changed and the user saves
- **THEN** each is written to its chosen file and the resulting config resolves to exactly the values that were on screen

#### Scenario: Reporting a partial failure
- **WHEN** one value in a save cannot be written
- **THEN** the others are still written, and the failure is reported against the field it belongs to

### Requirement: Optional comment-preserving write-back
The editor SHALL be able to write a changed value back to the file it resolved from, naming the target and, for a template value, the number of screens it affects. Each value's target SHALL default to the file it currently resolves from. The write SHALL leave the file byte-identical apart from the edited scalar. The editor SHALL refuse to write when started with `--read-only`.

#### Scenario: Comments survive
- **WHEN** a value in a file whose surrounding comments explain the value is written back
- **THEN** the comments, blank lines, quoting and key order are unchanged and only the number differs

#### Scenario: Choosing the target
- **WHEN** the changed value resolved from a template shared by several screens
- **THEN** the user is offered both writing to the template, labelled with the number of screens affected, and writing a screen-level override, and no write happens until one is chosen

#### Scenario: Structural change refused
- **WHEN** applying the value would require creating a mapping that does not exist
- **THEN** the write is refused with a message explaining what to paste instead

### Requirement: Template visibility and editing
The editor SHALL show which template a screen inherits from and which other screens share it. Inherited geometry and type SHALL be editable through the screen, with the template offered as a save destination.

#### Scenario: Blast radius is visible before the edit
- **WHEN** a screen that inherits from a shared template is selected
- **THEN** the template's name and the other screens using it are shown

#### Scenario: Editing an inherited value
- **WHEN** an inherited value is changed and saved to the template
- **THEN** every screen using that template resolves to the new value

### Requirement: Editable copy properties
The editor SHALL allow the title's and subtitle's text, size, `y` and `max_width` to be changed, and SHALL be able to write each back.

#### Scenario: Localised copy goes to the translation file
- **WHEN** a language is selected and the text is changed
- **THEN** the translation entry for that language and screen is offered as the destination, and writing the string to the screen or the template is refused because it would change the base text or every language at once

### Requirement: Adding a device to a screen
The editor SHALL be able to add a device to the screen being edited by copying one that exists elsewhere in the project, and SHALL render it immediately. When the source screen is adjacent in the store order, the copy SHALL be placed at the seam-continuation `x`.

#### Scenario: Building an overlapping pair
- **WHEN** a device is added from the neighbouring screen
- **THEN** it renders on top of the existing devices at the x that continues it across the seam, and the existing devices keep their indices

#### Scenario: A new device is not written silently
- **WHEN** a device has been added and the user saves
- **THEN** the addition is not written back, the user is told so, and the complete YAML entry is offered to paste — because creating the block, or turning a single `device:` into a `devices:` list, is a structural rewrite

### Requirement: Interactive latency
Interactions that do not change the source or the device size SHALL re-render in well under 500ms, by caching each stage of the pipeline against only the inputs it depends on. Caches SHALL be bounded.

#### Scenario: Dragging
- **WHEN** the device is dragged
- **THEN** neither the frame composite nor the background gradient is recomputed

#### Scenario: Changing the height
- **WHEN** the height changes
- **THEN** the frame composite is reused and only the sizing and cropping are redone
