package api

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/repository/dto"
	"cpa-usage/internal/service"
)

// Cross the real producer decoder, storage, raw/rollup readers and HTTP/CSV
// projections. Contradictory archival scalars must never become metric inputs.
func TestAccountingDirectCutAcrossIntakeAnalyticsEvidenceAndExport(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "direct-cut.db")})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	payload, err := os.ReadFile(filepath.Join("..", "cpa", "testdata", "usage", "v7.2.152-separate-reasoning.json"))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 7, 8, 20, 0, 0, time.UTC)
	event, _, err := service.DecodeRedisUsageMessage(string(payload), at)
	if err != nil {
		t.Fatal(err)
	}
	event.EventKey, event.Timestamp = "canonical-attempt", at
	historical := entities.UsageEvent{EventKey: "historical-attempt", Timestamp: at.Add(-time.Minute), Provider: event.Provider, Model: event.Model,
		InputTokens: 9_000_000, OutputTokens: 8_000_000, TotalTokens: 17_000_000, LatencyMS: 1200}
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{event, historical}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{Model: event.Model, PromptPricePer1M: 1, CompletionPricePer1M: 2, CachePricePer1M: .5}); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(nil, nil, repository.NewUsageReader(db), nil, AuthConfig{}, nil, "", OptionalProviders{Analytics: repository.NewAnalyticsReader(db)})
	get := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, response.Code, response.Body.String())
		}
		return response
	}
	corePath := "/api/v1/analytics/core?range=custom&start=2026-09-07T08:00:00Z&end=2026-09-07T09:00:00Z&granularity=hour"
	readCore := func() map[string]any {
		t.Helper()
		var result map[string]any
		if err := json.Unmarshal(get(corePath).Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	raw := readCore()
	summary := raw["summary"].(map[string]any)
	if summary["total_tokens"] != float64(142) || summary["request_count"] != float64(2) || summary["cost_status"] != "partial" || math.Abs(summary["total_cost"].(float64)-.000164) > 1e-12 {
		t.Fatalf("canonical metrics must not use archival scalars: %+v", summary)
	}
	accounting := summary["accounting"].(map[string]any)
	if accounting["coverage_pct"] != float64(50) {
		t.Fatalf("historical absence lost: %+v", accounting)
	}
	covered := at.Truncate(time.Hour)
	if err := repository.SaveUsageRollupBackfillStatus(db, dto.RollupBackfillStatus{Status: dto.RollupBackfillStatusCompleted, TargetBucketStart: &covered, CoveredBucketStart: &covered}); err != nil {
		t.Fatal(err)
	}
	rolled := readCore()
	if !reflect.DeepEqual(raw["summary"], rolled["summary"]) || !reflect.DeepEqual(raw["trend"], rolled["trend"]) {
		t.Fatalf("raw/rollup changed canonical metrics: raw=%+v rollup=%+v", raw, rolled)
	}
	var overview struct {
		Series struct {
			CostStatus map[string]string `json:"cost_status"`
		} `json:"series"`
	}
	if err := json.Unmarshal(get("/api/v1/usage/overview?range=custom&start=2026-09-07T08:00:00Z&end=2026-09-07T09:00:00Z").Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.Series.CostStatus["2026-09-07T08:00:00Z"] != "partial" {
		t.Fatalf("overview bucket must qualify its incomplete estimate: %+v", overview.Series)
	}
	window := "range=24h&window_end=2026-09-07T09:00:00Z"
	var evidence struct {
		Events []struct {
			RequestID    string `json:"request_id"`
			AttemptFacts struct {
				OutputTPS  *float64 `json:"output_tps"`
				Accounting struct {
					Total  *int64 `json:"total_tokens"`
					Output struct {
						Total *int64 `json:"total_tokens"`
					} `json:"output"`
				} `json:"accounting"`
			} `json:"attempt_facts"`
		} `json:"events"`
	}
	if err := json.Unmarshal(get("/api/v1/usage/events?"+window).Body.Bytes(), &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence.Events) != 2 {
		t.Fatalf("historical attempt metadata disappeared: %+v", evidence)
	}
	for _, row := range evidence.Events {
		if row.RequestID == event.RequestID {
			if row.AttemptFacts.Accounting.Total == nil || *row.AttemptFacts.Accounting.Total != 142 || row.AttemptFacts.Accounting.Output.Total == nil || *row.AttemptFacts.Accounting.Output.Total != 42 || row.AttemptFacts.OutputTPS == nil || *row.AttemptFacts.OutputTPS != 42 {
				t.Fatalf("evidence uses another token unit: %+v", row)
			}
		} else if row.AttemptFacts.Accounting.Total != nil || row.AttemptFacts.OutputTPS != nil {
			t.Fatalf("historical absence became a scalar fallback: %+v", row)
		}
	}
	exported := get("/api/v1/usage/events/export?" + window).Body.String()
	rows, err := csv.NewReader(strings.NewReader(exported)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || strings.Contains(exported, "17000000") || strings.Contains(exported, "9000000") {
		t.Fatalf("export revived archived token facts: %s", exported)
	}
}
