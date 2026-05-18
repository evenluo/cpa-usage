package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const signedSessionVersion = "v1"

type SessionManager struct {
	ttl        time.Duration
	now        func() time.Time
	generate   func() (string, error)
	signingKey []byte

	mu       sync.RWMutex
	sessions map[string]time.Time
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	return &SessionManager{
		ttl:      ttl,
		now:      time.Now,
		generate: generateToken,
		sessions: make(map[string]time.Time),
	}
}

func NewSignedSessionManager(ttl time.Duration, signingKey string) *SessionManager {
	manager := NewSessionManager(ttl)
	manager.signingKey = []byte(signingKey)
	return manager
}

func (m *SessionManager) Create() (string, time.Time, error) {
	nonce, err := m.generate()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := m.now().Add(m.ttl)

	if len(m.signingKey) > 0 {
		return m.signToken(expiresAt, nonce), expiresAt, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredLocked()
	m.sessions[nonce] = expiresAt

	return nonce, expiresAt, nil
}

func (m *SessionManager) Validate(token string) bool {
	if token == "" {
		return false
	}
	if len(m.signingKey) > 0 {
		return m.validateSignedToken(token)
	}

	m.mu.RLock()
	expiresAt, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	if !expiresAt.After(m.now()) {
		m.Delete(token)
		return false
	}
	return true
}

func (m *SessionManager) Delete(token string) {
	if token == "" {
		return
	}
	if len(m.signingKey) > 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func (m *SessionManager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()
}

func (m *SessionManager) cleanupExpiredLocked() {
	now := m.now()
	for token, expiresAt := range m.sessions {
		if !expiresAt.After(now) {
			delete(m.sessions, token)
		}
	}
}

func (m *SessionManager) signToken(expiresAt time.Time, nonce string) string {
	payload := strings.Join([]string{
		signedSessionVersion,
		strconv.FormatInt(expiresAt.Unix(), 10),
		nonce,
	}, ".")
	signature := signSessionPayload(m.signingKey, payload)
	return payload + "." + signature
}

func (m *SessionManager) validateSignedToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != signedSessionVersion || parts[2] == "" || parts[3] == "" {
		return false
	}

	expiresUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false
	}
	expiresAt := time.Unix(expiresUnix, 0)
	if !expiresAt.After(m.now()) {
		return false
	}

	payload := strings.Join(parts[:3], ".")
	expectedSignature := signSessionPayload(m.signingKey, payload)
	return subtle.ConstantTimeCompare([]byte(parts[3]), []byte(expectedSignature)) == 1
}

func signSessionPayload(signingKey []byte, payload string) string {
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
