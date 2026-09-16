package upstream

import (
	"context"
	"testing"
)

type stubSynth struct {
	fixed []byte
}

func (s stubSynth) Synthesize(ctx context.Context, r Request) ([]byte, error) {
	return s.fixed, nil
}

var _ Synthesizer = stubSynth{}

func useSynth(s Synthesizer, r Request) ([]byte, error) {
	return s.Synthesize(context.Background(), r)
}

func TestStubSatisfiesSynthesizer(t *testing.T) {
	stub := stubSynth{fixed: []byte("STUB-AUDIO")}
	req := Request{Text: "hello", VoiceID: "v1", ModelID: "m", Format: "mp3_44100_128"}
	var s Synthesizer = stub
	got, err := useSynth(s, req)
	if err != nil {
		t.Fatalf("stub synthesize: %v", err)
	}
	if string(got) != "STUB-AUDIO" {
		t.Fatalf("unexpected stub audio %q", got)
	}
	var compat Synthesizer = ElevenLabsCompat{}
	if compat == nil {
		t.Fatal("ElevenLabsCompat must satisfy Synthesizer")
	}
}
