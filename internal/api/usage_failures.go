package api

import (
	"net/http"
	"strings"
	"time"

	repodto "cpa-usage/internal/repository/dto"
	"github.com/gin-gonic/gin"
)

type usageFailureDistributionResponse struct {
	WindowStart   string                       `json:"window_start"`
	WindowEnd     string                       `json:"window_end"`
	TotalFailures int64                        `json:"total_failures"`
	Categories    usageFailureBreakdownPayload `json:"categories"`
	Statuses      usageFailureBreakdownPayload `json:"statuses"`
	Providers     usageFailureBreakdownPayload `json:"providers"`
	Accounts      usageFailureBreakdownPayload `json:"accounts"`
	Models        usageFailureBreakdownPayload `json:"models"`
	Endpoints     usageFailureBreakdownPayload `json:"endpoints"`
}

type usageFailureBreakdownPayload struct {
	Items      []usageFailureBreakdownItemPayload `json:"items"`
	OtherCount int64                              `json:"other_count"`
}

type usageFailureBreakdownItemPayload struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Category string `json:"category,omitempty"`
	Count    int64  `json:"count"`
}

func registerUsageFailuresRoute(router gin.IRoutes, usageProvider UsageProvider, usageIdentityProvider UsageIdentityProvider) {
	router.GET("/usage/failures", func(c *gin.Context) {
		filter, err := parseFixedUsageDiagnosticFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if usageProvider == nil {
			c.JSON(http.StatusOK, emptyUsageFailureDistributionResponse(filter))
			return
		}
		record, err := usageProvider.GetUsageFailureDistribution(c.Request.Context(), filter.repositoryFilter())
		if err != nil {
			writeInternalError(c, "get usage failure distribution failed", err)
			return
		}
		identities, err := loadUsageResolutionData(c, usageIdentityProvider)
		if err != nil {
			writeInternalError(c, "load usage failure account labels failed", err)
			return
		}
		c.JSON(http.StatusOK, buildUsageFailureDistributionPayload(filter, record, newUsageIdentityResolver(identities)))
	})
}

func emptyUsageFailureDistributionResponse(filter usageDiagnosticFilter) usageFailureDistributionResponse {
	return buildUsageFailureDistributionPayload(filter, &repodto.UsageFailureDistributionRecord{}, usageIdentityResolver{})
}

func buildUsageFailureDistributionPayload(filter usageDiagnosticFilter, record *repodto.UsageFailureDistributionRecord, resolver usageIdentityResolver) usageFailureDistributionResponse {
	if record == nil {
		record = &repodto.UsageFailureDistributionRecord{}
	}
	return usageFailureDistributionResponse{
		WindowStart:   filter.StartTime.UTC().Format(time.RFC3339Nano),
		WindowEnd:     filter.EndTime.UTC().Format(time.RFC3339Nano),
		TotalFailures: record.TotalFailures,
		Categories:    buildUsageFailureBreakdownPayload(record.Categories, statusCategoryLabel, statusCategory),
		Statuses:      buildUsageFailureBreakdownPayload(record.Statuses, statusCodeLabel, statusCategory),
		Providers:     buildUsageFailureBreakdownPayload(record.Providers, identityLabel, nil),
		Accounts: buildUsageFailureBreakdownPayload(record.Accounts, func(value string) string {
			if identity, matched := resolver.resolveByAuthIndex(value); matched {
				return identity.DisplayName
			}
			return value
		}, nil),
		Models:    buildUsageFailureBreakdownPayload(record.Models, identityLabel, nil),
		Endpoints: buildUsageFailureBreakdownPayload(record.Endpoints, identityLabel, nil),
	}
}

func buildUsageFailureBreakdownPayload(record repodto.UsageFailureBreakdownRecord, label func(string) string, category func(string) string) usageFailureBreakdownPayload {
	items := make([]usageFailureBreakdownItemPayload, 0, len(record.Items))
	for _, item := range record.Items {
		payload := usageFailureBreakdownItemPayload{Value: item.Value, Label: label(item.Value), Count: item.Count}
		if category != nil {
			payload.Category = category(item.Value)
		}
		items = append(items, payload)
	}
	return usageFailureBreakdownPayload{Items: items, OtherCount: record.OtherCount}
}

func statusCategory(value string) string {
	if value == "unknown" || value == "other" || (len(value) == 3 && strings.HasSuffix(value, "xx")) {
		return value
	}
	if len(value) == 3 && value[0] >= '1' && value[0] <= '5' {
		return value[:1] + "xx"
	}
	return "other"
}

func statusCategoryLabel(value string) string {
	switch value {
	case "unknown":
		return "Unknown status"
	case "other":
		return "Other observed status"
	default:
		return strings.ToUpper(value)
	}
}

func statusCodeLabel(value string) string {
	if value == "unknown" {
		return "Unknown status"
	}
	return "HTTP " + value
}

func identityLabel(value string) string { return value }
