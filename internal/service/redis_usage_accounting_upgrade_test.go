package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/cpa"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/repository/dto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const publishedStrictCacheColumnsMigration = "20260907_make_usage_event_cache_columns_nullable"

func TestAccountingV2UpgradeRepairsPublishedStrictCacheColumnsAndReplaysInboxAttempt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "usage.db")
	cfg := config.Config{SQLitePath: dbPath}

	fresh, err := repository.OpenDatabase(cfg)
	if err != nil {
		t.Fatalf("open fresh database: %v", err)
	}
	freshShapes := loadAccountingUpgradeColumnShapes(t, fresh)
	closeAccountingUpgradeDatabase(t, fresh)

	strict, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open published database shape: %v", err)
	}
	rebuildPublishedStrictUsageEvents(t, strict)
	assertAccountingUpgradeColumnShape(t, strict, "cache_read_tokens", accountingUpgradeColumnShape{NotNull: 1, DefaultText: "0"})
	assertAccountingUpgradeColumnShape(t, strict, "cache_creation_tokens", accountingUpgradeColumnShape{NotNull: 1, DefaultText: "0"})
	historicalAt := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	if err := strict.Exec(`INSERT INTO usage_events (
		event_key, request_id, provider, model, timestamp, source, auth_index,
		cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"published-history", "published-request", "published-provider", "published-model", historicalAt, "published-source", "published-account",
		21, 13, 5, 39,
	).Error; err != nil {
		t.Fatalf("seed published historical event: %v", err)
	}
	if err := strict.Exec("DELETE FROM schema_migrations WHERE version = ?", publishedStrictCacheColumnsMigration).Error; err != nil {
		t.Fatalf("make cache-nullability migration pending: %v", err)
	}
	closeAccountingUpgradeDatabase(t, strict)

	db, err := repository.OpenDatabase(cfg)
	if err != nil {
		t.Fatalf("run upgrade chain against published schema: %v", err)
	}
	t.Cleanup(func() { closeAccountingUpgradeDatabase(t, db) })
	assertAccountingUpgradeColumnShape(t, db, "cache_read_tokens", freshShapes["cache_read_tokens"])
	assertAccountingUpgradeColumnShape(t, db, "cache_creation_tokens", freshShapes["cache_creation_tokens"])
	for _, indexName := range publishedStrictUsageEventIndexNames {
		if !db.Migrator().HasIndex("usage_events", indexName) {
			t.Fatalf("upgrade dropped published usage_events index %s", indexName)
		}
	}

	var historical entities.UsageEvent
	if err := db.Where("event_key = ?", "published-history").First(&historical).Error; err != nil {
		t.Fatalf("load historical event after upgrade: %v", err)
	}
	if historical.EventKey != "published-history" || historical.RequestID != "published-request" || historical.Provider != "published-provider" || historical.Model != "published-model" ||
		!historical.Timestamp.Equal(historicalAt) || historical.Source != "published-source" || historical.AuthIndex != "published-account" || historical.CachedTokens != 21 ||
		historical.CacheReadTokens == nil || *historical.CacheReadTokens != 13 || historical.CacheCreationTokens == nil || *historical.CacheCreationTokens != 5 || historical.TotalTokens != 39 {
		t.Fatalf("upgrade changed published historical event: %+v", historical)
	}

	poppedAt := time.Date(2026, 9, 7, 15, 4, 5, 0, time.UTC)
	rows, err := repository.InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{{
		QueueKey:   cpa.ManagementUsageQueueKey,
		RawMessage: `{"timestamp":"2026-09-07T15:03:00Z","provider":"runtime-provider","model":"runtime-model","request_id":"runtime-request","accounting_version":2,"generate":true,"stream":true,"token_breakdown":{"schema_version":2,"quality":"complete","total_tokens":50,"input":{"total_tokens":30,"uncached_tokens":10,"cache_read_tokens":15,"cache_write_tokens":5},"output":{"total_tokens":20,"non_reasoning_tokens":14,"reasoning_tokens":6},"unclassified_tokens":0}}`,
		PoppedAt:   poppedAt,
	}})
	if err != nil || len(rows) != 1 {
		t.Fatalf("seed accounting v2 inbox row: rows=%+v err=%v", rows, err)
	}

	result, err := newRedisUsageProcessor(db).process(context.Background(), poppedAt.Add(time.Minute))
	if err != nil || result == nil || result.Status != "completed" || result.InsertedEvents != 1 || result.DedupedEvents != 0 {
		t.Fatalf("process accounting v2 inbox row: result=%+v err=%v", result, err)
	}
	expectedKey := redisUsageAttemptEventKey(rows[0].ID)
	var event entities.UsageEvent
	if err := db.Where("event_key = ?", expectedKey).First(&event).Error; err != nil {
		t.Fatalf("load persisted accounting v2 event: %v", err)
	}
	if event.AccountingState != repository.AccountingValid || event.TokenQuality == nil || *event.TokenQuality != "complete" ||
		event.CanonicalTotalTokens == nil || *event.CanonicalTotalTokens != 50 || event.CanonicalInputTokens == nil || *event.CanonicalInputTokens != 30 ||
		event.CanonicalCacheReadTokens == nil || *event.CanonicalCacheReadTokens != 15 || event.CanonicalCacheWriteTokens == nil || *event.CanonicalCacheWriteTokens != 5 ||
		event.CanonicalOutputTokens == nil || *event.CanonicalOutputTokens != 20 || event.CanonicalReasoningTokens == nil || *event.CanonicalReasoningTokens != 6 ||
		event.CacheReadTokens != nil || event.CacheCreationTokens != nil {
		t.Fatalf("unexpected canonical/archival event facts: %+v", event)
	}

	var rollup entities.UsageRollupHourly
	bucket := event.Timestamp.UTC().Truncate(time.Hour)
	if err := db.Where("bucket_start = ? AND provider = ? AND model = ?", bucket, "runtime-provider", "runtime-model").First(&rollup).Error; err != nil {
		t.Fatalf("load canonical hourly rollup: %v", err)
	}
	if rollup.RequestCount != 1 || rollup.AccountingValidAttempts != 1 || rollup.AccountingValidCompleteAttempts != 1 ||
		rollup.CanonicalTotalTokens != 50 || rollup.CanonicalInputTokens != 30 || rollup.CanonicalCacheReadTokens != 15 ||
		rollup.CanonicalCacheWriteTokens != 5 || rollup.CanonicalOutputTokens != 20 || rollup.CanonicalReasoningTokens != 6 {
		t.Fatalf("unexpected canonical hourly rollup: %+v", rollup)
	}

	if err := db.Model(&entities.RedisUsageInbox{}).Where("id = ?", rows[0].ID).Updates(map[string]any{
		"status": repository.RedisUsageInboxStatusPending, "usage_event_key": "", "processed_at": nil,
	}).Error; err != nil {
		t.Fatalf("restore original inbox ID for replay: %v", err)
	}
	result, err = newRedisUsageProcessor(db).process(context.Background(), poppedAt.Add(2*time.Minute))
	if err != nil || result == nil || result.Status != "completed" || result.InsertedEvents != 0 || result.DedupedEvents != 1 {
		t.Fatalf("replay original inbox ID: result=%+v err=%v", result, err)
	}
	var count int64
	if err := db.Model(&entities.UsageEvent{}).Where("event_key = ?", expectedKey).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("replay duplicated usage event: count=%d err=%v", count, err)
	}
	if err := db.Where("bucket_start = ? AND provider = ? AND model = ?", bucket, "runtime-provider", "runtime-model").First(&rollup).Error; err != nil {
		t.Fatalf("reload rollup after replay: %v", err)
	}
	if rollup.RequestCount != 1 || rollup.CanonicalTotalTokens != 50 || rollup.CanonicalCacheReadTokens != 15 {
		t.Fatalf("replay changed canonical rollup: %+v", rollup)
	}
	assertProcessedRedisInboxRow(t, db, rows[0].ID, expectedKey, poppedAt.Add(2*time.Minute))

	if err := db.Table("schema_migrations").Where("version = ?", publishedStrictCacheColumnsMigration).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("cache-nullability migration was not recorded once: count=%d err=%v", count, err)
	}
}

type accountingUpgradeColumnShape struct {
	NotNull     int
	DefaultText string
}

const publishedStrictUsageEventsDDL = `CREATE TABLE usage_events (
	id integer PRIMARY KEY AUTOINCREMENT,
	event_key text,
	api_group_key text,
	provider text,
	endpoint text,
	auth_type text,
	request_id text,
	model text,
	model_alias text,
	timestamp datetime,
	source text,
	auth_index text,
	failed numeric,
	latency_ms integer,
	input_tokens integer,
	output_tokens integer,
	reasoning_tokens integer,
	cached_tokens integer,
	total_tokens integer,
	created_at datetime,
	cache_read_tokens INTEGER NOT NULL DEFAULT 0,
	cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
	ttft_ms INTEGER,
	status_code INTEGER NOT NULL DEFAULT 0,
	executor_type TEXT NOT NULL DEFAULT '',
	reasoning_effort TEXT NOT NULL DEFAULT '',
	service_tier TEXT NOT NULL DEFAULT '',
	accounting_state TEXT NOT NULL DEFAULT 'absent',
	token_quality TEXT,
	canonical_total_tokens INTEGER,
	canonical_input_tokens INTEGER,
	canonical_uncached_tokens INTEGER,
	canonical_cache_read_tokens INTEGER,
	canonical_cache_write_tokens INTEGER,
	canonical_output_tokens INTEGER,
	canonical_non_reasoning_tokens INTEGER,
	canonical_reasoning_tokens INTEGER,
	canonical_unclassified_tokens INTEGER,
	generate numeric,
	stream numeric,
	response_service_tier TEXT
)`

var publishedStrictUsageEventIndexes = []struct {
	name string
	sql  string
}{
	{name: "idx_usage_events_api_group_key", sql: "CREATE INDEX idx_usage_events_api_group_key ON usage_events(api_group_key)"},
	{name: "idx_usage_events_auth_index", sql: "CREATE INDEX idx_usage_events_auth_index ON usage_events(auth_index)"},
	{name: "idx_usage_events_auth_type_auth_index_id", sql: "CREATE INDEX idx_usage_events_auth_type_auth_index_id ON usage_events(auth_type, auth_index, id)"},
	{name: "idx_usage_events_failed", sql: "CREATE INDEX idx_usage_events_failed ON usage_events(failed)"},
	{name: "idx_usage_events_model", sql: "CREATE INDEX idx_usage_events_model ON usage_events(model)"},
	{name: "idx_usage_events_timestamp_id", sql: "CREATE INDEX idx_usage_events_timestamp_id ON usage_events(timestamp DESC, id DESC)"},
	{name: "uniq_usage_events_event_key", sql: "CREATE UNIQUE INDEX uniq_usage_events_event_key ON usage_events(event_key)"},
}

var publishedStrictUsageEventIndexNames = func() []string {
	names := make([]string, 0, len(publishedStrictUsageEventIndexes))
	for _, index := range publishedStrictUsageEventIndexes {
		names = append(names, index.name)
	}
	return names
}()

func rebuildPublishedStrictUsageEvents(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("DROP TABLE usage_events").Error; err != nil {
		t.Fatalf("drop fresh usage_events: %v", err)
	}
	if err := db.Exec(publishedStrictUsageEventsDDL).Error; err != nil {
		t.Fatalf("create strict published usage_events: %v", err)
	}
	for _, index := range publishedStrictUsageEventIndexes {
		if err := db.Exec(index.sql).Error; err != nil {
			t.Fatalf("create strict published usage_events index %s: %v", index.name, err)
		}
	}
}

func loadAccountingUpgradeColumnShapes(t *testing.T, db *gorm.DB) map[string]accountingUpgradeColumnShape {
	t.Helper()
	rows, err := db.Raw("PRAGMA table_info(usage_events)").Rows()
	if err != nil {
		t.Fatalf("read usage_events schema: %v", err)
	}
	defer rows.Close()
	shapes := map[string]accountingUpgradeColumnShape{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan usage_events schema: %v", err)
		}
		shapes[name] = accountingUpgradeColumnShape{NotNull: notNull, DefaultText: strings.Trim(defaultValue.String, `"'`)}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate usage_events schema: %v", err)
	}
	return shapes
}

func assertAccountingUpgradeColumnShape(t *testing.T, db *gorm.DB, column string, want accountingUpgradeColumnShape) {
	t.Helper()
	got, ok := loadAccountingUpgradeColumnShapes(t, db)[column]
	if !ok || got != want {
		t.Fatalf("unexpected usage_events.%s schema: got=%+v want=%+v present=%t", column, got, want, ok)
	}
}

func closeAccountingUpgradeDatabase(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
}
