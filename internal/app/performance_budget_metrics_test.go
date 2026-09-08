package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPerformanceBudgetMetricsReadback(t *testing.T) {
	w := newBudgetWorkload(t, 1024, 64)
	w.run(t)
	rec := httptest.NewRecorder()
	w.app.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status: %d", rec.Code)
	}
	var snapshot struct {
		Database  databaseMetricsSnapshot    `json:"database"`
		Runtime   runtimeMetricsSnapshot     `json:"runtime"`
		HTTP      httpRequestMetricsSnapshot `json:"http_requests"`
		Pending   int64                      `json:"redis_inbox_pending"`
		AgeStatus string                     `json:"redis_inbox_oldest_pending_age_status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Database.Status != "available" || snapshot.Database.Pool == nil || snapshot.Runtime.Status != "available" || snapshot.Pending != 0 || snapshot.AgeStatus != "empty" {
		t.Fatalf("missing post-load observability: %s", rec.Body.String())
	}
	if len(snapshot.HTTP.Routes) != 3 {
		t.Fatalf("missing HTTP observations: %s", rec.Body.String())
	}
	for _, route := range snapshot.HTTP.Routes {
		if route.RequestsTotal != 3 || route.ErrorsTotal != 0 {
			t.Fatalf("unexpected route observations: %+v", route)
		}
	}
}
