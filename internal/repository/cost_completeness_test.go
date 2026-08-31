package repository

import (
	"testing"

	"cpa-usage/internal/repository/dto"
)

func TestAssessCostCompletenessTruthTable(t *testing.T) {
	tests := []struct {
		name           string
		missingPricing int64
		pricedBillable int64
		wantAvailable  bool
		wantStatus     string
	}{
		{name: "no billable tokens", wantAvailable: true, wantStatus: dto.CostStatusAvailable},
		{name: "complete pricing", pricedBillable: 2, wantAvailable: true, wantStatus: dto.CostStatusAvailable},
		{name: "partial pricing", missingPricing: 1, pricedBillable: 1, wantStatus: dto.CostStatusPartial},
		{name: "unavailable pricing", missingPricing: 2, wantStatus: dto.CostStatusUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := assessCostCompleteness(test.missingPricing, test.pricedBillable)
			if got.Available != test.wantAvailable || got.Status != test.wantStatus {
				t.Fatalf("assessment = %+v; want available=%t status=%q", got, test.wantAvailable, test.wantStatus)
			}
		})
	}
}
