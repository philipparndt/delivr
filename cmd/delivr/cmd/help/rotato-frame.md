Pre-render a Rotato template to a reusable frame.

This runs the Rotato UI automation exactly once using a solid magenta
placeholder. The resulting render is analysed to locate the device screen,
and saved as a frame PNG (magenta replaced with transparency) together with
a JSON metadata file.

Future batch runs detect these files and composite screenshots purely
in-process, skipping the UI automation entirely.

## Usage

```bash
delivr rotato frame --template <file.rotato> --output <dir> [--dim <WxH>]
```

## Example

```bash
delivr rotato frame --template iphone_front.rotato --output frames --dim 1320x2868
```
