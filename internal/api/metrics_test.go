package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type metricsStub struct {
	snapshot map[string]any
	err      error
}

func (s metricsStub) MetricsSnapshot(context.Context) (map[string]any, error) {
	return s.snapshot, s.err
}

func TestMetricsRouteServesRuntimeSnapshot(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{
		Metrics: metricsStub{snapshot: map[string]any{
			"uptime_seconds":      int64(42),
			"redis_inbox_pending": int64(3),
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode metrics body: %v", err)
	}
	if body["uptime_seconds"] != float64(42) {
		t.Fatalf("expected uptime_seconds 42, got %v", body["uptime_seconds"])
	}
	if body["redis_inbox_pending"] != float64(3) {
		t.Fatalf("expected redis_inbox_pending 3, got %v", body["redis_inbox_pending"])
	}
}

func TestMetricsRouteUnavailableWithoutProvider(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without metrics provider, got %d", recorder.Code)
	}
}

func TestMetricsRouteReportsProviderError(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{
		Metrics: metricsStub{err: context.DeadlineExceeded},
	})
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
}
