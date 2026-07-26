## ADDED Requirements

### Requirement: Edit help document
The repository SHALL contain `cmd/delivr/cmd/help/edit.md`, embedded and returned by `GetHelp("edit")`, following the style of the existing command help documents. It SHALL document the overlays and explain the three render behaviours the editor exists to make visible.

#### Scenario: Rendered help
- **WHEN** the user runs `delivr edit --help`
- **THEN** the rendered markdown describes usage, the overlays, the solved actions, the seam preview and write-back
