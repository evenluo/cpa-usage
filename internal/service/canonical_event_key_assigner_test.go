package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
)

type fakeCanonicalEventLookup struct {
	rows       []repository.UsageEventCanonicalLookupRow
	referenced map[string]bool
	findErr    error
	listErr    error

	findCalls [][]repository.UsageEventCanonicalLookupInput
	listCalls [][]string
}

func (f *fakeCanonicalEventLookup) FindEquivalentUsageEvents(_ context.Context, inputs []repository.UsageEventCanonicalLookupInput) ([]repository.UsageEventCanonicalLookupRow, error) {
	f.findCalls = append(f.findCalls, inputs)
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.rows, nil
}

func (f *fakeCanonicalEventLookup) ListProcessedEventKeyReferences(_ context.Context, eventKeys []string) ([]string, error) {
	f.listCalls = append(f.listCalls, eventKeys)
	if f.listErr != nil {
		return nil, f.listErr
	}
	referenced := make([]string, 0, len(eventKeys))
	for _, key := range eventKeys {
		if f.referenced[key] {
			referenced = append(referenced, key)
		}
	}
	return referenced, nil
}

func (f *fakeCanonicalEventLookup) findCallCount() int {
	return len(f.findCalls)
}

func (f *fakeCanonicalEventLookup) listCallCount() int {
	return len(f.listCalls)
}

var assignerTestTimestamp = time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC)

func assignerTestEvent(requestID, eventKey string) entities.UsageEvent {
	event := entities.UsageEvent{
		EventKey:     eventKey,
		APIGroupKey:  "api-key-a",
		Model:        "claude-sonnet",
		Timestamp:    assignerTestTimestamp,
		Source:       "source-a",
		AuthIndex:    "1",
		InputTokens:  10,
		OutputTokens: 20,
		TotalTokens:  30,
		RequestID:    requestID,
	}
	if eventKey == "" {
		eventKey = requestID
	}
	if eventKey == "" {
		eventKey = canonicalUsageEventKey(event)
	}
	event.EventKey = eventKey
	return event
}

func assignerLookupRow(event entities.UsageEvent, eventKey string) repository.UsageEventCanonicalLookupRow {
	return repository.UsageEventCanonicalLookupRow{
		EventKey:        eventKey,
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
	}
}

func assignerEventKeys(events []entities.UsageEvent) []string {
	keys := make([]string, 0, len(events))
	for _, event := range events {
		keys = append(keys, event.EventKey)
	}
	return keys
}

func assertEventKeys(t *testing.T, events []entities.UsageEvent, expected []string) {
	t.Helper()
	actual := assignerEventKeys(events)
	if len(actual) != len(expected) {
		t.Fatalf("expected %d event keys, got %v", len(expected), actual)
	}
	for i := range expected {
		if actual[i] != expected[i] {
			t.Fatalf("event %d: expected key %q, got %q (all keys: %v)", i, expected[i], actual[i], actual)
		}
	}
}

