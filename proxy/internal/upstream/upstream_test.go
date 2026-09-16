package upstream_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anq-no-one/tts-cache/proxy/internal/upstream"
)

func TestSynthesizePostsEscapedURLWithHeadersAndBody(t *testing.T) {
	var gotPath, gotQuery, gotKey, gotType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotQuery = r.URL.RawQuery
		gotKey = r.Header.Get("xi-api-key")
		gotType = r.Header.Get("Content-Type")
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.Write([]byte("AUDIO"))
	}))
	defer srv.Close()

	synth := upstream.ElevenLabsCompat{}
	audio, err := synth.Synthesize(context.Background(), upstream.Request{
		Text: "Hi.", VoiceID: "voice/id", ModelID: "m",
		Speed: 1.5, Format: "mp3 44100", APIKey: "secret", BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if string(audio) != "AUDIO" {
		t.Fatalf("unexpected audio %q", audio)
	}
	if gotPath != "/text-to-speech/voice%2Fid" {
		t.Fatalf("voice id must be path-escaped, got %q", gotPath)
	}
	if !strings.Contains(gotQuery, "output_format=mp3+44100") {
		t.Fatalf("format must be query-escaped, got %q", gotQuery)
	}
	if gotKey != "secret" || gotType != "application/json" {
		t.Fatalf("headers wrong: key=%q type=%q", gotKey, gotType)
	}
	if !strings.Contains(gotBody, `"text":"Hi."`) || !strings.Contains(gotBody, `"speed":1.5`) {
		t.Fatalf("body wrong: %q", gotBody)
	}
}

func TestSynthesizeKeepsShortErrorBodies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("slow down"))
	}))
	defer srv.Close()

	_, err := upstream.ElevenLabsCompat{}.Synthesize(context.Background(), upstream.Request{
		Text: "Hi.", VoiceID: "v", BaseURL: srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "slow down") {
		t.Fatalf("short error must pass through with status: %v", err)
	}
}

func TestSynthesizeTruncatesLongErrorBodies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(strings.Repeat("x", 500)))
	}))
	defer srv.Close()

	synth := upstream.ElevenLabsCompat{}
	_, err := synth.Synthesize(context.Background(), upstream.Request{
		Text: "Hi.", VoiceID: "v", BaseURL: srv.URL,
	})
	if err == nil {
		t.Fatal("non-200 must fail")
	}
	if strings.Count(err.Error(), "x") > 300 {
		t.Fatalf("error body must be truncated: %q", err.Error())
	}
}

func TestSynthesizeRespectsCanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("late"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := upstream.ElevenLabsCompat{}.Synthesize(ctx, upstream.Request{
		Text: "Hi.", VoiceID: "v", BaseURL: srv.URL,
	})
	if err == nil {
		t.Fatal("canceled context must fail")
	}
}

type stubSynth struct {
	audio []byte
	err   error
}

func (s stubSynth) Synthesize(ctx context.Context, r upstream.Request) ([]byte, error) {
	return s.audio, s.err
}

var _ upstream.Synthesizer = stubSynth{}

func TestStubSatisfiesSynthesizerInterface(t *testing.T) {
	var synth upstream.Synthesizer = stubSynth{audio: []byte("STUB")}
	audio, err := synth.Synthesize(context.Background(), upstream.Request{Text: "Hi."})
	if err != nil || string(audio) != "STUB" {
		t.Fatalf("stub must be interchangeable, got %q %v", audio, err)
	}
}

func TestSynthesizeRejectsBadURL(t *testing.T) {
	_, err := upstream.ElevenLabsCompat{}.Synthesize(context.Background(), upstream.Request{
		Text: "Hi.", VoiceID: "v", BaseURL: "http://exa mple.com",
	})
	if err == nil {
		t.Fatal("bad base url must fail")
	}
}
