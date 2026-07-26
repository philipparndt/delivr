package video

import (
	"strings"
	"testing"

	"github.com/philipparndt/delivr/internal/config"
)

func TestAudioForPrefersTheDevicesOwnTrack(t *testing.T) {
	global := &config.VideoAudioConfig{Track: "theme.mp3", FadeIn: 0.4, Volume: 0.9}
	own := &config.VideoAudioConfig{Track: "appletv.wav"}

	got := AudioFor(config.VideoDeviceConfig{Audio: own}, global)
	if got != own {
		t.Fatalf("device track ignored: got %+v", got)
	}
	// Replacement, not merge: inheriting the shared fade and volume would apply
	// settings measured against a different cut.
	if got.FadeIn != 0 || got.Volume != 0 {
		t.Errorf("shared settings leaked into the override: %+v", got)
	}
}

func TestAudioForFallsBackToTheSharedTrack(t *testing.T) {
	global := &config.VideoAudioConfig{Track: "theme.mp3"}
	if got := AudioFor(config.VideoDeviceConfig{}, global); got != global {
		t.Fatalf("expected the shared track, got %+v", got)
	}
	if got := AudioFor(config.VideoDeviceConfig{}, nil); got != nil {
		t.Fatalf("expected no track at all, got %+v", got)
	}
}

func TestAudioFilterAlwaysPads(t *testing.T) {
	// A track shorter than the preview would otherwise end early and leave the
	// tail with no audio stream — the rejection this whole path exists to avoid.
	got := audioFilter(&config.VideoAudioConfig{Track: "t.mp3"}, 28)
	if !strings.HasPrefix(got, "apad") {
		t.Errorf("filter must start by padding, got %q", got)
	}
}

func TestAudioFilterFades(t *testing.T) {
	got := audioFilter(&config.VideoAudioConfig{
		Track: "t.mp3", Volume: 0.9, FadeIn: 0.4, FadeOut: 1.5,
	}, 28)
	for _, want := range []string{"volume=0.900", "afade=t=in:st=0:d=0.400", "afade=t=out:st=26.500:d=1.500"} {
		if !strings.Contains(got, want) {
			t.Errorf("filter %q is missing %q", got, want)
		}
	}
}

func TestAudioFilterKeepsAFadeInsideAShortPreview(t *testing.T) {
	// A fade longer than the preview must not produce a negative start time,
	// which ffmpeg rejects outright.
	got := audioFilter(&config.VideoAudioConfig{Track: "t.mp3", FadeOut: 30}, 20)
	if strings.Contains(got, "st=-") {
		t.Errorf("negative fade start: %q", got)
	}
}

func TestEvenRoundsDownForH264(t *testing.T) {
	for in, want := range map[int]int{1920: 1920, 1921: 1920, 887: 886} {
		if got := even(in); got != want {
			t.Errorf("even(%d) = %d, want %d", in, got, want)
		}
	}
}