func TestCanonicalEventKeyAssignerAssign(t *testing.T) {
	canonicalEvent := assignerTestEvent("", "")
	canonicalKey := canonicalEvent.EventKey
	otherEvent := assignerTestEvent("", "")
	otherEvent.AuthIndex = "2"
	otherEvent.EventKey = canonicalUsageEventKey(otherEvent)

	tests := []struct {
		name            string
		events          []entities.UsageEvent
		rows            []repository.UsageEventCanonicalLookupRow
		referenced      map[string]bool
		expectedKeys    []string
		expectedFindIn  int // 期望 find 收到的去重后 input 数；-1 表示不应调用 find
		expectedListCnt int
	}{
		{
			name:            "request id present keeps own key and skips lookup",
			events:          []entities.UsageEvent{assignerTestEvent("req-1", "")},
			expectedKeys:    []string{"req-1"},
			expectedFindIn:  -1,
			expectedListCnt: 0,
		},
		{
			name:            "no request id without persisted equivalent keeps canonical key",
			events:          []entities.UsageEvent{assignerTestEvent("", "")},
			expectedKeys:    []string{canonicalKey},
			expectedFindIn:  1,
			expectedListCnt: 0,
		},
		{
			name:            "no request id adopts persisted request-id event key when unreferenced",
			events:          []entities.UsageEvent{assignerTestEvent("", "")},
			rows:            []repository.UsageEventCanonicalLookupRow{assignerLookupRow(canonicalEvent, "req-persisted")},
			expectedKeys:    []string{"req-persisted"},
			expectedFindIn:  1,
			expectedListCnt: 1,
		},
		{
			// incomingKey == canonicalKey 分支不看 referenced：已持久化 request-id 事件即使有 processed inbox 引用，等价事件仍对齐到它的 key。
			name:            "no request id adopts persisted request-id event key even when referenced",
			events:          []entities.UsageEvent{assignerTestEvent("", "")},
			rows:            []repository.UsageEventCanonicalLookupRow{assignerLookupRow(canonicalEvent, "req-persisted")},
			referenced:      map[string]bool{"req-persisted": true},
			expectedKeys:    []string{"req-persisted"},
			expectedFindIn:  1,
			expectedListCnt: 1,
		},
		{
			name:            "custom key without request id adopts persisted canonical key when unreferenced",
			events:          []entities.UsageEvent{assignerTestEvent("", "custom-key-1")},
			rows:            []repository.UsageEventCanonicalLookupRow{assignerLookupRow(canonicalEvent, canonicalKey)},
			expectedKeys:    []string{canonicalKey},
			expectedFindIn:  1,
			expectedListCnt: 1,
		},
		{
			name:            "custom key without request id is kept when persisted canonical key is referenced",
			events:          []entities.UsageEvent{assignerTestEvent("", "custom-key-1")},
			rows:            []repository.UsageEventCanonicalLookupRow{assignerLookupRow(canonicalEvent, canonicalKey)},
			referenced:      map[string]bool{canonicalKey: true},
			expectedKeys:    []string{"custom-key-1"},
			expectedFindIn:  1,
			expectedListCnt: 1,
		},
		{
			// 同一 canonical key 只被一个 incoming event 消费：第二个自定义 key 事件保留自身 key。
			name: "consumed persisted canonical key is not adopted twice in one batch",
			events: []entities.UsageEvent{
				assignerTestEvent("", "custom-key-1"),
				assignerTestEvent("", "custom-key-2"),
			},
			rows:            []repository.UsageEventCanonicalLookupRow{assignerLookupRow(canonicalEvent, canonicalKey)},
			expectedKeys:    []string{canonicalKey, "custom-key-2"},
			expectedFindIn:  1,
			expectedListCnt: 1,
		},
		{
			name: "request id event in batch becomes alignment target for equivalent events",
			events: []entities.UsageEvent{
				assignerTestEvent("req-batch", ""),
				assignerTestEvent("", ""),
			},
			expectedKeys:    []string{"req-batch", "req-batch"},
			expectedFindIn:  1,
			expectedListCnt: 0,
		},
		{
			name: "first no-request-id event without persisted equivalent seeds in-batch canonical key",
			events: []entities.UsageEvent{
				assignerTestEvent("", ""),
				assignerTestEvent("", ""),
			},
			expectedKeys:    []string{canonicalKey, canonicalKey},
			expectedFindIn:  1,
			expectedListCnt: 0,
		},
		{
			name: "in-batch equivalent events share the adopted persisted key",
			events: []entities.UsageEvent{
				assignerTestEvent("", ""),
				assignerTestEvent("", ""),
			},
			rows:            []repository.UsageEventCanonicalLookupRow{assignerLookupRow(canonicalEvent, "req-persisted")},
			expectedKeys:    []string{"req-persisted", "req-persisted"},
			expectedFindIn:  1,
			expectedListCnt: 1,
		},
		{
			name: "distinct canonical keys are aligned independently",
			events: []entities.UsageEvent{
				assignerTestEvent("", ""),
				otherEvent,
			},
			rows:            []repository.UsageEventCanonicalLookupRow{assignerLookupRow(canonicalEvent, "req-persisted")},
			expectedKeys:    []string{"req-persisted", otherEvent.EventKey},
			expectedFindIn:  2,
			expectedListCnt: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := &fakeCanonicalEventLookup{rows: tt.rows, referenced: tt.referenced}
			assigner := NewCanonicalEventKeyAssigner(lookup)

			if err := assigner.Assign(context.Background(), tt.events); err != nil {
				t.Fatalf("Assign returned error: %v", err)
			}
			assertEventKeys(t, tt.events, tt.expectedKeys)

			if tt.expectedFindIn < 0 {
				if lookup.findCallCount() != 0 {
					t.Fatalf("expected no find calls, got %d", lookup.findCallCount())
				}
			} else {
				if lookup.findCallCount() != 1 {
					t.Fatalf("expected 1 find call, got %d", lookup.findCallCount())
				}
				if got := len(lookup.findCalls[0]); got != tt.expectedFindIn {
					t.Fatalf("expected %d deduped lookup inputs, got %d", tt.expectedFindIn, got)
				}
			}
			if lookup.listCallCount() != tt.expectedListCnt {
				t.Fatalf("expected %d list calls, got %d", tt.expectedListCnt, lookup.listCallCount())
			}
		})
	}
}

