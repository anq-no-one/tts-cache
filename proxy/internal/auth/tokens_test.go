package auth

import (
	"path/filepath"
	"testing"
)

func TestIssueValidateRevoke(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "tokens.json"), []string{"invite-1"})
	id, raw, err := s.Issue("invite-1", "app-a")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if id == "" || raw == "" {
		t.Fatal("issue must return id and token")
	}
	if _, ok := s.Validate(raw); !ok {
		t.Fatal("issued token must validate")
	}
	if _, ok := s.Validate("wrong"); ok {
		t.Fatal("wrong token must not validate")
	}
	if _, _, err := s.Issue("nope", "app-b"); err != ErrBadInvite {
		t.Fatalf("bad invite must fail, got %v", err)
	}
	if !s.Revoke(id) {
		t.Fatal("revoke must succeed")
	}
	if _, ok := s.Validate(raw); ok {
		t.Fatal("revoked token must not validate")
	}
	if s.Revoke(id) {
		t.Fatal("second revoke must fail")
	}
}

func TestTokensSurviveReload(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "tokens.json"), []string{"invite-1"})
	_, raw, err := s.Issue("invite-1", "app-a")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	again := NewStore(filepath.Join(dir, "tokens.json"), []string{"invite-1"})
	if _, ok := again.Validate(raw); !ok {
		t.Fatal("token must validate after reload")
	}
}
