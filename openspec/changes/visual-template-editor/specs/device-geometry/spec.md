## ADDED Requirements

### Requirement: Measured frame geometry
The system SHALL measure, for a device image source, the native-pixel content box (alpha bounds at the configured crop threshold and padding) and the device body box, and SHALL record which source the body came from.

#### Scenario: Body from the frame mask
- **WHEN** the device has a `template:` whose frame JSON declares a `mask_path`
- **THEN** the body box is the bounding box of the mask's opaque pixels and the body source is reported as `mask`

#### Scenario: Body without a mask
- **WHEN** the device has no frame template, or the frame declares no mask
- **THEN** the body box is the bounding box of pixels with alpha above the opaque threshold and the body source is reported as `alpha`

### Requirement: Placement model
The system SHALL compute, from a measured frame, a `DeviceImage` config and a canvas size, the on-canvas rectangle of the drawn image, the on-canvas rectangle of the device body, the total scale, and the margin from each body edge to the corresponding canvas edge.

#### Scenario: auto_crop shifts the body off centre
- **WHEN** `auto_crop` is set, the content box is asymmetric around the body, and `x` is 0
- **THEN** the reported body rectangle is not centred on the canvas, by the difference between the content box's centre and the body's centre times the total scale

#### Scenario: height sizes the cropped image
- **WHEN** `auto_crop` is set and `height` is given
- **THEN** the total scale from native pixels is `height / contentHeightNative`, and the body's bottom edge is `y + (bodyBottomNative - contentTopNative) * scale`, which is not `y + height`

#### Scenario: no auto_crop
- **WHEN** `auto_crop` is not set
- **THEN** the drawn image is the whole scaled source and the total scale is derived from the source's own dimensions

### Requirement: Centring solver
The system SHALL solve for the `x` that places the device body's horizontal centre on the canvas's horizontal centre, leaving `height` and `y` unchanged.

#### Scenario: Recovering the measured correction
- **WHEN** centring is solved for an `iphone_front` frame at `height: 2400` on a 1320-wide canvas
- **THEN** the returned `x` is within one pixel of 170

### Requirement: Bleed solver
The system SHALL solve for the `y` that places the device body's bottom edge a given number of pixels past the bottom canvas edge, leaving `x` and `height` unchanged.

#### Scenario: Bleeding off the bottom
- **WHEN** a bleed of 60px is solved for
- **THEN** the resulting placement's body bottom is 60px below the canvas height

### Requirement: Fit solver
The system SHALL solve jointly for the `height` and `y` that place the device body's top edge at a given canvas y and its bottom edge a given number of pixels past the bottom canvas edge.

#### Scenario: Solving the shipped iPhone layout
- **WHEN** a body top of 470 and a bleed of 60 are solved for on a 2868-tall canvas with the `iphone_front` frame
- **THEN** the returned height and y are within one pixel of 2822 and 438

### Requirement: Seam solver
The system SHALL solve for the `x` at which a device continues into the adjacent screenshot, given the neighbouring screenshot's `x`, the canvas width, and the carousel gap.

#### Scenario: Continuing a device rightwards across the seam
- **WHEN** the left screenshot places the device at `x: 348` on a 1320-wide canvas with a 53px gap
- **THEN** the right screenshot's `x` is -1025

#### Scenario: Default gap
- **WHEN** no gap is given
- **THEN** the gap defaults to 4% of the canvas width
