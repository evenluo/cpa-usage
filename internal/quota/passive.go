package quota

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"cpa-usage/internal/cpa/dto/authfiles"
	"cpa-usage/internal/entities"
)

const maxPassiveSignalValueLength = 512

type PassiveQuotaSnapshot struct {
	Account *entities.PassiveQuotaObservation
	Models  []entities.PassiveModelQuotaObservation
}

// NormalizePassiveQuotaSnapshot is the single interpretation seam for CPA's
// allowlisted passive Claude/Codex quota signals. Unknown providers, keys and
// malformed values produce no fact rather than a zero-valued fact.
func NormalizePassiveQuotaSnapshot(provider string, account *authfiles.QuotaObservation, models map[string]authfiles.QuotaObservation) PassiveQuotaSnapshot {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "claude" && provider != "codex" {
		return PassiveQuotaSnapshot{}
	}

	snapshot := PassiveQuotaSnapshot{Account: normalizePassiveObservation(provider, "account", account)}
	modelNames := make([]string, 0, len(models))
	for model := range models {
		modelNames = append(modelNames, model)
	}
	sort.Strings(modelNames)
	for _, model := range modelNames {
		trimmedModel := strings.TrimSpace(model)
		if trimmedModel == "" || len(trimmedModel) > 200 || containsControl(trimmedModel) {
			continue
		}
		raw := models[model]
		observation := normalizePassiveObservation(provider, "model", &raw)
		if observation == nil {
			continue
		}
		snapshot.Models = append(snapshot.Models, entities.PassiveModelQuotaObservation{
			Model: trimmedModel, ObservedAt: observation.ObservedAt, ActiveLimit: observation.ActiveLimit, Quota: observation.Quota,
		})
	}
	return snapshot
}

func normalizePassiveObservation(provider, scope string, raw *authfiles.QuotaObservation) *entities.PassiveQuotaObservation {
	if raw == nil {
		return nil
	}
	observedAt, ok := passiveObservedAt(raw.ObservedAt)
	if !ok {
		return nil
	}
	signals := normalizedPassiveSignals(raw.Signals)
	var rows []entities.PassiveQuotaMetric
	switch provider {
	case "claude":
		rows = normalizeClaudePassiveRows(scope, signals)
	case "codex":
		rows = normalizeCodexPassiveRows(scope, signals)
	}
	activeLimit := ""
	if provider == "codex" {
		activeLimit = safePassiveIdentifier(signals["x-codex-active-limit"])
	}
	if len(rows) == 0 && activeLimit == "" {
		return nil
	}
	return &entities.PassiveQuotaObservation{ObservedAt: observedAt, ActiveLimit: activeLimit, Quota: rows}
}

