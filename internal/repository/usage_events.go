package repository

import (
	"context"
	"fmt"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

// Request Event Log Tab：先按列表条件统计总数，再加载当前页和筛选项。
func ListUsageEventsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) (*dto.UsageEventsPageRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	db = db.WithContext(ctx)

	// 第一步：应用列表筛选，统计分页总数。
	baseQuery := queryUsageEvents(db)
	baseQuery = applyUsageEventListQuery(baseQuery, filter)

	var totalCount int64
	if err := baseQuery.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("count usage events: %w", err)
	}

	// 第二步：model 筛选项只跟随时间窗口，不跟随当前列表筛选。
	modelOptions, err := listUsageEventModelFilterOptions(db, filter)
	if err != nil {
		return nil, err
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = filter.Limit
	}
	if pageSize <= 0 {
		pageSize = dto.DefaultUsageEventsLimit
	}
	offset := filter.Offset
	if offset <= 0 {
		offset = (page - 1) * pageSize
	}
	if offset < 0 {
		offset = 0
	}

	query := applyUsageEventListQuery(db.Model(&entities.UsageEvent{}), filter)
	query = query.Order("timestamp DESC, id DESC").Limit(pageSize).Offset(offset)

	var events []entities.UsageEvent
	if err := query.Find(&events).Error; err != nil {
		return nil, fmt.Errorf("load usage events: %w", err)
	}

	rows := make([]dto.UsageEventRecord, 0, len(events))
	for _, event := range events {
		rows = append(rows, dto.UsageEventRecord{
			ID:              event.ID,
			Timestamp:       event.Timestamp.UTC(),
			APIGroupKey:     strings.TrimSpace(event.APIGroupKey),
			APIKeyIdentity:  usageEventAPIKeyIdentity(event),
			Model:           strings.TrimSpace(event.Model),
			AuthType:        strings.TrimSpace(event.AuthType),
			Provider:        strings.TrimSpace(event.Provider),
			Source:          strings.TrimSpace(event.Source),
			AuthIndex:       strings.TrimSpace(event.AuthIndex),
			Failed:          event.Failed,
			LatencyMS:       event.LatencyMS,
			TTFTMS:          event.TTFTMS,
			OutputTPS:       usageEventOutputTPS(event.OutputTokens, event.LatencyMS, event.TTFTMS),
			InputTokens:     event.InputTokens,
			OutputTokens:    event.OutputTokens,
			ReasoningTokens: event.ReasoningTokens,
			CachedTokens:    event.CachedTokens,
			TotalTokens:     event.TotalTokens,
		})
	}
	totalPages := 0
	if totalCount > 0 {
		totalPages = int((totalCount + int64(pageSize) - 1) / int64(pageSize))
	}
	return &dto.UsageEventsPageRecord{Events: rows, Models: modelOptions, TotalCount: totalCount, Page: page, PageSize: pageSize, TotalPages: totalPages}, nil
}

// usageEventOutputTPS 是 Request Evidence 的 Output TPS 规则：仅当 output tokens、总延迟与 TTFT 可用且自洽时才计算，否则保持 nil，不估算回退值。
func usageEventOutputTPS(outputTokens, latencyMS int64, ttftMS *int64) *float64 {
	if outputTokens <= 0 || ttftMS == nil || *ttftMS <= 0 || latencyMS <= *ttftMS {
		return nil
	}
	value := float64(outputTokens) * 1000 / float64(latencyMS-*ttftMS)
	return &value
}

func usageEventAPIKeyIdentity(event entities.UsageEvent) string {
	if apiGroupKey := strings.TrimSpace(event.APIGroupKey); strings.HasPrefix(apiGroupKey, "sk-") {
		return apiGroupKey
	}
	if strings.TrimSpace(event.AuthType) == "apikey" {
		if source := strings.TrimSpace(event.Source); strings.HasPrefix(source, "sk-") {
			return source
		}
	}
	return ""
}

// Request Event Log Filter Options：只按时间窗口收集 model 候选值。
func ListUsageEventFilterOptionsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) (*dto.UsageEventFilterOptionsRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	db = db.WithContext(ctx)
	models, err := listUsageEventModelFilterOptions(db, filter)
	if err != nil {
		return nil, err
	}
	return &dto.UsageEventFilterOptionsRecord{Models: models}, nil
}

func listUsageEventModelFilterOptions(db *gorm.DB, filter dto.UsageQueryFilter) ([]string, error) {
	// 第一步：model 候选值只来自 usage_events，并且只套用时间窗口。
	query := applyUsageEventFilterOptionsQuery(queryUsageEvents(db), filter)

	// 第二步：去重并排除空 model，保持下拉选项稳定排序。
	var values []string
	if err := query.Select("DISTINCT TRIM(model)").Where("TRIM(model) <> ''").Order("TRIM(model) ASC").Pluck("model", &values).Error; err != nil {
		return nil, fmt.Errorf("load usage event model filter options: %w", err)
	}
	return values, nil
}

func queryUsageEvents(db *gorm.DB) *gorm.DB {
	return db.Model(&entities.UsageEvent{})
}

// Request Event Log 筛选项第一步：应用时间窗口和 provider scope，不叠加当前列表筛选。
func applyUsageEventFilterOptionsQuery(query *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	return applyUsageProviderFilter(applyUsageQueryWindow(query, filter), filter)
}

// Request Event Log 列表第一步：在时间窗口和 provider scope 上叠加 model/source/auth_index/result。
func applyUsageEventListQuery(query *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	query = applyUsageQueryWindow(query, filter)
	query = applyUsageProviderFilter(query, filter)
	if model := strings.TrimSpace(filter.Model); model != "" {
		query = query.Where("TRIM(model) = ?", model)
	}
	if source := strings.TrimSpace(filter.Source); source != "" {
		if authIndex := strings.TrimSpace(filter.AuthIndex); authIndex != "" {
			// 第二步：API 层会把 Source 下拉转成 auth_index，这里兼容直接传 source 的仓储调用。
			query = query.Where("(TRIM(auth_index) = ? OR TRIM(source) = ?)", authIndex, source)
		} else {
			query = query.Where("TRIM(source) = ?", source)
		}
	} else if authIndex := strings.TrimSpace(filter.AuthIndex); authIndex != "" {
		query = query.Where("TRIM(auth_index) = ?", authIndex)
	}
	switch strings.TrimSpace(filter.Result) {
	case "success":
		query = query.Where("failed = ?", false)
	case "failed":
		query = query.Where("failed = ?", true)
	}
	return query
}
