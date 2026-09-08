package repository

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// sqliteTrimSpaceCharacters mirrors the Unicode White_Space set used by
// strings.TrimSpace. SQLite's one-argument trim only removes ASCII spaces.
const sqliteTrimSpaceCharacters = "\t\n\v\f\r \u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000"

type hourlyUsageRollupKey struct {
	BucketStart    time.Time
	Provider       string
	Model          string
	AuthType       string
	AuthIndex      string
	APIKeyIdentity string
}

// RebuildUsageRollupsForEvents rebuilds every hourly bucket touched by the given raw events.
func RebuildUsageRollupsForEvents(db *gorm.DB, seedEvents []entities.UsageEvent) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if len(seedEvents) == 0 {
		return nil
	}

	buckets := map[time.Time]struct{}{}
	for _, event := range seedEvents {
		bucket := event.Timestamp.UTC().Truncate(time.Hour)
		buckets[bucket] = struct{}{}
	}
	if len(buckets) == 0 {
		return nil
	}

	bucketList := make([]time.Time, 0, len(buckets))
	for bucket := range buckets {
		bucketList = append(bucketList, bucket)
	}
	sort.Slice(bucketList, func(i, j int) bool {
		return bucketList[i].Before(bucketList[j])
	})

	events := make([]entities.UsageEvent, 0, len(seedEvents))
	for _, bucket := range bucketList {
		var bucketEvents []entities.UsageEvent
		if err := db.
			Where("timestamp >= ? AND timestamp < ?", bucket, bucket.Add(time.Hour)).
			Find(&bucketEvents).Error; err != nil {
			return fmt.Errorf("load usage events for hourly rollup rebuild: %w", err)
		}
		events = append(events, bucketEvents...)
	}

	return rebuildUsageRollupsForBuckets(db, bucketList, events)
}

func RebuildUsageRollupsForBucketRange(db *gorm.DB, startBucket time.Time, endBucket time.Time) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		return rebuildUsageRollupsForBucketRange(tx, startBucket, endBucket)
	})
}

func rebuildUsageRollupsForBucketRange(db *gorm.DB, startBucket time.Time, endBucket time.Time) error {
	start := startBucket.UTC().Truncate(time.Hour)
	end := endBucket.UTC().Truncate(time.Hour)
	if end.Before(start) {
		return nil
	}

	bucketList := hourlyBucketRange(start, end)
	if err := db.Where("bucket_start IN ?", bucketList).Delete(&entities.UsageRollupHourly{}).Error; err != nil {
		return fmt.Errorf("delete hourly usage rollups: %w", err)
	}
	return rebuildUsageRollupsForRangeWithSQL(db, start, end.Add(time.Hour))
}

