package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var usageEventCacheTokenColumns = []string{
	"cache_read_tokens",
	"cache_creation_tokens",
}

const usageEventCacheColumnsTemporaryTable = "usage_events__cache_columns_nullable"

var usageEventsCreateTableHeaderPattern = regexp.MustCompile("(?i)^\\s*CREATE\\s+TABLE\\s+(?:usage_events|[\"`]usage_events[\"`]|\\[usage_events\\])\\s*")

type sqliteSchemaObject struct {
	objectType string
	name       string
	statement  string
}

func makeUsageEventCacheColumnsNullableMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("usage_events") {
		return nil
	}

	strictColumns, err := strictUsageEventCacheTokenColumns(tx)
	if err != nil {
		return err
	}
	if len(strictColumns) == 0 {
		return nil
	}

	schemaObjects, err := loadExplicitUsageEventSchemaObjects(tx)
	if err != nil {
		return err
	}
	sequence, hasSequence, err := loadUsageEventSequence(tx)
	if err != nil {
		return err
	}

	createTableSQL, err := loadUsageEventCreateTableSQL(tx)
	if err != nil {
		return err
	}
	temporaryTableSQL, err := rewriteUsageEventCacheColumnsTableSQL(createTableSQL, strictColumns)
	if err != nil {
		return err
	}
	columnNames, err := loadUsageEventStoredColumnNames(tx)
	if err != nil {
		return err
	}
	quotedColumns := make([]string, 0, len(columnNames))
	for _, column := range columnNames {
		quotedColumns = append(quotedColumns, quoteSQLiteIdentifier(column))
	}
	copyColumns := strings.Join(quotedColumns, ",")
	statements := []string{
		temporaryTableSQL,
		fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s", quoteSQLiteIdentifier(usageEventCacheColumnsTemporaryTable), copyColumns, copyColumns, quoteSQLiteIdentifier("usage_events")),
		fmt.Sprintf("DROP TABLE %s", quoteSQLiteIdentifier("usage_events")),
		fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quoteSQLiteIdentifier(usageEventCacheColumnsTemporaryTable), quoteSQLiteIdentifier("usage_events")),
	}
	for _, statement := range statements {
		if err := tx.Exec(statement).Error; err != nil {
			return fmt.Errorf("rebuild usage_events with nullable cache token columns: %w", err)
		}
	}
	remainingStrictColumns, err := strictUsageEventCacheTokenColumns(tx)
	if err != nil {
		return err
	}
	if len(remainingStrictColumns) > 0 {
		return fmt.Errorf("usage_events cache token columns remain NOT NULL after rebuild: %v", remainingStrictColumns)
	}
	for _, object := range schemaObjects {
		if err := tx.Exec(object.statement).Error; err != nil {
			return fmt.Errorf("restore usage_events %s %s: %w", object.objectType, object.name, err)
		}
	}
	if hasSequence {
		if err := restoreUsageEventSequence(tx, sequence); err != nil {
			return err
		}
	}
	return nil
}

