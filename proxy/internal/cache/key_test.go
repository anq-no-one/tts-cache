package cache_test

import (
	"testing"

	"github.com/anq-no-one/tts-cache/proxy/internal/cache"
)

func base() cache.Params {
	return cache.Params{
		Text: "Rest for 30 seconds.", VoiceID: "voice1",
		ModelID: "fish-audio/s2.1-pro", Speed: 1.08,
		OutputFormat: "mp3_44100_128", Language: "en",
	}
}

func TestNormalizeCollapsesNoise(t *testing.T) {
	a := cache.Normalize("  Rest   for 30 seconds . ")
	b := cache.Normalize("rest for 30 seconds.")
	if a != b {
		t.Fatalf("normalize mismatch: %q vs %q", a, b)
	}
}

func TestKeyIgnoresTransportOnlyFields(t *testing.T) {
	p := base()
	if cache.Key(p) != cache.Key(p) {
		t.Fatal("same params must give same key")
	}
}

func TestGoldenKeyParityWithSwift(t *testing.T) {
	got := cache.Key(cache.Params{
		Text: "Rest for 30 seconds.", VoiceID: "v1",
		ModelID: "fish-audio/s2.1-pro", Speed: 1.08,
		OutputFormat: "mp3_44100_128", Language: "en",
	})
	if got != "v1-feab4a17bc7070e8" {
		t.Fatalf("golden key mismatch: got %s", got)
	}
}

func TestKeyChangesOnSoundFields(t *testing.T) {
	p := base()
	changed := []cache.Params{}
	q := p
	q.Speed = 1.5
	changed = append(changed, q)
	q = p
	q.ModelID = "other-model"
	changed = append(changed, q)
	q = p
	q.Text = "Different sentence."
	changed = append(changed, q)
	for i, c := range changed {
		if cache.Key(p) == cache.Key(c) {
			t.Fatalf("case %d: key must change when sound params change", i)
		}
	}
}

func TestZeroSpeedHasStableDistinctKey(t *testing.T) {
	a := cache.Key(cache.Params{Text: "Hi.", VoiceID: "v", Speed: 0})
	b := cache.Key(cache.Params{Text: "Hi.", VoiceID: "v", Speed: 0})
	c := cache.Key(cache.Params{Text: "Hi.", VoiceID: "v", Speed: 1})
	if a != b {
		t.Fatalf("zero speed must be deterministic: %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("zero speed must not collide with 1.000: %q", a)
	}
}