// rebuildUsageRollupsForRangeWithSQL keeps raw rows inside SQLite and only
// materializes their hourly aggregates. The surrounding transaction owns the
// delete-and-rebuild atomicity together with the backfill coverage checkpoint.
func rebuildUsageRollupsForRangeWithSQL(db *gorm.DB, start time.Time, endExclusive time.Time) error {
	now := time.Now().UTC()
	result := db.Exec(`
		INSERT INTO usage_rollups_hourly (
			bucket_start, provider, model, auth_type, auth_index, api_key_identity,
			request_count, success_count, failure_count,
			accounting_absent_attempts, accounting_valid_attempts,
			accounting_valid_complete_attempts, accounting_valid_inconsistent_attempts,
			accounting_valid_unclassified_attempts, canonical_complete_zero_attempts,
			canonical_complete_prompt_tokens, canonical_complete_cache_read_tokens,
			canonical_complete_output_tokens, canonical_total_tokens, canonical_input_tokens,
			canonical_uncached_tokens, canonical_cache_read_tokens, canonical_cache_write_tokens,
			canonical_output_tokens, canonical_non_reasoning_tokens, canonical_reasoning_tokens,
			canonical_unclassified_tokens, total_latency_ms, latency_sample_count,
			last_event_at, created_at, updated_at
		)
		SELECT
			rollup_bucket_start,
			rollup_provider,
			rollup_model,
			rollup_auth_type,
			rollup_auth_index,
			rollup_api_key_identity,
			COUNT(*) AS request_count,
			SUM(CASE WHEN failed THEN 0 ELSE 1 END) AS success_count,
			SUM(CASE WHEN failed THEN 1 ELSE 0 END) AS failure_count,
			SUM(CASE WHEN accounting_state = 'absent' THEN 1 ELSE 0 END) AS accounting_absent_attempts,
			SUM(CASE WHEN accounting_state = 'valid' THEN 1 ELSE 0 END) AS accounting_valid_attempts,
			SUM(CASE WHEN accounting_state = 'valid' AND token_quality = 'complete' THEN 1 ELSE 0 END) AS accounting_valid_complete_attempts,
			SUM(CASE WHEN accounting_state = 'valid' AND token_quality = 'inconsistent' THEN 1 ELSE 0 END) AS accounting_valid_inconsistent_attempts,
			SUM(CASE WHEN accounting_state = 'valid' AND token_quality = 'unclassified' THEN 1 ELSE 0 END) AS accounting_valid_unclassified_attempts,
			SUM(CASE WHEN accounting_state = 'valid' AND token_quality = 'complete'
				AND COALESCE(canonical_uncached_tokens, 0) = 0
				AND COALESCE(canonical_cache_read_tokens, 0) = 0
				AND COALESCE(canonical_cache_write_tokens, 0) = 0
				AND COALESCE(canonical_output_tokens, 0) = 0 THEN 1 ELSE 0 END) AS canonical_complete_zero_attempts,
			SUM(CASE WHEN accounting_state = 'valid' AND token_quality = 'complete'
				THEN COALESCE(canonical_uncached_tokens, 0) + COALESCE(canonical_cache_write_tokens, 0) ELSE 0 END) AS canonical_complete_prompt_tokens,
			SUM(CASE WHEN accounting_state = 'valid' AND token_quality = 'complete'
				THEN COALESCE(canonical_cache_read_tokens, 0) ELSE 0 END) AS canonical_complete_cache_read_tokens,
			SUM(CASE WHEN accounting_state = 'valid' AND token_quality = 'complete'
				THEN COALESCE(canonical_output_tokens, 0) ELSE 0 END) AS canonical_complete_output_tokens,
			SUM(CASE WHEN accounting_state = 'valid' THEN COALESCE(canonical_total_tokens, 0) ELSE 0 END) AS canonical_total_tokens,
			SUM(CASE WHEN accounting_state = 'valid' THEN COALESCE(canonical_input_tokens, 0) ELSE 0 END) AS canonical_input_tokens,
			SUM(CASE WHEN accounting_state = 'valid' THEN COALESCE(canonical_uncached_tokens, 0) ELSE 0 END) AS canonical_uncached_tokens,
			SUM(CASE WHEN accounting_state = 'valid' THEN COALESCE(canonical_cache_read_tokens, 0) ELSE 0 END) AS canonical_cache_read_tokens,
			SUM(CASE WHEN accounting_state = 'valid' THEN COALESCE(canonical_cache_write_tokens, 0) ELSE 0 END) AS canonical_cache_write_tokens,
			SUM(CASE WHEN accounting_state = 'valid' THEN COALESCE(canonical_output_tokens, 0) ELSE 0 END) AS canonical_output_tokens,
			SUM(CASE WHEN accounting_state = 'valid' THEN COALESCE(canonical_non_reasoning_tokens, 0) ELSE 0 END) AS canonical_non_reasoning_tokens,
			SUM(CASE WHEN accounting_state = 'valid' THEN COALESCE(canonical_reasoning_tokens, 0) ELSE 0 END) AS canonical_reasoning_tokens,
			SUM(CASE WHEN accounting_state = 'valid' THEN COALESCE(canonical_unclassified_tokens, 0) ELSE 0 END) AS canonical_unclassified_tokens,
			SUM(CASE WHEN latency_ms > 0 THEN latency_ms ELSE 0 END) AS total_latency_ms,
			SUM(CASE WHEN latency_ms > 0 THEN 1 ELSE 0 END) AS latency_sample_count,
			MAX(timestamp) AS last_event_at,
			@now AS created_at,
			@now AS updated_at
		FROM (
			SELECT
				strftime('%Y-%m-%d %H:00:00+00:00', timestamp) AS rollup_bucket_start,
				TRIM(provider, @trim_space) AS rollup_provider,
				TRIM(model, @trim_space) AS rollup_model,
				TRIM(auth_type, @trim_space) AS rollup_auth_type,
				TRIM(auth_index, @trim_space) AS rollup_auth_index,
				CASE
					WHEN substr(TRIM(api_group_key, @trim_space), 1, 3) = 'sk-' THEN TRIM(api_group_key, @trim_space)
					WHEN TRIM(auth_type, @trim_space) = 'apikey' AND substr(TRIM(source, @trim_space), 1, 3) = 'sk-' THEN TRIM(source, @trim_space)
					ELSE ''
				END AS rollup_api_key_identity,
				usage_events.*
			FROM usage_events
			WHERE timestamp >= @start AND timestamp < @end
		) AS normalized_events
		GROUP BY rollup_bucket_start, rollup_provider, rollup_model, rollup_auth_type, rollup_auth_index, rollup_api_key_identity
	`,
		sql.Named("trim_space", sqliteTrimSpaceCharacters),
		sql.Named("now", now),
		sql.Named("start", start),
		sql.Named("end", endExclusive),
	)
	if result.Error != nil {
		return fmt.Errorf("create hourly usage rollups from raw events: %w", result.Error)
	}
	return nil
}

