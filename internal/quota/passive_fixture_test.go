package quota

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage/internal/cpa/dto/authfiles"
)

func TestPinnedProducerFixtureNormalizesAccountModelTimeAndUnits(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("..", "cpa", "testdata", "authfiles", "v7.2.152-passive-quota.json"))
	if err != nil {
		t.Fatalf("read supported fixture: %v", err)
	}
	var response authfiles.AuthFilesResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode supported fixture: %v", err)
	}
	if len(response.Files) != 2 {
		t.Fatalf("fixture files = %d, want 2", len(response.Files))
	}

	codex := NormalizePassiveQuotaSnapshot(response.Files[0].Type, response.Files[0].Quota, response.Files[0].ModelQuotas)
	wantAccountAt := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	if codex.Account == nil || !codex.Account.ObservedAt.Equal(wantAccountAt) || len(codex.Account.Quota) != 3 {
		t.Fatalf("unexpected Codex account fixture normalization: %+v", codex.Account)
	}
	primary := codex.Account.Quota[0]
	if primary.Scope != "account" || primary.UsedPercent == nil || *primary.UsedPercent != 51 || primary.Window == nil || primary.Window.Seconds != 18_000 {
		t.Fatalf("unexpected Codex percent/window units: %+v", primary)
	}
	credits := codex.Account.Quota[2]
	if credits.Unit != "credits" || credits.Remaining == nil || *credits.Remaining != 0 {
		t.Fatalf("unexpected Codex credit units: %+v", credits)
	}
	wantModelAt := time.Date(2026, 9, 7, 7, 30, 0, 0, time.UTC)
	if len(codex.Models) != 1 || codex.Models[0].Model != "gpt-5.3-codex" || !codex.Models[0].ObservedAt.Equal(wantModelAt) || codex.Models[0].Quota[0].Scope != "model" {
		t.Fatalf("unexpected Codex model fixture normalization: %+v", codex.Models)
	}

	claude := NormalizePassiveQuotaSnapshot(response.Files[1].Type, response.Files[1].Quota, response.Files[1].ModelQuotas)
	if claude.Account == nil || len(claude.Account.Quota) != 2 || claude.Account.Quota[0].UsedPercent == nil || *claude.Account.Quota[0].UsedPercent != 25 || claude.Account.Quota[1].UsedPercent == nil || *claude.Account.Quota[1].UsedPercent != 53 {
		t.Fatalf("unexpected Claude fraction-to-percent normalization: %+v", claude.Account)
	}
}

func TestPinnedV7262FixtureKeepsPassiveQuotaUnavailable(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("..", "cpa", "testdata", "authfiles", "v7.2.62-no-passive-quota.json"))
	if err != nil {
		t.Fatalf("read baseline fixture: %v", err)
	}
	var response authfiles.AuthFilesResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode baseline fixture: %v", err)
	}
	got := NormalizePassiveQuotaSnapshot(response.Files[0].Type, response.Files[0].Quota, response.Files[0].ModelQuotas)
	if got.Account != nil || len(got.Models) != 0 {
		t.Fatalf("v7.2.62 fixture must remain unavailable, got %+v", got)
	}
}
