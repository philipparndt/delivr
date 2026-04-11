## ADDED Requirements

### Requirement: Include directive in YAML configs
The YAML loader SHALL support a `!include <path>` tag that replaces the tagged node with the contents of the referenced file. Paths SHALL be resolved relative to the file containing the `!include` directive.

#### Scenario: Include a mapping
- **WHEN** a YAML file contains `devices: !include devices.yaml`
- **THEN** the `devices` key's value is replaced with the parsed contents of `devices.yaml`

### Requirement: Recursive includes
Included files SHALL be able to contain their own `!include` directives, resolved relative to the included file's location.

#### Scenario: Nested includes
- **WHEN** `delivr.yaml` includes `generate.yaml` which includes `templates.yaml`
- **THEN** all three files are resolved into a single config tree

### Requirement: Sequence flattening
When `!include` is used as an item in a YAML sequence and the included file contains a sequence, the items SHALL be flattened into the parent sequence.

#### Scenario: Multiple screen files flattened
- **WHEN** `screens: [!include screens/iphone.yaml, !include screens/ipad.yaml]` and each file contains a list of screens
- **THEN** the resulting `screens` list contains all screens from both files in order

### Requirement: Cycle detection
The loader SHALL detect circular includes and return an error.

#### Scenario: Circular include
- **WHEN** `a.yaml` includes `b.yaml` which includes `a.yaml`
- **THEN** an error is returned indicating a circular include
