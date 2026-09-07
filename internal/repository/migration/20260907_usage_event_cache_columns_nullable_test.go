package migration

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMakeUsageEventCacheColumnsNullableMigrationCorrectsPreviouslyAddedStrictColumns(t *testing.T) {
	db := openUsageEventCacheColumnsMigrationTestDatabase(t, "strict.db")
	defer closeOpenedDatabase(t, db)

	createUsageEventCacheColumnsTestTable(t, db, "integer NOT NULL DEFAULT 0", "integer NOT NULL DEFAULT 0")
	if err := addUsageAttemptFieldsMigration(db); err != nil {
		t.Fatalf("rerun original usage attempt fields migration: %v", err)
	}
	assertUsageEventCacheColumnShape(t, db, "cache_read_tokens", 1, "0")
	assertUsageEventCacheColumnShape(t, db, "cache_creation_tokens", 1, "0")

	if err := makeUsageEventCacheColumnsNullableMigration(db); err != nil {
		t.Fatalf("make usage event cache columns nullable: %v", err)
	}
	assertUsageEventCacheColumnShape(t, db, "cache_read_tokens", 0, "")
	assertUsageEventCacheColumnShape(t, db, "cache_creation_tokens", 0, "")
}

func TestMakeUsageEventCacheColumnsNullableMigrationPreservesDataSchemaObjectsConstraintsAndSequence(t *testing.T) {
	db := openUsageEventCacheColumnsMigrationTestDatabase(t, "preserve.db")
	defer closeOpenedDatabase(t, db)

	if err := db.Exec(`CREATE TABLE event_kinds (kind text PRIMARY KEY)`).Error; err != nil {
		t.Fatalf("create event kinds: %v", err)
	}
	if err := db.Exec(`INSERT INTO event_kinds (kind) VALUES ('attempt')`).Error; err != nil {
		t.Fatalf("seed event kinds: %v", err)
	}
	if err := db.Exec(`CREATE TABLE usage_event_audit (event_id integer, event_key text)`).Error; err != nil {
		t.Fatalf("create usage event audit: %v", err)
	}
	if err := db.Exec(`CREATE TABLE usage_events (
		id integer PRIMARY KEY AUTOINCREMENT,
		event_key text NOT NULL,
		kind text NOT NULL DEFAULT 'attempt',
		cache_read_tokens integer NOT NULL DEFAULT 0,
		cache_creation_tokens integer DEFAULT (0) NOT NULL,
		CONSTRAINT usage_events_event_key_unique UNIQUE (event_key),
		CONSTRAINT usage_events_nonnegative_cache CHECK (cache_read_tokens >= 0 AND cache_creation_tokens >= 0),
		CONSTRAINT usage_events_kind_fk FOREIGN KEY (kind) REFERENCES event_kinds(kind)
	)`).Error; err != nil {
		t.Fatalf("create strict usage events: %v", err)
	}
	for _, statement := range []string{
		`CREATE INDEX idx_usage_events_cache_expression ON usage_events(cache_read_tokens, lower(event_key)) WHERE cache_creation_tokens >= 0`,
		`CREATE TRIGGER trg_usage_events_audit AFTER INSERT ON usage_events BEGIN INSERT INTO usage_event_audit(event_id, event_key) VALUES (NEW.id, NEW.event_key); END`,
		`INSERT INTO usage_events (id, event_key, cache_read_tokens, cache_creation_tokens) VALUES (7, 'historical', 12, 34)`,
		`UPDATE sqlite_sequence SET seq = 99 WHERE name = 'usage_events'`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare strict usage events: %v", err)
		}
	}

	objectsBefore := loadUsageEventSchemaObjects(t, db)
	if err := makeUsageEventCacheColumnsNullableMigration(db); err != nil {
		t.Fatalf("make usage event cache columns nullable: %v", err)
	}

	assertUsageEventCacheColumnShape(t, db, "cache_read_tokens", 0, "")
	assertUsageEventCacheColumnShape(t, db, "cache_creation_tokens", 0, "")
	assertUsageEventSchemaObjectsEqual(t, objectsBefore, loadUsageEventSchemaObjects(t, db))

	var eventKey, kind string
	var cacheReadTokens, cacheCreationTokens int64
	if err := db.Raw(`SELECT event_key, kind, cache_read_tokens, cache_creation_tokens FROM usage_events WHERE id = 7`).Row().Scan(&eventKey, &kind, &cacheReadTokens, &cacheCreationTokens); err != nil {
		t.Fatalf("read preserved usage event: %v", err)
	}
	if eventKey != "historical" || kind != "attempt" || cacheReadTokens != 12 || cacheCreationTokens != 34 {
		t.Fatalf("unexpected preserved usage event: key=%q kind=%q read=%d creation=%d", eventKey, kind, cacheReadTokens, cacheCreationTokens)
	}

	tableSQL := loadSQLiteObjectSQL(t, db, "table", "usage_events")
	for _, constraint := range []string{
		"CONSTRAINT usage_events_event_key_unique UNIQUE (event_key)",
		"CONSTRAINT usage_events_nonnegative_cache CHECK (cache_read_tokens >= 0 AND cache_creation_tokens >= 0)",
		"CONSTRAINT usage_events_kind_fk FOREIGN KEY (kind) REFERENCES event_kinds(kind)",
	} {
		if !strings.Contains(tableSQL, constraint) {
			t.Fatalf("expected migrated table SQL to preserve %q, got %s", constraint, tableSQL)
		}
	}

	var sequence int64
	if err := db.Raw(`SELECT seq FROM sqlite_sequence WHERE name = 'usage_events'`).Row().Scan(&sequence); err != nil {
		t.Fatalf("read preserved usage_events sequence: %v", err)
	}
	if sequence != 99 {
		t.Fatalf("expected usage_events sequence 99, got %d", sequence)
	}

	if err := db.Exec(`INSERT INTO usage_events (event_key, cache_read_tokens, cache_creation_tokens) VALUES ('nullable', NULL, NULL)`).Error; err != nil {
		t.Fatalf("insert usage event with unknown cache facts: %v", err)
	}
	var insertedID int64
	if err := db.Raw(`SELECT id FROM usage_events WHERE event_key = 'nullable'`).Row().Scan(&insertedID); err != nil {
		t.Fatalf("read inserted nullable usage event: %v", err)
	}
	if insertedID != 100 {
		t.Fatalf("expected preserved sequence to allocate id 100, got %d", insertedID)
	}
	var auditCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM usage_event_audit WHERE event_id = 100 AND event_key = 'nullable'`).Row().Scan(&auditCount); err != nil {
		t.Fatalf("read usage event audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected restored trigger to audit nullable insert, got %d rows", auditCount)
	}
}

func TestMakeUsageEventCacheColumnsNullableMigrationChangesOnlyStrictColumn(t *testing.T) {
	for _, strictColumn := range []string{"cache_read_tokens", "cache_creation_tokens"} {
		t.Run(strictColumn, func(t *testing.T) {
			db := openUsageEventCacheColumnsMigrationTestDatabase(t, strictColumn+".db")
			defer closeOpenedDatabase(t, db)

			readDefinition := "integer DEFAULT 17 CHECK (cache_read_tokens >= 0)"
			creationDefinition := "integer DEFAULT 23 CHECK (cache_creation_tokens >= 0)"
			if strictColumn == "cache_read_tokens" {
				readDefinition = "integer NOT NULL DEFAULT 0"
			} else {
				creationDefinition = "integer NOT NULL DEFAULT 0"
			}
			createUsageEventCacheColumnsTestTable(t, db, readDefinition, creationDefinition)
			beforeSQL := loadSQLiteObjectSQL(t, db, "table", "usage_events")

			if err := makeUsageEventCacheColumnsNullableMigration(db); err != nil {
				t.Fatalf("make usage event cache columns nullable: %v", err)
			}

			assertUsageEventCacheColumnShape(t, db, strictColumn, 0, "")
			unchangedColumn := "cache_creation_tokens"
			unchangedDefinition := creationDefinition
			if strictColumn == "cache_creation_tokens" {
				unchangedColumn = "cache_read_tokens"
				unchangedDefinition = readDefinition
			}
			shape := loadUsageAttemptColumnShapes(t, db)[unchangedColumn]
			if shape.NotNull != 0 || shape.DefaultText == "" {
				t.Fatalf("expected already-nullable %s definition to remain nullable with its default, got %+v", unchangedColumn, shape)
			}
			afterSQL := loadSQLiteObjectSQL(t, db, "table", "usage_events")
			if !strings.Contains(afterSQL, unchangedDefinition) {
				t.Fatalf("expected already-nullable definition %q to survive; before=%s after=%s", unchangedDefinition, beforeSQL, afterSQL)
			}
		})
	}
}

func TestMakeUsageEventCacheColumnsNullableMigrationIsNoOpForNullableSchema(t *testing.T) {
	db := openUsageEventCacheColumnsMigrationTestDatabase(t, "nullable.db")
	defer closeOpenedDatabase(t, db)

	createUsageEventCacheColumnsTestTable(t, db, "integer", "integer")
	if err := db.Exec(`CREATE INDEX idx_usage_events_event_key ON usage_events(event_key)`).Error; err != nil {
		t.Fatalf("create usage event index: %v", err)
	}
	var schemaVersionBefore int64
	if err := db.Raw(`PRAGMA schema_version`).Row().Scan(&schemaVersionBefore); err != nil {
		t.Fatalf("read schema version before migration: %v", err)
	}

	if err := makeUsageEventCacheColumnsNullableMigration(db); err != nil {
		t.Fatalf("make usage event cache columns nullable: %v", err)
	}

	var schemaVersionAfter int64
	if err := db.Raw(`PRAGMA schema_version`).Row().Scan(&schemaVersionAfter); err != nil {
		t.Fatalf("read schema version after migration: %v", err)
	}
	if schemaVersionAfter != schemaVersionBefore {
		t.Fatalf("expected nullable schema migration to be a no-op: schema_version before=%d after=%d", schemaVersionBefore, schemaVersionAfter)
	}
}

func TestMakeUsageEventCacheColumnsNullableMigrationRollsBackWithLedgerTransaction(t *testing.T) {
	db := openUsageEventCacheColumnsMigrationTestDatabase(t, "rollback.db")
	defer closeOpenedDatabase(t, db)

	createUsageEventCacheColumnsTestTable(t, db, "integer NOT NULL DEFAULT 0", "integer NOT NULL DEFAULT 0")
	if err := db.Exec(`INSERT INTO usage_events (id, event_key, cache_read_tokens, cache_creation_tokens) VALUES (3, 'rollback', 4, 5)`).Error; err != nil {
		t.Fatalf("seed rollback usage event: %v", err)
	}
	if err := db.Exec(`UPDATE sqlite_sequence SET seq = 29 WHERE name = 'usage_events'`).Error; err != nil {
		t.Fatalf("set rollback sequence: %v", err)
	}
	if err := createSchemaMigrationsTable(db); err != nil {
		t.Fatalf("create schema migrations table: %v", err)
	}
	testVersion := "test_nullable_cache_columns_rollback"
	err := runSchemaMigration(db, databaseMigration{
		version: testVersion,
		run: func(tx *gorm.DB) error {
			if err := makeUsageEventCacheColumnsNullableMigration(tx); err != nil {
				return err
			}
			return fmt.Errorf("force rollback after rebuild")
		},
	})
	if err == nil {
		t.Fatal("expected forced migration failure")
	}

	assertUsageEventCacheColumnShape(t, db, "cache_read_tokens", 1, "0")
	assertUsageEventCacheColumnShape(t, db, "cache_creation_tokens", 1, "0")
	var rowCount, sequence, ledgerCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM usage_events WHERE id = 3 AND event_key = 'rollback' AND cache_read_tokens = 4 AND cache_creation_tokens = 5`).Row().Scan(&rowCount); err != nil {
		t.Fatalf("read rolled back usage event: %v", err)
	}
	if err := db.Raw(`SELECT seq FROM sqlite_sequence WHERE name = 'usage_events'`).Row().Scan(&sequence); err != nil {
		t.Fatalf("read rolled back sequence: %v", err)
	}
	if err := db.Table("schema_migrations").Where("version = ?", testVersion).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("read rolled back migration ledger: %v", err)
	}
	if rowCount != 1 || sequence != 29 || ledgerCount != 0 {
		t.Fatalf("expected complete rollback, got row_count=%d sequence=%d ledger_count=%d", rowCount, sequence, ledgerCount)
	}
}

