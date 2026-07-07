package repository

import (
	"cpa-usage/internal/repository/dto"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/migration"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func OpenDatabase(cfg config.Config) (*gorm.DB, error) {
	databaseExists, err := sqliteDatabaseFileExists(cfg.SQLitePath)
	if err != nil {
		return nil, err
	}
	if err := ensureSQLiteParentDir(cfg.SQLitePath); err != nil {
		return nil, err
	}
	dsn := sqliteDSN(cfg.SQLitePath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database %s: %w", filepath.Clean(cfg.SQLitePath), err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("configure sqlite database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		return nil, fmt.Errorf("enable sqlite WAL: %w", err)
	}
	if err := db.Exec("PRAGMA busy_timeout=5000").Error; err != nil {
		return nil, fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if err := db.Exec("PRAGMA foreign_keys=ON").Error; err != nil {
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	hasTables, err := sqliteDatabaseHasTables(db)
	if err != nil {
		return nil, err
	}
	if !databaseExists || !hasTables {
		if err := db.AutoMigrate(entities.All()...); err != nil {
			return nil, fmt.Errorf("auto migrate fresh database: %w", err)
		}
		if err := EnsureUsageRollupBackfillState(db); err != nil {
			return nil, fmt.Errorf("seed usage rollup backfill state: %w", err)
		}
		if err := migration.MarkAllAsApplied(db); err != nil {
			return nil, fmt.Errorf("mark schema migrations applied: %w", err)
		}
		return db, nil
	}

	if err := migration.Run(db); err != nil {
		return nil, fmt.Errorf("run schema migrations: %w", err)
	}
	if err := EnsureUsageRollupBackfillState(db); err != nil {
		return nil, fmt.Errorf("seed usage rollup backfill state: %w", err)
	}

	return db, nil
}

func sqliteDSN(path string) string {
	trimmed := strings.TrimSpace(path)
	if strings.Contains(trimmed, "?") {
		return trimmed
	}
	return trimmed + "?_busy_timeout=5000&_foreign_keys=on"
}

func sqliteDatabaseFileExists(path string) (bool, error) {
	trimmed := strings.TrimSpace(path)
	if before, _, ok := strings.Cut(trimmed, "?"); ok {
		trimmed = before
	}
	if trimmed == "" || trimmed == ":memory:" {
		return false, nil
	}
	_, err := os.Stat(trimmed)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("check sqlite database %s: %w", filepath.Clean(trimmed), err)
}

func ensureSQLiteParentDir(path string) error {
	trimmed := strings.TrimSpace(path)
	if before, _, ok := strings.Cut(trimmed, "?"); ok {
		trimmed = before
	}
	if trimmed == "" || trimmed == ":memory:" {
		return nil
	}
	dir := filepath.Dir(trimmed)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite directory %s: %w", filepath.Clean(dir), err)
	}
	return nil
}

func sqliteDatabaseHasTables(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
		return false, fmt.Errorf("check sqlite database tables: %w", err)
	}
	return count > 0, nil
}

func InsertUsageEvents(db *gorm.DB, events []entities.UsageEvent) (int, int, error) {
	if db == nil {
		return 0, 0, fmt.Errorf("database is nil")
	}
	if len(events) == 0 {
		return 0, 0, nil
	}

	inserted := 0
	affectedEventKeys := map[string]struct{}{}

	// 按仓储默认批次拆分写入，避免单条 INSERT 的 SQLite 变量数量过多。
	if err := db.Transaction(func(tx *gorm.DB) error {
		for start := 0; start < len(events); start += insertBatchSize(entities.UsageEvent{}) {
			end := min(start+insertBatchSize(entities.UsageEvent{}), len(events))
			batch := events[start:end]
			batchKeys := make([]string, 0, len(batch))
			for _, event := range batch {
				if key := strings.TrimSpace(event.EventKey); key != "" {
					batchKeys = append(batchKeys, key)
				}
			}
			existingKeys := map[string]struct{}{}
			if len(batchKeys) > 0 {
				var persistedKeys []string
				if err := tx.Model(&entities.UsageEvent{}).Where("event_key IN ?", batchKeys).Pluck("event_key", &persistedKeys).Error; err != nil {
					return fmt.Errorf("load existing usage event keys: %w", err)
				}
				for _, key := range persistedKeys {
					existingKeys[strings.TrimSpace(key)] = struct{}{}
				}
			}

			// 每批仍按 event_key 去重，保持原有重复事件忽略语义。
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "event_key"}},
				DoNothing: true,
			}).Create(&batch)
			if result.Error != nil {
				return fmt.Errorf("insert usage events: %w", result.Error)
			}
			if result.RowsAffected > 0 {
				for _, key := range batchKeys {
					if _, existed := existingKeys[key]; !existed {
						affectedEventKeys[key] = struct{}{}
					}
				}
			}
			inserted += int(result.RowsAffected)
		}

		if len(affectedEventKeys) == 0 {
			return nil
		}
		keys := make([]string, 0, len(affectedEventKeys))
		for key := range affectedEventKeys {
			keys = append(keys, key)
		}
		var affectedEvents []entities.UsageEvent
		if err := tx.Where("event_key IN ?", keys).Find(&affectedEvents).Error; err != nil {
			return fmt.Errorf("load affected usage events for hourly rollup rebuild: %w", err)
		}
		if err := RebuildUsageRollupsForEvents(tx, affectedEvents); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return 0, 0, err
	}

	deduped := len(events) - inserted
	return inserted, deduped, nil
}

// CleanupStorage 是每日维护任务的统一仓储清理入口：先清 Redis inbox，最后执行 VACUUM。
// VACUUM 必须在删除完成后单独执行，任何一步失败都会停止后续步骤并把已完成部分的结果返回给上层日志。
func CleanupStorage(db *gorm.DB, now time.Time) (dto.StorageCleanupResult, error) {
	redisResult, err := CleanupRedisUsageInbox(db, now)
	if err != nil {
		return dto.StorageCleanupResult{RedisInbox: redisResult}, err
	}
	if err := db.Exec("VACUUM").Error; err != nil {
		return dto.StorageCleanupResult{RedisInbox: redisResult}, err
	}
	return dto.StorageCleanupResult{RedisInbox: redisResult}, nil
}

func Vacuum(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	return db.Exec("VACUUM").Error
}
