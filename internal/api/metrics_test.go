package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

type metricsStub struct {
	snapshot map[string]any
	err      error
}

type observingMetricsStub struct {
	mu           sync.Mutex
	observations []HTTPRequestObservation
}

func (s *observingMetricsStub) MetricsSnapshot(context.Context) (map[string]any, error) {
	return map[string]any{"uptime_seconds": int64(1)}, nil
}

func (s *observingMetricsStub) ObserveHTTPRequest(observation HTTPRequestObservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations = append(s.observations, observation)
}

func (s *observingMetricsStub) snapshot() []HTTPRequestObservation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]HTTPRequestObservation(nil), s.observations...)
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

func TestMetricsMiddlewareObservesOnlyBoundedRouteTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("")
	provider := &observingMetricsStub{}
	registerMetricsRoute(group, provider)
	group.GET("/items/:id", func(c *gin.Context) {
		c.Status(http.StatusUnprocessableEntity)
	})

	request := httptest.NewRequest(http.MethodGet, "/items/customer-secret?api_key=query-secret", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	observations := provider.snapshot()
	if len(observations) != 1 {
		t.Fatalf("expected one request observation, got %+v", observations)
	}
	observation := observations[0]
	if observation.Method != http.MethodGet || observation.Route != "/items/:id" || observation.Status != http.StatusUnprocessableEntity {
		t.Fatalf("unexpected observation: %+v", observation)
	}
	observedText := observation.Method + " " + observation.Route
	for _, secret := range []string{"customer-secret", "query-secret", "api_key"} {
		if strings.Contains(observedText, secret) {
			t.Fatalf("request observation leaked %q: %q", secret, observedText)
		}
	}
}

func TestMetricsMiddlewareMarksCanceledRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("")
	provider := &observingMetricsStub{}
	registerMetricsRoute(group, provider)
	group.GET("/work", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/work", nil).WithContext(ctx)
	router.ServeHTTP(httptest.NewRecorder(), request)

	observations := provider.snapshot()
	if len(observations) != 1 || !observations[0].Canceled {
		t.Fatalf("expected one canceled request observation, got %+v", observations)
	}
}

func TestMetricsMiddlewareSkipsUnmatchedRawPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("")
	provider := &observingMetricsStub{}
	registerMetricsRoute(group, provider)

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/unknown/customer-secret", nil))
	if observations := provider.snapshot(); len(observations) != 0 {
		t.Fatalf("expected unmatched raw path to be skipped, got %+v", observations)
	}
}

func TestMetricsMiddlewareCountsRecoveredPanicAsServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	group := router.Group("")
	provider := &observingMetricsStub{}
	registerMetricsRoute(group, provider)
	group.GET("/panic", func(*gin.Context) {
		panic("boom")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	observations := provider.snapshot()
	if response.Code != http.StatusInternalServerError || len(observations) != 1 || observations[0].Status != http.StatusInternalServerError {
		t.Fatalf("expected recovered panic to be observed as 500, response=%d observations=%+v", response.Code, observations)
	}
}
