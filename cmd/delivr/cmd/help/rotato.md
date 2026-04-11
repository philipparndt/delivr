Batch process screenshot images through a Rotato 3D scene,
exporting each as a 3D mockup.

Use this to prepare 3D device images before composing final
App Store screenshots.

## Usage

```bash
delivr rotato --template <file.rotato> --images <dir|files> --output <dir> [--frames <dir>]
```

## Workflow

1. Create your 3D scene in Rotato and save as `.rotato` file
2. Run this command to batch-process all screenshots through that scene
3. Use the output 3D images in your delivr config (`mode: image`)

## Fast Path

If you have pre-rendered frames (from `delivr rotato frame`), pass `--frames`
to skip Rotato UI automation entirely and composite in-process.

## Examples

```bash
# Process all PNGs in a directory
delivr rotato --template scene.rotato --images ./screenshots --output ./3d

# Process specific files
delivr rotato --template scene.rotato --images "screen1.png,screen2.png" --output ./3d

# Use pre-rendered frames (fast)
delivr rotato --template scene.rotato --images ./screenshots --output ./3d --frames ./frames
```
