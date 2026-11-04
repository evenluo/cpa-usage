package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/service"
)

func TestAuthRenewalAcrossCPAV8SyncSQLiteAndIdentityAPI(t *testing.T) {
	v8Read := false
	cpaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer management-secret" {
			t.Errorf("unexpected management authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v8/management/credentials":
			v8Read = true
			_, _ = w.Write([]byte(`{"files":[{"auth_index":"codex-auth","name":"codex.json","type":"codex","provider":"codex","status":"error","requires_sign_in":true,"renewal_state":"expired_sign_in_required","recovery_stage":"sign_in_required","access_token_expires_at":"2026-10-29T10:00:00Z","next_action":"sign_in","recovery_hint":"Re-authenticate on this CPA instance.","refresh_error":{"code":"invalid_grant","message":"do not retain","http_status":400}}]}`))
		case "/v0/management/gemini-api-key", "/v0/management/claude-api-key", "/v0/management/codex-api-key", "/v0/management/vertex-api-key", "/v0/management/openai-compatibility":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer cpaServer.Close()

	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "auth-renewal.db")})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	syncService := service.NewSyncService(db, config.Config{
		CPABaseURL:       cpaServer.URL,
		CPAManagementKey: "management-secret",
		RequestTimeout:   2 * time.Second,
	})
	if err := syncService.SyncMetadata(context.Background()); err != nil {
		t.Fatalf("sync v8 metadata: %v", err)
	}
	if !v8Read {
		t.Fatal("expected the v8 credentials endpoint to be called")
	}

	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: repository.NewUsageIdentityReader(db)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/usage/identities", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK {
		t.Fatalf("expected identity API 200, got %d: %s", response.Code, body)
	}
	for _, expected := range []string{
		`"requires_sign_in":true`,
		`"renewal_state":"expired_sign_in_required"`,
		`"recovery_stage":"sign_in_required"`,
		`"access_token_expires_at":"2026-10-29T10:00:00Z"`,
		`"next_action":"sign_in"`,
		`"refresh_error":{"code":"invalid_grant","http_status":400}`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %s in identity response: %s", expected, body)
		}
	}
	if strings.Contains(body, "do not retain") {
		t.Fatalf("upstream refresh-error message leaked into identity response: %s", body)
	}
}
