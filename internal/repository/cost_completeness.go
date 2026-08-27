package repository

import "cpa-usage/internal/repository/dto"

type costCompletenessAssessment struct {
	Available bool
	Status    string
}

func assessCostCompleteness(missingPricingEvents int64, pricedBillableEvents int64) costCompletenessAssessment {
	if missingPricingEvents == 0 {
		return costCompletenessAssessment{Available: true, Status: dto.CostStatusAvailable}
	}
	if pricedBillableEvents > 0 {
		return costCompletenessAssessment{Status: dto.CostStatusPartial}
	}
	return costCompletenessAssessment{Status: dto.CostStatusUnavailable}
}
