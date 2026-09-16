package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Request struct {
	Text    string
	VoiceID string
	ModelID string
	Speed   float64
	Format  string
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

type Synthesizer interface {
	Synthesize(ctx context.Context, r Request) ([]byte, error)
}

type ElevenLabsCompat struct {
	Client *http.Client
}

var _ Synthesizer = ElevenLabsCompat{}

func (e ElevenLabsCompat) Synthesize(ctx context.Context, r Request) ([]byte, error) {
	if e.Client == nil {
		e.Client = &http.Client{Timeout: 30 * time.Second}
	}
	query := url.Values{}
	query.Set("output_format", r.Format)
	endpoint := strings.TrimSuffix(r.BaseURL, "/") + "/text-to-speech/" + url.PathEscape(r.VoiceID) + "?" + query.Encode()
	body, _ := json.Marshal(map[string]any{
		"text":           r.Text,
		"model_id":       r.ModelID,
		"voice_settings": map[string]any{"speed": r.Speed},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", r.APIKey)
	resp, err := e.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream http %d: %s", resp.StatusCode, truncate(data))
	}
	return data, nil
}

func truncate(b []byte) string {
	if len(b) > 300 {
		return string(b[:300])
	}
	return string(b)
}
