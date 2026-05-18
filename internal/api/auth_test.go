package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/auth"
)

func TestAuthSessionReportsAuthenticatedWhenDisabled(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, AuthConfig{Enabled: false}, nil, "")
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !contains(resp.Body.String(), `"authenticated":true`) {
		t.Fatalf("unexpected response: %d %s", resp.Code, resp.Body.String())
	}
}

func TestAuthProtectedRouteRequiresSessionWhenEnabled(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour}
	router := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, sessions), "")
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/overview", nil)

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.Code)
	}
}

func TestAuthLoginSetsCookieAndUnlocksProtectedRoute(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour}
	handler := NewAuthHandler(config, sessions)
	router := NewRouter(nil, nil, nil, nil, config, handler, "")

	loginResp := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginResp, loginReq)

	if loginResp.Code != http.StatusNoContent {
		t.Fatalf("expected login status 204, got %d", loginResp.Code)
	}
	cookie := loginResp.Result().Cookies()
	if len(cookie) == 0 {
		t.Fatal("expected auth cookie to be set")
	}
	if cookie[0].Name != defaultSessionCookieName {
		t.Fatalf("expected cookie %q, got %q", defaultSessionCookieName, cookie[0].Name)
	}
	if cookie[0].Path != "/" {
		t.Fatalf("expected root cookie path '/', got %q", cookie[0].Path)
	}

	usageResp := httptest.NewRecorder()
	usageReq := httptest.NewRequest(http.MethodGet, "/api/v1/usage/overview", nil)
	usageReq.AddCookie(cookie[0])
	router.ServeHTTP(usageResp, usageReq)

	if usageResp.Code != http.StatusOK {
		t.Fatalf("expected protected route to succeed, got %d %s", usageResp.Code, usageResp.Body.String())
	}
}

func TestAuthLoginRejectsWrongPassword(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour}
	router := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, sessions), "")
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.Code)
	}
}

func TestAuthLoginRateLimitsRepeatedFailures(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour}
	router := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, sessions), "")

	for i := 0; i < 5; i++ {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "198.51.100.1:1234"
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected failed attempt %d to return 401, got %d", i+1, resp.Code)
		}
	}

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.1:1234"
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected repeated failed attempts to return 429, got %d", resp.Code)
	}
}

func TestAuthLoginRateLimitUsesTrustedForwardedClientIP(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour, TrustedProxies: []string{"198.51.100.10"}}
	router := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, sessions), "")

	for i := 0; i < 5; i++ {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		req.RemoteAddr = "198.51.100.10:1234"
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected failed attempt %d to return 401, got %d", i+1, resp.Code)
		}
	}

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.8")
	req.RemoteAddr = "198.51.100.10:1234"
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected separate forwarded client IP to remain below the threshold, got %d", resp.Code)
	}
}

func TestAuthLoginFailedAttemptsExpire(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour}
	handler := NewAuthHandler(config, sessions)
	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	handler.now = func() time.Time { return now }
	router := NewRouter(nil, nil, nil, nil, config, handler, "")

	for i := 0; i < 5; i++ {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "198.51.100.1:1234"
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected failed attempt %d to return 401, got %d", i+1, resp.Code)
		}
	}

	now = now.Add(failedLoginAttemptWindow + time.Second)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.1:1234"
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected expired failed attempts to reset the threshold, got %d", resp.Code)
	}
}

func TestAuthLoginAllowsCorrectPasswordAfterRateLimitThreshold(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour}
	router := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, sessions), "")

	for i := 0; i < 5; i++ {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "198.51.100.2:1234"
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected failed attempt %d to return 401, got %d", i+1, resp.Code)
		}
	}

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.2:1234"
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected correct password to clear failed attempts and login, got %d", resp.Code)
	}
}

func TestAuthLogoutDeletesSessionCookie(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour}
	handler := NewAuthHandler(config, sessions)
	router := NewRouter(nil, nil, nil, nil, config, handler, "")

	loginResp := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusNoContent {
		t.Fatalf("expected login status 204, got %d", loginResp.Code)
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie to be set")
	}

	logoutResp := httptest.NewRecorder()
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(cookies[0])
	router.ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("expected logout status 204, got %d", logoutResp.Code)
	}
	clearCookies := logoutResp.Result().Cookies()
	if len(clearCookies) == 0 || clearCookies[0].Name != defaultSessionCookieName || clearCookies[0].MaxAge >= 0 {
		t.Fatalf("expected logout to clear session cookie, got %+v", clearCookies)
	}

	usageResp := httptest.NewRecorder()
	usageReq := httptest.NewRequest(http.MethodGet, "/api/v1/usage/overview", nil)
	usageReq.AddCookie(cookies[0])
	router.ServeHTTP(usageResp, usageReq)
	if usageResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected logged out session to be rejected, got %d", usageResp.Code)
	}
}

