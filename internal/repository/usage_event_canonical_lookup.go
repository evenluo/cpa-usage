package repository

import (
	"fmt"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

type UsageEventCanonicalLookupInput struct {
	APIGroupKey     string
	Model           string
	Timestamp       time.Time
	Source          string
	AuthIndex       string
	Failed          bool
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
}

type UsageEventCanonicalLookupRow struct {
	EventKey        string
	APIGroupKey     string
	Model           string
	Timestamp       time.Time
	Source          string
	AuthIndex       string
	Failed          bool
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
}

func FindEquivalentUsageEvents(db *gorm.DB, inputs []UsageEventCanonicalLookupInput) ([]UsageEventCanonicalLookupRow, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	const columnsPerLookupClause = 11
	maxClauses := sqliteVariableLimit / columnsPerLookupClause
	rows := make([]UsageEventCanonicalLookupRow, 0, len(inputs))
	for start := 0; start < len(inputs); start += maxClauses {
		end := min(start+maxClauses, len(inputs))
		clauses := make([]string, 0, end-start)
		args := make([]any, 0, (end-start)*columnsPerLookupClause)
		for _, input := range inputs[start:end] {
			clauses = append(clauses, "(TRIM(api_group_key) = ? AND TRIM(model) = ? AND timestamp = ? AND TRIM(source) = ? AND TRIM(auth_index) = ? AND failed = ? AND input_tokens = ? AND output_tokens = ? AND reasoning_tokens = ? AND cached_tokens = ? AND total_tokens = ?)")
			args = append(args,
				strings.TrimSpace(input.APIGroupKey),
				strings.TrimSpace(input.Model),
				input.Timestamp.UTC(),
				strings.TrimSpace(input.Source),
				strings.TrimSpace(input.AuthIndex),
				input.Failed,
				input.InputTokens,
				input.OutputTokens,
				input.ReasoningTokens,
				input.CachedTokens,
				input.TotalTokens,
			)
		}
		var chunk []entities.UsageEvent
		if err := db.
			Select("id", "event_key", "api_group_key", "model", "timestamp", "source", "auth_index", "failed", "input_tokens", "output_tokens", "reasoning_tokens", "cached_tokens", "total_tokens").
			Where(strings.Join(clauses, " OR "), args...).
			Order("id ASC").
			Find(&chunk).Error; err != nil {
			return nil, fmt.Errorf("find equivalent usage events: %w", err)
		}
		for _, event := range chunk {
			rows = append(rows, UsageEventCanonicalLookupRow{
				EventKey:        event.EventKey,
				APIGroupKey:     event.APIGroupKey,
				Model:           event.Model,
				Timestamp:       event.Timestamp.UTC(),
				Source:          event.Source,
				AuthIndex:       event.AuthIndex,
				Failed:          event.Failed,
				InputTokens:     event.InputTokens,
				OutputTokens:    event.OutputTokens,
				ReasoningTokens: event.ReasoningTokens,
				CachedTokens:    event.CachedTokens,
				TotalTokens:     event.TotalTokens,
			})
		}
	}
	return rows, nil
}