func TestCanonicalEventKeyAssignerAssignEmptyBatchSkipsLookup(t *testing.T) {
	lookup := &fakeCanonicalEventLookup{}
	assigner := NewCanonicalEventKeyAssigner(lookup)

	if err := assigner.Assign(context.Background(), nil); err != nil {
		t.Fatalf("Assign returned error: %v", err)
	}
	if lookup.findCallCount() != 0 || lookup.listCallCount() != 0 {
		t.Fatalf("expected no lookup calls, got find=%d list=%d", lookup.findCallCount(), lookup.listCallCount())
	}
}

func TestCanonicalEventKeyAssignerAssignNormalizesTimestampsToUTC(t *testing.T) {
	lookup := &fakeCanonicalEventLookup{}
	assigner := NewCanonicalEventKeyAssigner(lookup)
	event := assignerTestEvent("", "")
	event.Timestamp = assignerTestTimestamp.In(time.FixedZone("UTC+8", 8*3600))
	events := []entities.UsageEvent{event}

	if err := assigner.Assign(context.Background(), events); err != nil {
		t.Fatalf("Assign returned error: %v", err)
	}
	if events[0].Timestamp.Location() != time.UTC {
		t.Fatalf("expected UTC timestamp, got %s", events[0].Timestamp)
	}
	if !events[0].Timestamp.Equal(assignerTestTimestamp) {
		t.Fatalf("expected instant to be preserved, got %s", events[0].Timestamp)
	}
}

func TestCanonicalEventKeyAssignerAssignPropagatesLookupErrors(t *testing.T) {
	tests := []struct {
		name     string
		lookup   *fakeCanonicalEventLookup
		expected string
	}{
		{
			name:     "find equivalent events error",
			lookup:   &fakeCanonicalEventLookup{findErr: errors.New("find failed")},
			expected: "find failed",
		},
		{
			name: "list processed references error",
			lookup: &fakeCanonicalEventLookup{
				rows:    []repository.UsageEventCanonicalLookupRow{assignerLookupRow(assignerTestEvent("", ""), "req-persisted")},
				listErr: errors.New("list failed"),
			},
			expected: "list failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assigner := NewCanonicalEventKeyAssigner(tt.lookup)
			err := assigner.Assign(context.Background(), []entities.UsageEvent{assignerTestEvent("", "")})
			if err == nil || err.Error() != tt.expected {
				t.Fatalf("expected error %q, got %v", tt.expected, err)
			}
		})
	}
}