// incrementUsageRollupsForEvents adds only newly inserted raw attempts to the
// derived hourly rows. Callers invoke it in the same transaction that inserted
// those attempts so raw data and rollups commit together.
func incrementUsageRollupsForEvents(db *gorm.DB, events []entities.UsageEvent) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if len(events) == 0 {
		return nil
	}
	rollups := buildUsageRollupsForEvents(events)
	if len(rollups) == 0 {
		return nil
	}
	assignments := map[string]any{
		"request_count":                          gorm.Expr("request_count + excluded.request_count"),
		"success_count":                          gorm.Expr("success_count + excluded.success_count"),
		"failure_count":                          gorm.Expr("failure_count + excluded.failure_count"),
		"accounting_absent_attempts":             gorm.Expr("accounting_absent_attempts + excluded.accounting_absent_attempts"),
		"accounting_valid_attempts":              gorm.Expr("accounting_valid_attempts + excluded.accounting_valid_attempts"),
		"accounting_valid_complete_attempts":     gorm.Expr("accounting_valid_complete_attempts + excluded.accounting_valid_complete_attempts"),
		"accounting_valid_inconsistent_attempts": gorm.Expr("accounting_valid_inconsistent_attempts + excluded.accounting_valid_inconsistent_attempts"),
		"accounting_valid_unclassified_attempts": gorm.Expr("accounting_valid_unclassified_attempts + excluded.accounting_valid_unclassified_attempts"),
		"canonical_complete_zero_attempts":       gorm.Expr("canonical_complete_zero_attempts + excluded.canonical_complete_zero_attempts"),
		"canonical_complete_prompt_tokens":       gorm.Expr("canonical_complete_prompt_tokens + excluded.canonical_complete_prompt_tokens"),
		"canonical_complete_cache_read_tokens":   gorm.Expr("canonical_complete_cache_read_tokens + excluded.canonical_complete_cache_read_tokens"),
		"canonical_complete_output_tokens":       gorm.Expr("canonical_complete_output_tokens + excluded.canonical_complete_output_tokens"),
		"canonical_total_tokens":                 gorm.Expr("canonical_total_tokens + excluded.canonical_total_tokens"),
		"canonical_input_tokens":                 gorm.Expr("canonical_input_tokens + excluded.canonical_input_tokens"),
		"canonical_uncached_tokens":              gorm.Expr("canonical_uncached_tokens + excluded.canonical_uncached_tokens"),
		"canonical_cache_read_tokens":            gorm.Expr("canonical_cache_read_tokens + excluded.canonical_cache_read_tokens"),
		"canonical_cache_write_tokens":           gorm.Expr("canonical_cache_write_tokens + excluded.canonical_cache_write_tokens"),
		"canonical_output_tokens":                gorm.Expr("canonical_output_tokens + excluded.canonical_output_tokens"),
		"canonical_non_reasoning_tokens":         gorm.Expr("canonical_non_reasoning_tokens + excluded.canonical_non_reasoning_tokens"),
		"canonical_reasoning_tokens":             gorm.Expr("canonical_reasoning_tokens + excluded.canonical_reasoning_tokens"),
		"canonical_unclassified_tokens":          gorm.Expr("canonical_unclassified_tokens + excluded.canonical_unclassified_tokens"),
		"total_latency_ms":                       gorm.Expr("total_latency_ms + excluded.total_latency_ms"),
		"latency_sample_count":                   gorm.Expr("latency_sample_count + excluded.latency_sample_count"),
		"last_event_at":                          gorm.Expr("MAX(last_event_at, excluded.last_event_at)"),
		"updated_at":                             gorm.Expr("excluded.updated_at"),
	}
	conflict := clause.OnConflict{
		Columns: []clause.Column{
			{Name: "bucket_start"}, {Name: "provider"}, {Name: "model"},
			{Name: "auth_type"}, {Name: "auth_index"}, {Name: "api_key_identity"},
		},
		DoUpdates: clause.Assignments(assignments),
	}
	if err := db.Clauses(conflict).CreateInBatches(&rollups, insertBatchSize(entities.UsageRollupHourly{})).Error; err != nil {
		return fmt.Errorf("increment hourly usage rollups: %w", err)
	}
	return nil
}

