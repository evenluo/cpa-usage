package service

import (
	"context"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

// CanonicalEventLookup 是 CanonicalEventKeyAssigner 对 usage event 存储的窄查询口：
// 只需要按 canonical 维度查已持久化事件，以及查 event key 是否已被 processed redis usage inbox 行引用。
type CanonicalEventLookup interface {
	// FindEquivalentUsageEvents 返回与给定 canonical 维度输入等价的已持久化事件。
	FindEquivalentUsageEvents(ctx context.Context, inputs []repository.UsageEventCanonicalLookupInput) ([]repository.UsageEventCanonicalLookupRow, error)
	// ListProcessedEventKeyReferences 返回 eventKeys 中已被 processed redis usage inbox 行引用的子集。
	ListProcessedEventKeyReferences(ctx context.Context, eventKeys []string) ([]string, error)
}

type repositoryCanonicalEventLookup struct {
	db *gorm.DB
}

func NewRepositoryCanonicalEventLookup(db *gorm.DB) CanonicalEventLookup {
	return repositoryCanonicalEventLookup{db: db}
}

func (l repositoryCanonicalEventLookup) FindEquivalentUsageEvents(_ context.Context, inputs []repository.UsageEventCanonicalLookupInput) ([]repository.UsageEventCanonicalLookupRow, error) {
	return repository.FindEquivalentUsageEvents(l.db, inputs)
}

func (l repositoryCanonicalEventLookup) ListProcessedEventKeyReferences(_ context.Context, eventKeys []string) ([]string, error) {
	return repository.ListProcessedRedisUsageInboxEventKeys(l.db, eventKeys)
}

// CanonicalEventKeyAssigner 负责为 incoming usage event 分配 canonical event key（Request Evidence 去重）：
// 无 request_id 的事件与已持久化 canonical event 等价、且该事件 key 未被 processed inbox 行引用时，复用其 key；
// 同批内等价事件按序对齐，key building、存储查询、引用检查与批内去重都收敛在这一个模块内。
type CanonicalEventKeyAssigner struct {
	lookup CanonicalEventLookup
}

func NewCanonicalEventKeyAssigner(lookup CanonicalEventLookup) CanonicalEventKeyAssigner {
	return CanonicalEventKeyAssigner{lookup: lookup}
}

// Assign 先把事件时间归一化为 UTC，再为无 request_id 的事件对齐 canonical event key；
// 带 request_id 的事件始终保留自身 key，并为同批等价事件提供对齐目标。
func (a CanonicalEventKeyAssigner) Assign(ctx context.Context, events []entities.UsageEvent) error {
	if len(events) == 0 {
		return nil
	}
	for i := range events {
		events[i].Timestamp = events[i].Timestamp.UTC()
	}
	existing, err := a.loadExistingCanonicalKeys(ctx, events)
	if err != nil {
		return err
	}
	canonicalEventKeys := make(map[string]string, len(events))
	consumedCanonicalKeys := make(map[string]struct{}, len(events))
	for i := range events {
		canonicalKey := canonicalUsageEventKey(events[i])
		incomingKey := strings.TrimSpace(events[i].EventKey)
		if strings.TrimSpace(events[i].RequestID) != "" {
			canonicalEventKeys[canonicalKey] = incomingKey
			continue
		}
		if existingKey := canonicalEventKeys[canonicalKey]; existingKey != "" {
			if incomingKey == canonicalKey {
				events[i].EventKey = existingKey
			} else if existingKey == canonicalKey {
				if _, consumed := consumedCanonicalKeys[canonicalKey]; !consumed {
					events[i].EventKey = existingKey
					consumedCanonicalKeys[canonicalKey] = struct{}{}
				}
			}
			continue
		}

		existingKey := existing[canonicalKey]
		if existingKey.eventKey == "" {
			canonicalEventKeys[canonicalKey] = incomingKey
			continue
		}
		if incomingKey == canonicalKey {
			events[i].EventKey = existingKey.eventKey
		} else if existingKey.eventKey == canonicalKey && !existingKey.referenced {
			events[i].EventKey = existingKey.eventKey
			consumedCanonicalKeys[canonicalKey] = struct{}{}
		}
		canonicalEventKeys[canonicalKey] = existingKey.eventKey
	}
	return nil
}

// existingCanonicalKey 是已持久化 canonical event 的对齐信息：eventKey 为 0 值表示存储中不存在等价事件。
type existingCanonicalKey struct {
	eventKey   string
	referenced bool
}

func (a CanonicalEventKeyAssigner) loadExistingCanonicalKeys(ctx context.Context, events []entities.UsageEvent) (map[string]existingCanonicalKey, error) {
	inputs := make([]repository.UsageEventCanonicalLookupInput, 0, len(events))
	seen := make(map[string]struct{}, len(events))
	for _, event := range events {
		if strings.TrimSpace(event.RequestID) != "" {
			continue
		}
		canonicalKey := canonicalUsageEventKey(event)
		if _, ok := seen[canonicalKey]; ok {
			continue
		}
		seen[canonicalKey] = struct{}{}
		inputs = append(inputs, repository.UsageEventCanonicalLookupInput{
			APIGroupKey:     event.APIGroupKey,
			Model:           event.Model,
			Timestamp:       event.Timestamp,
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
	existing := make(map[string]existingCanonicalKey, len(inputs))
	if len(inputs) == 0 {
		return existing, nil
	}
	rows, err := a.lookup.FindEquivalentUsageEvents(ctx, inputs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		key := canonicalLookupRowEventKey(row)
		if _, ok := existing[key]; ok {
			continue
		}
		existing[key] = existingCanonicalKey{eventKey: strings.TrimSpace(row.EventKey)}
	}
	return a.markReferencedEventKeys(ctx, existing)
}

func (a CanonicalEventKeyAssigner) markReferencedEventKeys(ctx context.Context, existing map[string]existingCanonicalKey) (map[string]existingCanonicalKey, error) {
	eventKeys := make([]string, 0, len(existing))
	seen := make(map[string]struct{}, len(existing))
	for _, value := range existing {
		if value.eventKey == "" {
			continue
		}
		if _, ok := seen[value.eventKey]; ok {
			continue
		}
		seen[value.eventKey] = struct{}{}
		eventKeys = append(eventKeys, value.eventKey)
	}
	if len(eventKeys) == 0 {
		return existing, nil
	}
	referencedKeys, err := a.lookup.ListProcessedEventKeyReferences(ctx, eventKeys)
	if err != nil {
		return nil, err
	}
	referenced := make(map[string]bool, len(referencedKeys))
	for _, key := range referencedKeys {
		referenced[strings.TrimSpace(key)] = true
	}
	for key, value := range existing {
		value.referenced = referenced[value.eventKey]
		existing[key] = value
	}
	return existing, nil
}

func canonicalUsageEventKey(event entities.UsageEvent) string {
	return BuildEventKey(
		event.APIGroupKey,
		event.Model,
		event.Timestamp,
		event.Source,
		event.AuthIndex,
		event.Failed,
		repodto.TokenStats{
			InputTokens:     event.InputTokens,
			OutputTokens:    event.OutputTokens,
			ReasoningTokens: event.ReasoningTokens,
			CachedTokens:    event.CachedTokens,
			TotalTokens:     event.TotalTokens,
		},
	)
}

func canonicalLookupRowEventKey(row repository.UsageEventCanonicalLookupRow) string {
	return BuildEventKey(
		row.APIGroupKey,
		row.Model,
		row.Timestamp,
		row.Source,
		row.AuthIndex,
		row.Failed,
		repodto.TokenStats{
			InputTokens:     row.InputTokens,
			OutputTokens:    row.OutputTokens,
			ReasoningTokens: row.ReasoningTokens,
			CachedTokens:    row.CachedTokens,
			TotalTokens:     row.TotalTokens,
		},
	)
}
