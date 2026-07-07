package entities

import "testing"

func TestAllIncludesCoreModels(t *testing.T) {
	items := All()
	if len(items) != 6 {
		t.Fatalf("expected 6 core models including rollup backfill state, got %d", len(items))
	}
	if _, ok := items[0].(*UsageEvent); !ok {
		t.Fatalf("expected UsageEvent to be first registered model, got %T", items[0])
	}
	if _, ok := items[1].(*RedisUsageInbox); !ok {
		t.Fatalf("expected RedisUsageInbox to be registered, got %T", items[1])
	}
	if _, ok := items[4].(*KeyAlias); !ok {
		t.Fatalf("expected KeyAlias to be registered, got %T", items[4])
	}
	if _, ok := items[5].(*UsageRollupBackfillState); !ok {
		t.Fatalf("expected UsageRollupBackfillState to be registered, got %T", items[5])
	}
}