func passiveObservedAt(value any) (time.Time, bool) {
	raw, ok := value.(string)
	if !ok {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil || parsed.IsZero() {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func normalizedPassiveSignals(input map[string]any) map[string]string {
	result := make(map[string]string)
	for key, value := range input {
		text, ok := value.(string)
		if !ok {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(key))
		text = strings.TrimSpace(text)
		if name == "" || text == "" || len(text) > maxPassiveSignalValueLength || containsControl(text) {
			continue
		}
		result[name] = text
	}
	return result
}

func normalizeClaudePassiveRows(scope string, signals map[string]string) []entities.PassiveQuotaMetric {
	rows := make([]entities.PassiveQuotaMetric, 0, 5)
	for _, window := range []struct {
		name, label string
		seconds     int64
	}{{"5h", "5h", 5 * 60 * 60}, {"7d", "Weekly", 7 * 24 * 60 * 60}} {
		prefix := "anthropic-ratelimit-unified-" + window.name + "-"
		row := entities.PassiveQuotaMetric{
			Key: "claude." + window.name, Label: window.label, Scope: scope,
			Window: &entities.PassiveQuotaWindow{Seconds: window.seconds},
		}
		hasFact := false
		if value, ok := parseBoundedFloat(signals[prefix+"utilization"], 0, 1); ok {
			percent := value * 100
			row.UsedPercent = &percent
			hasFact = true
		}
		if allowed, ok := parseAllowedStatus(signals[prefix+"status"]); ok {
			row.Allowed = &allowed
			reached := !allowed
			row.LimitReached = &reached
			hasFact = true
		}
		if resetAt, ok := parseSourceTime(signals[prefix+"reset"]); ok {
			row.ResetAt = resetAt
			hasFact = true
		}
		if hasFact {
			rows = append(rows, row)
		}
	}
	for _, state := range []struct {
		key, prefix, label, rowScope, metric string
	}{
		{key: "claude.unified", prefix: "anthropic-ratelimit-unified-", label: "Unified limit", rowScope: scope},
		{key: "claude.7d_oi", prefix: "anthropic-ratelimit-unified-7d_oi-", label: "Fable 7d", rowScope: "model", metric: "Fable"},
	} {
		row := entities.PassiveQuotaMetric{Key: state.key, Label: state.label, Scope: state.rowScope, Metric: state.metric}
		hasFact := false
		if allowed, ok := parseAllowedStatus(signals[state.prefix+"status"]); ok {
			row.Allowed = &allowed
			reached := !allowed
			row.LimitReached = &reached
			hasFact = true
		}
		if resetAt, ok := parseSourceTime(signals[state.prefix+"reset"]); ok {
			row.ResetAt = resetAt
			hasFact = true
		}
		if hasFact {
			rows = append(rows, row)
		}
	}
	if retryRow := passiveRetryHint("claude.retry_after", scope, signals["retry-after"]); retryRow != nil {
		rows = append(rows, *retryRow)
	}
	return rows
}

type codexPassiveGroup struct {
	name       string
	limitName  string
	allowed    *bool
	limit      *bool
	windowData map[string]*codexPassiveWindow
}

type codexPassiveWindow struct {
	usedPercent *float64
	seconds     *int64
	resetAfter  *float64
	resetAt     string
}

func normalizeCodexPassiveRows(scope string, signals map[string]string) []entities.PassiveQuotaMetric {
	planType := safePassiveLabel(signals["x-codex-plan-type"], 80)
	groups := make(map[string]*codexPassiveGroup)
	group := func(name string) *codexPassiveGroup {
		entry := groups[name]
		if entry == nil {
			entry = &codexPassiveGroup{name: name, windowData: make(map[string]*codexPassiveWindow)}
			groups[name] = entry
		}
		return entry
	}

	for key, value := range signals {
		if !strings.HasPrefix(key, "x-codex-") {
			continue
		}
		body := strings.TrimPrefix(key, "x-codex-")
		if body == "plan-type" || body == "active-limit" || strings.HasPrefix(body, "credits-") {
			continue
		}
		if name, field, ok := splitCodexGroupField(body); ok {
			entry := group(name)
			switch field {
			case "allowed":
				if parsed, valid := parseBool(value); valid {
					entry.allowed = &parsed
				}
			case "limit-reached":
				if parsed, valid := parseBool(value); valid {
					entry.limit = &parsed
				}
			case "limit-name":
				entry.limitName = safePassiveLabel(value, 160)
			}
			continue
		}
		name, windowName, field, ok := splitCodexWindowField(body)
		if !ok {
			continue
		}
		entry := group(name)
		window := entry.windowData[windowName]
		if window == nil {
			window = &codexPassiveWindow{}
			entry.windowData[windowName] = window
		}
		switch field {
		case "used-percent":
			if parsed, valid := parseBoundedFloat(value, 0, 100); valid {
				window.usedPercent = &parsed
			}
		case "window-minutes":
			if minutes, valid := parsePositiveInt(value); valid && minutes <= math.MaxInt64/60 {
				seconds := minutes * 60
				window.seconds = &seconds
			}
		case "reset-after-seconds":
			if parsed, valid := parseNonNegativeFloat(value); valid {
				window.resetAfter = &parsed
			}
		case "reset-at":
			if parsed, valid := parseUnixTime(value); valid {
				window.resetAt = parsed
			}
		}
	}

	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	rows := make([]entities.PassiveQuotaMetric, 0, len(names)*2+2)
	for _, name := range names {
		entry := groups[name]
		for windowName, window := range entry.windowData {
			if window.usedPercent == nil && window.seconds == nil && window.resetAfter == nil && window.resetAt == "" {
				delete(entry.windowData, windowName)
			}
		}
		if entry.allowed != nil && entry.limit != nil && *entry.allowed == *entry.limit {
			entry.allowed = nil
			entry.limit = nil
		}
		for _, windowName := range []string{"primary", "secondary"} {
			window := entry.windowData[windowName]
			if window == nil || (window.usedPercent == nil && window.seconds == nil && window.resetAfter == nil && window.resetAt == "") {
				continue
			}
			row := entities.PassiveQuotaMetric{
				Key: codexMetricKey(name, windowName), Label: codexPassiveLabel(entry, windowName, window.seconds), Scope: scope,
				Metric: entry.limitName, PlanType: planType, UsedPercent: window.usedPercent, Allowed: entry.allowed,
				LimitReached: entry.limit, ResetAfterSeconds: window.resetAfter, ResetAt: window.resetAt,
			}
			if window.seconds != nil {
				row.Window = &entities.PassiveQuotaWindow{Seconds: *window.seconds}
			}
			rows = append(rows, row)
		}
		if len(entry.windowData) == 0 && (entry.allowed != nil || entry.limit != nil) {
			rows = append(rows, entities.PassiveQuotaMetric{
				Key: codexMetricKey(name, "state"), Label: codexPassiveGroupLabel(entry), Scope: scope,
				Metric: entry.limitName, PlanType: planType, Allowed: entry.allowed, LimitReached: entry.limit,
			})
		}
	}
	if credits := normalizeCodexCredits(scope, planType, signals); credits != nil {
		rows = append(rows, *credits)
	}
	if retryRow := passiveRetryHint("codex.retry_after", scope, signals["retry-after"]); retryRow != nil {
		retryRow.PlanType = planType
		rows = append(rows, *retryRow)
	}
	return rows
}

func splitCodexGroupField(body string) (string, string, bool) {
	for _, field := range []string{"limit-reached", "limit-name", "allowed"} {
		if body == field {
			return "", field, true
		}
		suffix := "-" + field
		if strings.HasSuffix(body, suffix) {
			name := strings.TrimSuffix(body, suffix)
			if safePassiveIdentifier(name) != "" {
				return name, field, true
			}
		}
	}
	return "", "", false
}

func splitCodexWindowField(body string) (string, string, string, bool) {
	for _, field := range []string{"reset-after-seconds", "window-minutes", "used-percent", "reset-at"} {
		for _, window := range []string{"primary", "secondary"} {
			suffix := "-" + window + "-" + field
			if body == window+"-"+field {
				return "", window, field, true
			}
			if strings.HasSuffix(body, suffix) {
				name := strings.TrimSuffix(body, suffix)
				if safePassiveIdentifier(name) != "" {
					return name, window, field, true
				}
			}
		}
	}
	return "", "", "", false
}

func normalizeCodexCredits(scope, planType string, signals map[string]string) *entities.PassiveQuotaMetric {
	row := entities.PassiveQuotaMetric{Key: "codex.credits", Label: "Credits", Scope: scope, Metric: "credits", Unit: "credits", PlanType: planType}
	hasFact := false
	if value, ok := parseBool(signals["x-codex-credits-has-credits"]); ok {
		row.HasCredits = &value
		hasFact = true
	}
	if value, ok := parseBool(signals["x-codex-credits-unlimited"]); ok {
		row.Unlimited = &value
		hasFact = true
	}
	if value, ok := parseBoundedFloat(signals["x-codex-credits-balance"], 0, math.MaxFloat64); ok {
		row.Remaining = &value
		hasFact = true
	}
	if !hasFact {
		return nil
	}
	return &row
}

func codexMetricKey(group, window string) string {
	if group == "" {
		group = "rate_limit"
	}
	return "codex." + group + "." + window
}

func codexPassiveLabel(group *codexPassiveGroup, window string, seconds *int64) string {
	base := codexPassiveGroupLabel(group)
	windowLabel := "Window"
	if seconds != nil {
		switch *seconds {
		case 5 * 60 * 60:
			windowLabel = "5h"
		case 7 * 24 * 60 * 60:
			windowLabel = "Weekly"
		}
	}
	if group.name == "" {
		return windowLabel
	}
	return base + " " + windowLabel
}

func codexPassiveGroupLabel(group *codexPassiveGroup) string {
	if group.limitName != "" {
		return group.limitName
	}
	if group.name == "" {
		return "Rate limit"
	}
	name := strings.TrimPrefix(group.name, "additional-")
	parts := strings.FieldsFunc(name, func(char rune) bool { return char == '-' || char == '_' })
	for i := range parts {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, " ")
}

func safePassiveIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' {
			return ""
		}
	}
	return value
}