func rebuildUsageRollupsForBuckets(db *gorm.DB, bucketList []time.Time, events []entities.UsageEvent) error {
	if len(bucketList) == 0 {
		return nil
	}
	rollups := buildUsageRollupsForBuckets(bucketList, events)
	if err := db.Where("bucket_start IN ?", bucketList).Delete(&entities.UsageRollupHourly{}).Error; err != nil {
		return fmt.Errorf("delete hourly usage rollups: %w", err)
	}
	if len(rollups) == 0 {
		return nil
	}
	if err := db.Create(&rollups).Error; err != nil {
		return fmt.Errorf("create hourly usage rollups: %w", err)
	}
	return nil
}

func buildUsageRollupsForBuckets(bucketList []time.Time, events []entities.UsageEvent) []entities.UsageRollupHourly {
	buckets := make(map[time.Time]struct{}, len(bucketList))
	for _, bucket := range bucketList {
		buckets[bucket.UTC().Truncate(time.Hour)] = struct{}{}
	}
	rollupsByKey := map[hourlyUsageRollupKey]*entities.UsageRollupHourly{}
	for _, event := range events {
		bucket := event.Timestamp.UTC().Truncate(time.Hour)
		if _, ok := buckets[bucket]; !ok {
			continue
		}
		key := hourlyUsageRollupKey{
			BucketStart:    bucket,
			Provider:       strings.TrimSpace(event.Provider),
			Model:          strings.TrimSpace(event.Model),
			AuthType:       strings.TrimSpace(event.AuthType),
			AuthIndex:      strings.TrimSpace(event.AuthIndex),
			APIKeyIdentity: usageEventAPIKeyIdentity(event),
		}
		rollup := rollupsByKey[key]
		if rollup == nil {
			rollup = &entities.UsageRollupHourly{
				BucketStart:    key.BucketStart,
				Provider:       key.Provider,
				Model:          key.Model,
				AuthType:       key.AuthType,
				AuthIndex:      key.AuthIndex,
				APIKeyIdentity: key.APIKeyIdentity,
			}
			rollupsByKey[key] = rollup
		}
		applyUsageEventToHourlyRollup(rollup, event)
	}

	if len(rollupsByKey) == 0 {
		return nil
	}
	rollups := make([]entities.UsageRollupHourly, 0, len(rollupsByKey))
	for _, rollup := range rollupsByKey {
		rollups = append(rollups, *rollup)
	}
	return rollups
}

