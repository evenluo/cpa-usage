package authfiles

import (
	"encoding/json"
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
