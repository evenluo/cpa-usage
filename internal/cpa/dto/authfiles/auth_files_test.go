package authfiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthFilesResponsePreservesAvailabilityFieldsAndAbsence(t *testing.T) {
	var response AuthFilesResponse
	if err := json.Unmarshal([]byte(`{"files":[{"auth_index":"with-state","status":"error","unavailable":true,"last_refresh":"2026-09-07T07:45:00Z","next_retry_after":"2026-09-07T08:30:00Z"},{"auth_index":"without-state"}]}`), &response); err != nil {
		t.Fatalf("decode auth files response: %v", err)
	}
	if len(response.Files) != 2 {
		t.Fatalf("expected two auth files, got %+v", response.Files)
	}
	withState := response.Files[0]
	if withState.Status == nil || *withState.Status != "error" || withState.Unavailable == nil || !*withState.Unavailable {
		t.Fatalf("expected status and unavailable observations, got %+v", withState)
	}
	wantLastRefresh := time.Date(2026, 9, 7, 7, 45, 0, 0, time.UTC)
	wantNextRetryAfter := time.Date(2026, 9, 7, 8, 30, 0, 0, time.UTC)
	if withState.LastRefresh == nil || !withState.LastRefresh.Equal(wantLastRefresh) || withState.NextRetryAfter == nil || !withState.NextRetryAfter.Equal(wantNextRetryAfter) {
		t.Fatalf("expected source timestamps, got %+v", withState)
	}
	withoutState := response.Files[1]
	if withoutState.Status != nil || withoutState.Unavailable != nil || withoutState.LastRefresh != nil || withoutState.NextRetryAfter != nil {
		t.Fatalf("expected missing source observations to remain absent, got %+v", withoutState)
	}
}

func TestPinnedAuthFileFixturesPreservePassiveQuotaAvailabilityBoundary(t *testing.T) {
	for _, testCase := range []struct {
		name, file string
		wantQuota  bool
	}{
		{name: "v7.2.152", file: "v7.2.152-passive-quota.json", wantQuota: true},
		{name: "v7.2.62", file: "v7.2.62-no-passive-quota.json", wantQuota: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			payload, err := os.ReadFile(filepath.Join("..", "..", "testdata", "authfiles", testCase.file))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var response AuthFilesResponse
			if err := json.Unmarshal(payload, &response); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			if len(response.Files) == 0 {
				t.Fatal("fixture contains no auth files")
			}
			if (response.Files[0].Quota != nil) != testCase.wantQuota {
				t.Fatalf("passive quota presence = %t, want %t", response.Files[0].Quota != nil, testCase.wantQuota)
			}
		})
	}
}

func TestAuthFilesResponsePreservesPassiveQuotaShapeWithoutTypingSignalValues(t *testing.T) {
	var response AuthFilesResponse
	if err := json.Unmarshal([]byte(`{"files":[{
		"auth_index":"codex-account",
		"quota":{"observed_at":"2026-09-07T08:00:00.123Z","signals":{"X-Codex-Primary-Used-Percent":"51","X-Codex-Unknown":7}},
		"model_quotas":{"gpt-5.3-codex":{"observed_at":"bad-time","signals":{"X-Codex-Allowed":true}}}
	}]}`), &response); err != nil {
		t.Fatalf("decode passive quota response: %v", err)
	}
	if len(response.Files) != 1 || response.Files[0].Quota == nil {
		t.Fatalf("expected account passive quota payload, got %+v", response.Files)
	}
	if response.Files[0].Quota.ObservedAt != "2026-09-07T08:00:00.123Z" || response.Files[0].Quota.Signals["X-Codex-Primary-Used-Percent"] != "51" {
		t.Fatalf("unexpected account passive quota payload: %+v", response.Files[0].Quota)
	}
	if response.Files[0].Quota.Signals["X-Codex-Unknown"] != float64(7) {
		t.Fatalf("expected malformed signal value to remain isolated for the normalizer, got %+v", response.Files[0].Quota.Signals)
	}
	if len(response.Files[0].ModelQuotas) != 1 {
		t.Fatalf("expected model passive quota payload, got %+v", response.Files[0].ModelQuotas)
	}
}