func TestMakeUsageEventCacheColumnsNullableMigrationRejectsUnknownStrictDefinitionWithoutMutation(t *testing.T) {
	db := openUsageEventCacheColumnsMigrationTestDatabase(t, "unknown-definition.db")
	defer closeOpenedDatabase(t, db)

	createUsageEventCacheColumnsTestTable(t, db, "integer NOT NULL DEFAULT 1", "integer NOT NULL DEFAULT 0")
	if err := createSchemaMigrationsTable(db); err != nil {
		t.Fatalf("create schema migrations table: %v", err)
	}
	tableSQLBefore := loadSQLiteObjectSQL(t, db, "table", "usage_events")
	err := runSchemaMigration(db, databaseMigration{
		version: "test_nullable_cache_columns_unknown_definition",
		run:     makeUsageEventCacheColumnsNullableMigration,
	})
	if err == nil || !strings.Contains(err.Error(), "expected exactly one strict usage_events.cache_read_tokens definition, found 0") {
		t.Fatalf("expected unknown strict definition failure, got %v", err)
	}
	if tableSQLAfter := loadSQLiteObjectSQL(t, db, "table", "usage_events"); tableSQLAfter != tableSQLBefore {
		t.Fatalf("expected rejected schema to remain unchanged: before=%s after=%s", tableSQLBefore, tableSQLAfter)
	}
	if db.Migrator().HasTable(usageEventCacheColumnsTemporaryTable) {
		t.Fatal("expected rejected migration not to create its temporary table")
	}
	var ledgerCount int64
	if err := db.Table("schema_migrations").Where("version = ?", "test_nullable_cache_columns_unknown_definition").Count(&ledgerCount).Error; err != nil {
		t.Fatalf("read rejected migration ledger: %v", err)
	}
	if ledgerCount != 0 {
		t.Fatalf("expected rejected migration not to be recorded, got %d rows", ledgerCount)
	}
}

