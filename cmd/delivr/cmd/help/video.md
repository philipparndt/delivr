# delivr video

Record App Store preview videos off a simulator, then cut them to Apple's spec.

Apple accepts previews of **15 to 30 seconds** at a fixed resolution per device
family, encoded as H.264 or ProRes — and only tells you otherwise *after* an
upload. `delivr video` produces a file that already satisfies those rules.

## Usage

    delivr video --config delivr.yaml
    delivr video --config delivr.yaml --device appletv
    delivr video --config delivr.yaml --skip-record   # re-cut the last capture

## Configuration

```yaml
video:
  output: ./output/previews
  record:
    app: ../myapp/build/MyApp.app
    launch_args: ["--preview"]
    duration: 36        # film longer than the finished cut needs
    warm_up: true       # discard one run first, so shader hitching is not filmed
  audio:
    track: ../myapp/Music/theme.mp3
    offset: 1.0         # start this far into the track, to match the trim
    fade_in: 0.4
    fade_out: 1.5
    volume: 0.9
  devices:
    iphone:
      duration: 28
      auto_trim: true
      poster_frame: 3.0
    appletv:
      duration: 28
      auto_trim: true
      audio:            # replaces video.audio outright for this device
        track: ../myapp/build/preview-appletv.wav
```

Width, height and the simulator name fall back to the matching entry in
`devices`, so sizes are declared once.

## Notes

**`warm_up`** runs the sequence once and throws it away. The first run of a
scene pays for shader compilation and texture caches; filming that means the
hitching is the first thing a viewer sees.

**`auto_trim`** finds the end of the opening black by luminance and cuts from
there. Simulator boot and launch take a variable amount of time, so a fixed
offset drifts between runs — the difference between opening on your title card
and opening on an empty screen.

**`--skip-record`** re-encodes the raw capture kept under `output/.raw/`, which
is the fast way to iterate on trim points without filming again.

**`audio`** lays a soundtrack under the cut. Every preview gets an audio track
whether you configure one or not — App Store Connect rejects a video with no
audio *stream*, and reports it as "unsupported or corrupted audio", which reads
like a codec fault rather than an absent track. Without `audio` that stream is
silence.

`offset` matters more than it looks: `auto_trim` cuts past the app's opening,
so a track that starts at launch has to skip the same distance or the preview
opens mid-phrase.

A device-level `audio` block replaces `video.audio` outright rather than
merging into it. Use it when the audio belongs to what one specific recording
shows — an app whose sounds follow its own content produces a different
soundtrack per device, and one shared track then drifts against all but one.

Requires `ffmpeg` on PATH.