func TestSubpathAuthUsesPrefixedRoutesAndCookiePath(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour, BasePath: "/cpa"}
	handler := NewAuthHandler(config, sessions)
	router := NewRouter(nil, nil, nil, nil, config, handler, "/cpa")

	sessionResp := httptest.NewRecorder()
	sessionReq := httptest.NewRequest(http.MethodGet, "/cpa/api/v1/auth/session", nil)
	router.ServeHTTP(sessionResp, sessionReq)
	if sessionResp.Code != http.StatusOK || !contains(sessionResp.Body.String(), `"authenticated":false`) {
		t.Fatalf("unexpected session response: %d %s", sessionResp.Code, sessionResp.Body.String())
	}

	loginResp := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/cpa/api/v1/auth/login", strings.NewReader(`{"password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusNoContent {
		t.Fatalf("expected login status 204, got %d", loginResp.Code)
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie to be set")
	}
	if cookies[0].Path != "/cpa" {
		t.Fatalf("expected subpath cookie path '/cpa', got %q", cookies[0].Path)
	}

	usageResp := httptest.NewRecorder()
	usageReq := httptest.NewRequest(http.MethodGet, "/cpa/api/v1/usage/overview", nil)
	usageReq.AddCookie(cookies[0])
	router.ServeHTTP(usageResp, usageReq)
	if usageResp.Code != http.StatusOK {
		t.Fatalf("expected protected route under subpath to succeed, got %d %s", usageResp.Code, usageResp.Body.String())
	}

	unprefixedResp := httptest.NewRecorder()
	unprefixedReq := httptest.NewRequest(http.MethodGet, "/api/v1/usage/overview", nil)
	unprefixedReq.AddCookie(cookies[0])
	router.ServeHTTP(unprefixedResp, unprefixedReq)
	if unprefixedResp.Code != http.StatusNotFound {
		t.Fatalf("expected unprefixed route to 404, got %d", unprefixedResp.Code)
	}
}

func TestAuthLoginUsesConfiguredSharedCookieScope(t *testing.T) {
	sessions := auth.NewSignedSessionManager(time.Hour, "0123456789abcdef0123456789abcdef")
	config := AuthConfig{
		Enabled:             true,
		LoginPassword:       "secret",
		SessionTTL:          time.Hour,
		BasePath:            "/usage",
		SessionCookieName:   "shared_cpa_session",
		SessionCookieDomain: "cpa.example.com",
		SessionCookiePath:   "/",
	}
	router := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, sessions), "/usage")

	loginResp := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/usage/api/v1/auth/login", strings.NewReader(`{"password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-Forwarded-Proto", "https")
	router.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusNoContent {
		t.Fatalf("expected login status 204, got %d", loginResp.Code)
	}

	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie to be set")
	}
	cookie := cookies[0]
	if cookie.Name != "shared_cpa_session" || cookie.Domain != "cpa.example.com" || cookie.Path != "/" || !cookie.Secure {
		t.Fatalf("unexpected shared cookie scope: %+v", cookie)
	}

	otherSessions := auth.NewSignedSessionManager(time.Hour, "0123456789abcdef0123456789abcdef")
	otherRouter := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, otherSessions), "/usage")
	sessionResp := httptest.NewRecorder()
	sessionReq := httptest.NewRequest(http.MethodGet, "/usage/api/v1/auth/session", nil)
	sessionReq.AddCookie(cookie)
	otherRouter.ServeHTTP(sessionResp, sessionReq)
	if sessionResp.Code != http.StatusOK || !contains(sessionResp.Body.String(), `"authenticated":true`) {
		t.Fatalf("expected signed cookie to validate across handler instances, got %d %s", sessionResp.Code, sessionResp.Body.String())
	}
}

func TestAuthProtectedRouteAcceptsSharedBearerToken(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SharedBearerToken: "management-secret", SessionTTL: time.Hour}
	router := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, sessions), "")

	sessionResp := httptest.NewRecorder()
	sessionReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	sessionReq.Header.Set("Authorization", "Bearer management-secret")
	router.ServeHTTP(sessionResp, sessionReq)
	if sessionResp.Code != http.StatusOK || !contains(sessionResp.Body.String(), `"authenticated":true`) {
		t.Fatalf("expected shared bearer session to authenticate, got %d %s", sessionResp.Code, sessionResp.Body.String())
	}

	usageResp := httptest.NewRecorder()
	usageReq := httptest.NewRequest(http.MethodGet, "/api/v1/usage/overview", nil)
	usageReq.Header.Set("Authorization", "Bearer management-secret")
	router.ServeHTTP(usageResp, usageReq)
	if usageResp.Code != http.StatusOK {
		t.Fatalf("expected shared bearer token to unlock protected route, got %d %s", usageResp.Code, usageResp.Body.String())
	}
}

func TestAuthLoginAcceptsSharedBearerTokenAsPassword(t *testing.T) {
	sessions := auth.NewSessionManager(time.Hour)
	config := AuthConfig{Enabled: true, LoginPassword: "secret", SharedBearerToken: "management-secret", SessionTTL: time.Hour}
	router := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, sessions), "")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"management-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected shared bearer token to login, got %d %s", resp.Code, resp.Body.String())
	}
	if len(resp.Result().Cookies()) == 0 {
		t.Fatal("expected login with shared bearer token to set cpa-usage session cookie")
	}
}
