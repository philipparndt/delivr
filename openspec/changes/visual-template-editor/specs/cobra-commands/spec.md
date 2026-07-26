## ADDED Requirements

### Requirement: Edit command registered
The CLI SHALL register an `edit` command alongside `generate`, `capture`, `video` and `deliver`, sharing the same `--config` convention and the same root/standalone config auto-detection.

#### Scenario: Listed in help
- **WHEN** the user runs `delivr --help`
- **THEN** `edit` appears with a one-line description

#### Scenario: Root config auto-detected
- **WHEN** `--config` points at a unified `delivr.yaml` with a `generate:` section
- **THEN** the editor loads it through the same root-config path `generate` uses
