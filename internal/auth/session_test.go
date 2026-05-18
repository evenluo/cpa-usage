package auth

import (
	"testing"
	"time"
)

func TestSessionManagerCreateValidateDelete(t *testing.T) {
	manager := NewSessionManager(2 * time.Hour)
	manager.now = func() time.Time { return time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC) }
	manager.generate = func() (string, error) { return "token-1", nil }

	token, expiresAt, err := manager.Create()
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if token != "token-1" {
		t.Fatalf("expected token token-1, got %q", token)
	}
	if !expiresAt.Equal(time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected expiry: %s", expiresAt)
	}
	if !manager.Validate(token) {
		t.Fatal("expected token to validate")
	}

	manager.Delete(token)
	if manager.Validate(token) {
		t.Fatal("expected deleted token to fail validation")
	}
}

func TestSessionManagerRejectsExpiredSessions(t *testing.T) {
	baseTime := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	manager := NewSessionManager(30 * time.Minute)
	manager.now = func() time.Time { return baseTime }
	manager.generate = func() (string, error) { return "token-2", nil }

	token, _, err := manager.Create()
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	manager.now = func() time.Time { return baseTime.Add(31 * time.Minute) }
	if manager.Validate(token) {
		t.Fatal("expected expired token to fail validation")
	}
}

func TestSessionManagerCleanupExpired(t *testing.T) {
	baseTime := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	manager := NewSessionManager(time.Hour)
	manager.now = func() time.Time { return baseTime }
	manager.generate = func() (string, error) { return "token-3", nil }

	if _, _, err := manager.Create(); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	manager.mu.Lock()
	manager.sessions["expired"] = baseTime.Add(-time.Minute)
	manager.mu.Unlock()

	manager.CleanupExpired()

	manager.mu.RLock()
	_, expiredExists := manager.sessions["expired"]
	_, activeExists := manager.sessions["token-3"]
	manager.mu.RUnlock()

	if expiredExists {
		t.Fatal("expected expired token to be removed")
	}
	if !activeExists {
		t.Fatal("expected active token to remain")
	}
}

func TestSignedSessionManagerValidatesAcrossInstances(t *testing.T) {
	baseTime := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	issuer := NewSignedSessionManager(time.Hour, "0123456789abcdef0123456789abcdef")
	issuer.now = func() time.Time { return baseTime }
	issuer.generate = func() (string, error) { return "nonce-1", nil }

	token, expiresAt, err := issuer.Create()
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !expiresAt.Equal(baseTime.Add(time.Hour)) {
		t.Fatalf("unexpected expiry: %s", expiresAt)
	}

	validator := NewSignedSessionManager(time.Hour, "0123456789abcdef0123456789abcdef")
	validator.now = func() time.Time { return baseTime.Add(30 * time.Minute) }
	if !validator.Validate(token) {
		t.Fatal("expected signed token to validate in a separate manager")
	}
}

func TestSignedSessionManagerRejectsInvalidTokens(t *testing.T) {
	baseTime := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	manager := NewSignedSessionManager(time.Hour, "0123456789abcdef0123456789abcdef")
	manager.now = func() time.Time { return baseTime }
	manager.generate = func() (string, error) { return "nonce-2", nil }

	token, _, err := manager.Create()
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	wrongKey := NewSignedSessionManager(time.Hour, "abcdef0123456789abcdef0123456789")
	wrongKey.now = func() time.Time { return baseTime.Add(30 * time.Minute) }
	if wrongKey.Validate(token) {
		t.Fatal("expected token signed with another key to fail validation")
	}

	manager.now = func() time.Time { return baseTime.Add(2 * time.Hour) }
	if manager.Validate(token) {
		t.Fatal("expected expired signed token to fail validation")
	}

	manager.now = func() time.Time { return baseTime.Add(30 * time.Minute) }
	if manager.Validate(token + "tampered") {
		t.Fatal("expected tampered signed token to fail validation")
	}
}