func safePassiveLabel(value string, maxLength int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxLength || containsControl(value) {
		return ""
	}
	return value
}

func containsControl(value string) bool {
	for _, char := range value {
		if unicode.IsControl(char) {
			return true
		}
	}
	return false
}

func parseAllowedStatus(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allowed", "allowed_warning":
		return true, true
	case "rejected":
		return false, true
	default:
		return false, false
	}
}

func parseBool(value string) (bool, bool) {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return parsed, err == nil
}

func parseBoundedFloat(value string, minValue, maxValue float64) (float64, bool) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < minValue || parsed > maxValue {
		return 0, false
	}
	return parsed, true
}

func parsePositiveInt(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed, err == nil && parsed > 0
}

func parseNonNegativeFloat(value string) (float64, bool) {
	return parseBoundedFloat(value, 0, math.MaxFloat64)
}

func parseSourceTime(value string) (string, bool) {
	value = strings.TrimSpace(value)
	var parsed time.Time
	if timestamp, ok := parseUnixTimeValue(value); ok {
		parsed = timestamp
	} else if timestamp, err := time.Parse(time.RFC3339Nano, value); err == nil {
		parsed = timestamp.UTC()
	} else if timestamp, err := http.ParseTime(value); err == nil {
		parsed = timestamp.UTC()
	} else {
		return "", false
	}
	if parsed.Year() < 2000 || parsed.Year() > 9999 {
		return "", false
	}
	return parsed.Format(time.RFC3339), true
}

func parseUnixTime(value string) (string, bool) {
	parsed, ok := parseUnixTimeValue(strings.TrimSpace(value))
	if !ok || parsed.Year() < 2000 || parsed.Year() > 9999 {
		return "", false
	}
	return parsed.Format(time.RFC3339), true
}

func parseUnixTimeValue(value string) (time.Time, bool) {
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || seconds <= 0 || math.IsInf(seconds, 0) || math.IsNaN(seconds) || seconds > float64(math.MaxInt64) {
		return time.Time{}, false
	}
	whole := int64(seconds)
	return time.Unix(whole, int64((seconds-float64(whole))*float64(time.Second))).UTC(), true
}

func passiveRetryHint(key, scope, value string) *entities.PassiveQuotaMetric {
	row := &entities.PassiveQuotaMetric{Key: key, Label: "Observed retry hint", Scope: scope}
	if seconds, ok := parseNonNegativeFloat(value); ok {
		row.ResetAfterSeconds = &seconds
		return row
	}
	if resetAt, ok := parseSourceTime(value); ok {
		row.ResetAt = resetAt
		return row
	}
	return nil
}
