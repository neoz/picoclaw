package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/cost"
)

type CostSummaryTool struct {
	tracker *cost.CostTracker
}

func NewCostSummaryTool(tracker *cost.CostTracker) *CostSummaryTool {
	return &CostSummaryTool{tracker: tracker}
}

func (t *CostSummaryTool) Name() string {
	return "cost_summary"
}

func (t *CostSummaryTool) Description() string {
	return "Get API usage cost summary including session, daily, and monthly totals with per-model breakdown. Use this when the user asks about spending, costs, or budget."
}

func (t *CostSummaryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}
}

func (t *CostSummaryTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.tracker == nil {
		return "Cost tracking is not enabled.", nil
	}

	summary := t.tracker.GetSummary()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Session: $%.4f (%d requests, %d tokens)\n", summary.SessionCostUSD, summary.RequestCount, summary.TotalTokens))
	b.WriteString(fmt.Sprintf("Today:   $%.4f\n", summary.DailyCostUSD))
	b.WriteString(fmt.Sprintf("Month:   $%.4f\n", summary.MonthlyCostUSD))

	if len(summary.ByModel) > 0 {
		b.WriteString("\nBy model:\n")
		for _, ms := range summary.ByModel {
			b.WriteString(fmt.Sprintf("  %s: $%.4f (%d reqs, %d tokens)\n",
				ms.Model, ms.CostUSD, ms.RequestCount, ms.TotalTokens))
		}
	}

	return b.String(), nil
}
