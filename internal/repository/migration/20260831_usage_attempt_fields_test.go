package migration

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const usageAttemptMigrationBenchmarkRowCount = 65_536

func TestAddUsageAttemptFieldsMigrationPreservesLegacyCacheProjection(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "legacy.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeOpenedDatabase(t, db)

	if err := db.Exec(`CREATE TABLE usage_events (
		id integer PRIMARY KEY AUTOINCREMENT,
		event_key text,
		cached_tokens integer
	)`).Error; err != nil {
		t.Fatalf("create legacy usage_events table: %v", err)
	}
	if err := db.Exec(`INSERT INTO usage_events (event_key, cached_tokens) VALUES (?, ?)`, "legacy", 7).Error; err != nil {
		t.Fatalf("seed legacy usage event: %v", err)
	}

	if err := addUsageAttemptFieldsMigration(db); err != nil {
		t.Fatalf("add usage attempt fields: %v", err)
	}

	for _, column := range []string{
		"status_code",
		"executor_type",
		"reasoning_effort",
		"service_tier",
		"cache_read_tokens",
		"cache_creation_tokens",
	} {
		if !db.Migrator().HasColumn("usage_events", column) {
			t.Fatalf("expected usage_events.%s column", column)
		}
	}

	var cachedTokens int64
	var cacheReadTokens, cacheCreationTokens sql.NullInt64
	if err := db.Raw(`SELECT cached_tokens, cache_read_tokens, cache_creation_tokens FROM usage_events WHERE event_key = ?`, "legacy").Row().Scan(
		&cachedTokens,
		&cacheReadTokens,
		&cacheCreationTokens,
	); err != nil {
		t.Fatalf("scan migrated usage event: %v", err)
	}
	if cachedTokens != 7 {
		t.Fatalf("expected legacy cached_tokens projection to remain 7, got %d", cachedTokens)
	}
	if cacheReadTokens.Valid || cacheCreationTokens.Valid {
		t.Fatalf("expected historical explicit cache facts to remain unknown, got read=%+v creation=%+v", cacheReadTokens, cacheCreationTokens)
	}

	freshDB, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "fresh.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open fresh database: %v", err)
	}
	defer closeOpenedDatabase(t, freshDB)
	if err := freshDB.AutoMigrate(&entities.UsageEvent{}); err != nil {
		t.Fatalf("auto-migrate fresh usage_events: %v", err)
	}

	upgradedColumns := loadUsageAttemptColumnShapes(t, db)
	freshColumns := loadUsageAttemptColumnShapes(t, freshDB)
	for _, name := range []string{"status_code", "executor_type", "reasoning_effort", "service_tier"} {
		if upgradedColumns[name] != freshColumns[name] {
			t.Fatalf("expected fresh and upgraded %s constraints to match: fresh=%+v upgraded=%+v", name, freshColumns[name], upgradedColumns[name])
		}
	}
}

type usageAttemptColumnShape struct {
	NotNull     int
	DefaultText string
}

func loadUsageAttemptColumnShapes(t *testing.T, db *gorm.DB) map[string]usageAttemptColumnShape {
	t.Helper()
	rows, err := db.Raw(`PRAGMA table_info(usage_events)`).Rows()
	if err != nil {
		t.Fatalf("read usage_events schema: %v", err)
	}
	defer rows.Close()
	shapes := make(map[string]usageAttemptColumnShape)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan usage_events schema: %v", err)
		}
		shapes[name] = usageAttemptColumnShape{NotNull: notNull, DefaultText: strings.Trim(defaultValue.String, `'"`)}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate usage_events schema: %v", err)
	}
	return shapes
}

// BenchmarkAddUsageAttemptFieldsMigrationHighCardinality measures the six
// schema-only ALTER TABLE statements against a deterministic legacy table. The
// 65,536 existing rows are seeded before the timer because the migration itself
// must not include fixture write cost.
func BenchmarkAddUsageAttemptFieldsMigrationHighCardinality(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.ReportMetric(usageAttemptMigrationBenchmarkRowCount, "legacy_usage_events")
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		b.StopTimer()
		db := openUsageAttemptMigrationBenchmarkDatabase(b, benchmarkIndex)
		beforeBytes := usageAttemptMigrationDatabaseBytes(b, db)
		b.StartTimer()
		if err := addUsageAttemptFieldsMigration(db); err != nil {
			b.Fatalf("add usage attempt fields migration: %v", err)
		}
		b.StopTimer()

		for _, column := range []string{
			"status_code",
			"executor_type",
			"reasoning_effort",
			"service_tier",
			"cache_read_tokens",
			"cache_creation_tokens",
		} {
			if !db.Migrator().HasColumn("usage_events", column) {
				b.Fatalf("migration benchmark missing usage_events.%s", column)
			}
		}
		afterBytes := usageAttemptMigrationDatabaseBytes(b, db)
		b.ReportMetric(float64(afterBytes), "database_bytes")
		b.ReportMetric(float64(afterBytes-beforeBytes), "database_growth_bytes")
		closeUsageAttemptMigrationBenchmarkDatabase(b, db)
	}
}

func openUsageAttemptMigrationBenchmarkDatabase(b *testing.B, benchmarkIndex int) *gorm.DB {
	b.Helper()
	dbPath := filepath.Join(b.TempDir(), fmt.Sprintf("usage-attempt-migration-%d.db", benchmarkIndex))
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(dbPath)), &gorm.Config{})
	if err != nil {
		b.Fatalf("open legacy migration benchmark database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE usage_events (
		id integer PRIMARY KEY AUTOINCREMENT,
		event_key text,
		cached_tokens integer
	)`).Error; err != nil {
		b.Fatalf("create legacy usage_events benchmark table: %v", err)
	}
	seedUsageAttemptMigrationBenchmarkRows(b, db, usageAttemptMigrationBenchmarkRowCount)
	return db
}

func seedUsageAttemptMigrationBenchmarkRows(b *testing.B, db *gorm.DB, rowCount int) {
	b.Helper()
	const batchSize = 500
	for start := 0; start < rowCount; start += batchSize {
		end := min(start+batchSize, rowCount)
		var statement strings.Builder
		statement.WriteString("INSERT INTO usage_events (event_key, cached_tokens) VALUES ")
		args := make([]any, 0, (end-start)*2)
		for rowIndex := start; rowIndex < end; rowIndex++ {
			if rowIndex > start {
				statement.WriteString(",")
			}
			statement.WriteString("(?, ?)")
			args = append(args, fmt.Sprintf("legacy-event-%06d", rowIndex), rowIndex%500)
		}
		if err := db.Exec(statement.String(), args...).Error; err != nil {
			b.Fatalf("seed legacy usage event rows %d-%d: %v", start, end, err)
		}
	}
}

func usageAttemptMigrationDatabaseBytes(b *testing.B, db *gorm.DB) int64 {
	b.Helper()
	var pageCount, pageSize int64
	if err := db.Raw("PRAGMA page_count").Scan(&pageCount).Error; err != nil {
		b.Fatalf("read migration benchmark page count: %v", err)
	}
	if err := db.Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
		b.Fatalf("read migration benchmark page size: %v", err)
	}
	return pageCount * pageSize
}

func closeUsageAttemptMigrationBenchmarkDatabase(b *testing.B, db *gorm.DB) {
	b.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		b.Fatalf("get migration benchmark database handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		b.Fatalf("close migration benchmark database: %v", err)
	}
}
