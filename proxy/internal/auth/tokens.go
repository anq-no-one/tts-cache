package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrBadInvite = errors.New("bad invitation code")

type Token struct {
	ID        string    `json:"id"`
	AppName   string    `json:"app_name"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	invites map[string]bool
	byID    map[string]*Token
	byHash  map[string]*Token
}

func NewStore(path string, invites []string) *Store {
	s := &Store{
		path:    path,
		invites: map[string]bool{},
		byID:    map[string]*Token{},
		byHash:  map[string]*Token{},
	}
	for _, code := range invites {
		if code != "" {
			s.invites[code] = true
		}
	}
	s.load()
	return s
}

func (s *Store) Issue(inviteCode, appName string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.invites[inviteCode] || inviteCode == "" {
		return "", "", ErrBadInvite
	}
	raw, err := randomHex(32)
	if err != nil {
		return "", "", err
	}
	id, err := randomHex(8)
	if err != nil {
		return "", "", err
	}
	t := &Token{
		ID:        id,
		AppName:   appName,
		Hash:      hashToken(raw),
		CreatedAt: time.Now().UTC(),
	}
	s.byID[id] = t
	s.byHash[t.Hash] = t
	if err := s.saveLocked(); err != nil {
		delete(s.byID, id)
		delete(s.byHash, t.Hash)
		return "", "", err
	}
	return id, raw, nil
}

func (s *Store) Validate(raw string) (*Token, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.byHash[hashToken(raw)]
	if !ok || t.Revoked {
		return nil, false
	}
	return t, true
}

func (s *Store) Revoke(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.byID[id]
	if !ok || t.Revoked {
		return false
	}
	t.Revoked = true
	if err := s.saveLocked(); err != nil {
		t.Revoked = false
		return false
	}
	return true
}

func (s *Store) file() string {
	return s.path
}

func (s *Store) load() {
	raw, err := os.ReadFile(s.file())
	if err != nil {
		return
	}
	var tokens []*Token
	if err := json.Unmarshal(raw, &tokens); err != nil {
		return
	}
	for _, t := range tokens {
		s.byID[t.ID] = t
		s.byHash[t.Hash] = t
	}
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tokens := make([]*Token, 0, len(s.byID))
	for _, t := range s.byID {
		tokens = append(tokens, t)
	}
	raw, err := json.Marshal(tokens)
	if err != nil {
		return err
	}
	return os.WriteFile(s.file(), raw, 0o600)
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func CheckSecret(have, want string) bool {
	if have == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(have), []byte(want)) == 1
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
