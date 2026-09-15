package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
)

var spaces = regexp.MustCompile(`\s+`)

const KeyVersion = "v1"

type Params struct {
	Text         string
	VoiceID      string
	ModelID      string
	Speed        float64
	OutputFormat string
	Language     string
}

func Normalize(text string) string {
	s := strings.TrimSpace(text)
	s = spaces.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, " .", ".")
	s = strings.ReplaceAll(s, " ,", ",")
	return strings.ToLower(s)
}

func speedKey(speed float64) string {
	if speed == 0 {
		return "1"
	}
	return strconv.FormatFloat(speed, 'f', 3, 64)
}

func Key(p Params) string {
	parts := []string{
		KeyVersion,
		Normalize(p.Text),
		strings.TrimSpace(p.VoiceID),
		strings.TrimSpace(p.ModelID),
		speedKey(p.Speed),
		strings.TrimSpace(p.OutputFormat),
		strings.TrimSpace(p.Language),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return KeyVersion + "-" + hex.EncodeToString(sum[:])[:16]
}

func Filename(key string) string { return key + ".mp3" }

func MetaFilename(key string) string { return key + ".meta.json" }