func buildUsageRollupsForEvents(events []entities.UsageEvent) []entities.UsageRollupHourly {
	if len(events) == 0 {
		return nil
	}
	buckets := make(map[time.Time]struct{})
	for _, event := range events {
		buckets[event.Timestamp.UTC().Truncate(time.Hour)] = struct{}{}
	}
	bucketList := make([]time.Time, 0, len(buckets))
	for bucket := range buckets {
		bucketList = append(bucketList, bucket)
	}
	return buildUsageRollupsForBuckets(bucketList, events)
}

func hourlyBucketRange(start time.Time, end time.Time) []time.Time {
	if end.Before(start) {
		return nil
	}
	buckets := make([]time.Time, 0, int(end.Sub(start)/time.Hour)+1)
	for bucket := start.UTC().Truncate(time.Hour); !bucket.After(end); bucket = bucket.Add(time.Hour) {
		buckets = append(buckets, bucket)
	}
	return buckets
}

func applyUsageEventToHourlyRollup(rollup *entities.UsageRollupHourly, event entities.UsageEvent) {
	rollup.RequestCount++
	if event.Failed {
		rollup.FailureCount++
	} else {
		rollup.SuccessCount++
	}
	applyUsageAccountingToHourlyRollup(rollup, event)
	if event.LatencyMS > 0 {
		rollup.TotalLatencyMS += event.LatencyMS
		rollup.LatencySampleCount++
	}
	eventTime := event.Timestamp.UTC()
	if rollup.LastEventAt.IsZero() || eventTime.After(rollup.LastEventAt) {
		rollup.LastEventAt = eventTime
	}
}

func applyUsageAccountingToHourlyRollup(rollup *entities.UsageRollupHourly, event entities.UsageEvent) {
	switch event.AccountingState {
	case AccountingAbsent:
		rollup.AccountingAbsentAttempts++
	case AccountingValid:
		rollup.AccountingValidAttempts++
		switch optionalStringValue(event.TokenQuality) {
		case "complete":
			rollup.AccountingValidCompleteAttempts++
			promptTokens := optionalInt64Value(event.CanonicalUncachedTokens) + optionalInt64Value(event.CanonicalCacheWriteTokens)
			cacheReadTokens := optionalInt64Value(event.CanonicalCacheReadTokens)
			outputTokens := optionalInt64Value(event.CanonicalOutputTokens)
			rollup.CanonicalCompletePromptTokens += promptTokens
			rollup.CanonicalCompleteCacheReadTokens += cacheReadTokens
			rollup.CanonicalCompleteOutputTokens += outputTokens
			if promptTokens == 0 && cacheReadTokens == 0 && outputTokens == 0 {
				rollup.CanonicalCompleteZeroAttempts++
			}
		case "inconsistent":
			rollup.AccountingValidInconsistentAttempts++
		case "unclassified":
			rollup.AccountingValidUnclassifiedAttempts++
		}
		rollup.CanonicalTotalTokens += optionalInt64Value(event.CanonicalTotalTokens)
		rollup.CanonicalInputTokens += optionalInt64Value(event.CanonicalInputTokens)
		rollup.CanonicalUncachedTokens += optionalInt64Value(event.CanonicalUncachedTokens)
		rollup.CanonicalCacheReadTokens += optionalInt64Value(event.CanonicalCacheReadTokens)
		rollup.CanonicalCacheWriteTokens += optionalInt64Value(event.CanonicalCacheWriteTokens)
		rollup.CanonicalOutputTokens += optionalInt64Value(event.CanonicalOutputTokens)
		rollup.CanonicalNonReasoningTokens += optionalInt64Value(event.CanonicalNonReasoningTokens)
		rollup.CanonicalReasoningTokens += optionalInt64Value(event.CanonicalReasoningTokens)
		rollup.CanonicalUnclassifiedTokens += optionalInt64Value(event.CanonicalUnclassifiedTokens)
	}
}

func optionalInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
