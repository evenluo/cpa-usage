package entities

import "time"

// PassiveQuotaObservation is the latest normalized provider snapshot observed
// by CPA. It is evidence with its original source time, not a refresh cache or
// scheduler state.
type PassiveQuotaObservation struct {
	ObservedAt  time.Time            `json:"observed_at"`
	ActiveLimit string               `json:"active_limit,omitempty"`
	Quota       []PassiveQuotaMetric `json:"quota"`
}

type PassiveModelQuotaObservation struct {
	Model       string               `json:"model"`
	ObservedAt  time.Time            `json:"observed_at"`
	ActiveLimit string               `json:"active_limit,omitempty"`
	Quota       []PassiveQuotaMetric `json:"quota"`
}

type PassiveQuotaMetric struct {
	Key               string              `json:"key"`
	Label             string              `json:"label"`
	Scope             string              `json:"scope"`
	Metric            string              `json:"metric,omitempty"`
	PlanType          string              `json:"planType,omitempty"`
	UsedPercent       *float64            `json:"usedPercent,omitempty"`
	Remaining         *float64            `json:"remaining,omitempty"`
	Unit              string              `json:"unit,omitempty"`
	Allowed           *bool               `json:"allowed,omitempty"`
	LimitReached      *bool               `json:"limitReached,omitempty"`
	Unlimited         *bool               `json:"unlimited,omitempty"`
	Window            *PassiveQuotaWindow `json:"window,omitempty"`
	ResetAt           string              `json:"resetAt,omitempty"`
	ResetAfterSeconds *int64              `json:"resetAfterSeconds,omitempty"`
}

type PassiveQuotaWindow struct {
	Seconds int64 `json:"seconds"`
}