func openUsageEventCacheColumnsMigrationTestDatabase(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), name))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}

func createUsageEventCacheColumnsTestTable(t *testing.T, db *gorm.DB, readDefinition, creationDefinition string) {
	t.Helper()
	statement := fmt.Sprintf(`CREATE TABLE usage_events (
		id integer PRIMARY KEY AUTOINCREMENT,
		event_key text,
		cache_read_tokens %s,
		cache_creation_tokens %s
	)`, readDefinition, creationDefinition)
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("create usage events: %v", err)
	}
}

func assertUsageEventCacheColumnShape(t *testing.T, db *gorm.DB, column string, wantNotNull int, wantDefault string) {
	t.Helper()
	shape, ok := loadUsageAttemptColumnShapes(t, db)[column]
	if !ok {
		t.Fatalf("expected usage_events.%s", column)
	}
	if shape.NotNull != wantNotNull || shape.DefaultText != wantDefault {
		t.Fatalf("unexpected usage_events.%s shape: got %+v, want not_null=%d default=%q", column, shape, wantNotNull, wantDefault)
	}
}

func loadUsageEventSchemaObjects(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	rows, err := db.Raw(`SELECT type, name, sql FROM sqlite_master WHERE tbl_name = 'usage_events' AND type IN ('index', 'trigger') AND sql IS NOT NULL`).Rows()
	if err != nil {
		t.Fatalf("read usage event schema objects: %v", err)
	}
	defer rows.Close()
	objects := make(map[string]string)
	for rows.Next() {
		var objectType, name, statement string
		if err := rows.Scan(&objectType, &name, &statement); err != nil {
			t.Fatalf("scan usage event schema object: %v", err)
		}
		objects[objectType+":"+name] = statement
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate usage event schema objects: %v", err)
	}
	return objects
}

func assertUsageEventSchemaObjectsEqual(t *testing.T, want, got map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected schema objects %v, got %v", want, got)
	}
	for name, wantSQL := range want {
		if got[name] != wantSQL {
			t.Fatalf("expected schema object %s SQL %q, got %q", name, wantSQL, got[name])
		}
	}
}

func loadSQLiteObjectSQL(t *testing.T, db *gorm.DB, objectType, name string) string {
	t.Helper()
	var statement sql.NullString
	if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type = ? AND name = ?`, objectType, name).Row().Scan(&statement); err != nil {
		t.Fatalf("read sqlite %s %s SQL: %v", objectType, name, err)
	}
	return statement.String
}