func loadUsageEventCreateTableSQL(tx *gorm.DB) (string, error) {
	var statement string
	if err := tx.Raw(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'usage_events'`).Row().Scan(&statement); err != nil {
		return "", fmt.Errorf("read usage_events create table SQL: %w", err)
	}
	if strings.TrimSpace(statement) == "" {
		return "", fmt.Errorf("usage_events create table SQL is empty")
	}
	return statement, nil
}

func rewriteUsageEventCacheColumnsTableSQL(statement string, strictColumns []string) (string, error) {
	rewritten := statement
	for _, column := range strictColumns {
		pattern := strictUsageEventCacheColumnPattern(column)
		if matches := pattern.FindAllStringIndex(rewritten, -1); len(matches) != 1 {
			return "", fmt.Errorf("expected exactly one strict usage_events.%s definition, found %d", column, len(matches))
		}
		rewritten = pattern.ReplaceAllString(rewritten, "$1$2$3")
	}
	rewritten = usageEventsCreateTableHeaderPattern.ReplaceAllString(rewritten, "CREATE TABLE "+quoteSQLiteIdentifier(usageEventCacheColumnsTemporaryTable)+" ")
	if rewritten == statement || !strings.HasPrefix(strings.TrimSpace(rewritten), "CREATE TABLE "+quoteSQLiteIdentifier(usageEventCacheColumnsTemporaryTable)) {
		return "", fmt.Errorf("usage_events create table header does not match the published schema")
	}
	return rewritten, nil
}

func strictUsageEventCacheColumnPattern(column string) *regexp.Regexp {
	quotedColumn := "(?:" + regexp.QuoteMeta(column) + "|`" + regexp.QuoteMeta(column) + "`|\"" + regexp.QuoteMeta(column) + "\"|\\[" + regexp.QuoteMeta(column) + "\\])"
	defaultZero := `DEFAULT\s+(?:0|\(\s*0\s*\))`
	constraints := `(?:NOT\s+NULL\s+` + defaultZero + `|` + defaultZero + `\s+NOT\s+NULL)`
	return regexp.MustCompile(`(?i)(^|[,(]\s*)(` + quotedColumn + `\s+INTEGER)\s+` + constraints + `(\s*(?:,|\)))`)
}

func loadUsageEventStoredColumnNames(tx *gorm.DB) ([]string, error) {
	rows, err := tx.Raw(`PRAGMA table_xinfo(usage_events)`).Rows()
	if err != nil {
		return nil, fmt.Errorf("read usage_events stored columns: %w", err)
	}
	defer rows.Close()
	columns := make([]string, 0)
	for rows.Next() {
		var cid, notNull, primaryKey, hidden int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey, &hidden); err != nil {
			return nil, fmt.Errorf("scan usage_events stored columns: %w", err)
		}
		if hidden == 0 {
			columns = append(columns, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage_events stored columns: %w", err)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("usage_events has no stored columns")
	}
	return columns, nil
}

func quoteSQLiteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func strictUsageEventCacheTokenColumns(tx *gorm.DB) ([]string, error) {
	rows, err := tx.Raw(`PRAGMA table_info(usage_events)`).Rows()
	if err != nil {
		return nil, fmt.Errorf("read usage_events columns: %w", err)
	}
	defer rows.Close()

	notNullByColumn := make(map[string]bool, len(usageEventCacheTokenColumns))
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("scan usage_events columns: %w", err)
		}
		for _, target := range usageEventCacheTokenColumns {
			if name == target {
				notNullByColumn[name] = notNull != 0
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage_events columns: %w", err)
	}

	strictColumns := make([]string, 0, len(usageEventCacheTokenColumns))
	for _, column := range usageEventCacheTokenColumns {
		notNull, ok := notNullByColumn[column]
		if !ok {
			return nil, fmt.Errorf("usage_events.%s does not exist", column)
		}
		if notNull {
			strictColumns = append(strictColumns, column)
		}
	}
	return strictColumns, nil
}

func loadExplicitUsageEventSchemaObjects(tx *gorm.DB) ([]sqliteSchemaObject, error) {
	rows, err := tx.Raw(`
		SELECT type, name, sql
		FROM sqlite_master
		WHERE tbl_name = 'usage_events'
			AND type IN ('index', 'trigger')
			AND sql IS NOT NULL
		ORDER BY type, name`).Rows()
	if err != nil {
		return nil, fmt.Errorf("read usage_events schema objects: %w", err)
	}
	defer rows.Close()

	objects := make([]sqliteSchemaObject, 0)
	for rows.Next() {
		var object sqliteSchemaObject
		if err := rows.Scan(&object.objectType, &object.name, &object.statement); err != nil {
			return nil, fmt.Errorf("scan usage_events schema object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage_events schema objects: %w", err)
	}
	return objects, nil
}

func loadUsageEventSequence(tx *gorm.DB) (int64, bool, error) {
	var sequence int64
	err := tx.Raw(`SELECT seq FROM sqlite_sequence WHERE name = 'usage_events'`).Row().Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read usage_events sequence: %w", err)
	}
	return sequence, true, nil
}

func restoreUsageEventSequence(tx *gorm.DB, sequence int64) error {
	result := tx.Exec(`UPDATE sqlite_sequence SET seq = ? WHERE name = 'usage_events'`, sequence)
	if result.Error != nil {
		return fmt.Errorf("restore usage_events sequence: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	if err := tx.Exec(`INSERT INTO sqlite_sequence (name, seq) VALUES ('usage_events', ?)`, sequence).Error; err != nil {
		return fmt.Errorf("restore missing usage_events sequence: %w", err)
	}
	return nil
}
