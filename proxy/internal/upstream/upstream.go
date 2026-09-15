package upstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Request struct {
	Text      string
	VoiceID   string
	ModelID   string
	Speed     float64
	Format    string
	APIKey    string
	BaseURL   string
	Timeout   time.Duration
	StreamURL string
}

type Synthesizer interface {
	Synthesize(r Request) ([]byte, error)
}

type ElevenLabsCompat struct {
	Client *http.Client
}

func (e ElevenLabsCompat) Synthesize(r Request) ([]byte, error) {
	if e.Client == nil {
		e.Client = &http.Client{Timeout: 30 * time.Second}
	}
	url := r.BaseURL + "/text-to-speech/" + r.VoiceID + "?output_format=" + r.Format
	body, _ := json.Marshal(map[string]any{
		"text":           r.Text,
		"model_id":       r.ModelID,
		"voice_settings": map[string]any{"speed": r.Speed},
	})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
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
