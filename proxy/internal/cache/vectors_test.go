package cache_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anq-no-one/tts-cache/proxy/internal/cache"
)

func TestSharedVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "vectors", "vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vecs []struct {
		Input      string `json:"input"`
		Normalized string `json:"normalized"`
	}
	if err := json.Unmarshal(raw, &vecs); err != nil {
		t.Fatal(err)
	}
	for i, v := range vecs {
		if got := cache.Normalize(v.Input); got != v.Normalized {
			t.Fatalf("vector %d: got %q want %q", i, got, v.Normalized)
		}
	}
}
