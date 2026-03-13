package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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
	return "Get API usage cost and token statistics. Without parameters: overview of session/today/month. " +
		"Use 'period' for specific queries: 'today', 'month' (current month), 'YYYY-MM' (specific month), 'YYYY-MM-DD' (specific day). " +
		"Use 'last_days', 'last_months', or 'last_years' for relative ranges (e.g. last 7 days, last 3 months). " +
		"Returns cost, token counts (input/output/total), request count, and per-model breakdown."
}

func (t *CostSummaryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"period": map[string]interface{}{
				"type":        "string",
				"description": "Time period: 'today', 'month' (current month), 'YYYY-MM' (specific month like '2026-03'), 'YYYY-MM-DD' (specific day)",
			},
			"last_days": map[string]interface{}{
				"type":        "integer",
				"description": "Query the last N days (e.g. 7 for last week, 30 for last month)",
			},
			"last_months": map[string]interface{}{
				"type":        "integer",
				"description": "Query the last N months",
			},
			"last_years": map[string]interface{}{
				"type":        "integer",
				"description": "Query the last N years",
			},
		},
		"required": []string{},
	}
}

func (t *CostSummaryTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.tracker == nil {
		return "Cost tracking is not enabled.", nil
	}

	period, _ := args["period"].(string)
	lastDays := toInt(args["last_days"])
	lastMonths := toInt(args["last_months"])
	lastYears := toInt(args["last_years"])

	// If any specific query param is set, return range stats
	if period != "" || lastDays > 0 || lastMonths > 0 || lastYears > 0 {
		return t.executeRange(period, lastDays, lastMonths, lastYears)
	}

	// Default: overview
	return t.executeOverview()
}

func (t *CostSummaryTool) executeOverview() (string, error) {
	summary := t.tracker.GetSummary()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Session: $%.4f (%d requests, %d tokens", summary.SessionCostUSD, summary.RequestCount, summary.TotalTokens))
	if summary.CachedTokens > 0 {
		cacheRate := float64(summary.CachedTokens) / float64(summary.TotalTokens) * 100
		b.WriteString(fmt.Sprintf(", %d cached %.1f%%", summary.CachedTokens, cacheRate))
	}
	b.WriteString(")\n")
	b.WriteString(fmt.Sprintf("Today:   $%.4f\n", summary.DailyCostUSD))
	b.WriteString(fmt.Sprintf("Month:   $%.4f\n", summary.MonthlyCostUSD))

	if len(summary.ByModel) > 0 {
		b.WriteString("\nBy model:\n")
		models := make([]string, 0, len(summary.ByModel))
		for k := range summary.ByModel {
			models = append(models, k)
		}
		sort.Strings(models)
		for _, name := range models {
			ms := summary.ByModel[name]
			line := fmt.Sprintf("  %s: $%.4f (%d reqs, %d tokens", ms.Model, ms.CostUSD, ms.RequestCount, ms.TotalTokens)
			if ms.CachedTokens > 0 {
				cacheRate := float64(ms.CachedTokens) / float64(ms.TotalTokens) * 100
				line += fmt.Sprintf(", %d cached %.1f%%", ms.CachedTokens, cacheRate)
			}
			b.WriteString(line + ")\n")
		}
	}

	return b.String(), nil
}

func (t *CostSummaryTool) executeRange(period string, lastDays, lastMonths, lastYears int) (string, error) {
	now := time.Now().UTC()
	var from, to time.Time
	var label string

	// Add a small buffer to include records written at the current moment
	futureEdge := now.Add(time.Minute)

	switch {
	case lastDays > 0:
		to = futureEdge
		from = now.AddDate(0, 0, -lastDays)
		label = fmt.Sprintf("Last %d days", lastDays)
	case lastMonths > 0:
		to = futureEdge
		from = now.AddDate(0, -lastMonths, 0)
		label = fmt.Sprintf("Last %d months", lastMonths)
	case lastYears > 0:
		to = futureEdge
		from = now.AddDate(-lastYears, 0, 0)
		label = fmt.Sprintf("Last %d years", lastYears)
	case period == "today":
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 0, 1)
		label = "Today"
	case period == "month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 1, 0)
		label = fmt.Sprintf("%s %d", now.Month(), now.Year())
	default:
		// Try YYYY-MM-DD
		if parsed, err := time.Parse("2006-01-02", period); err == nil {
			from = parsed.UTC()
			to = from.AddDate(0, 0, 1)
			label = period
		} else if parsed, err := time.Parse("2006-01", period); err == nil {
			// Try YYYY-MM
			from = parsed.UTC()
			to = from.AddDate(0, 1, 0)
			label = period
		} else {
			return fmt.Sprintf("Invalid period: %q. Use 'today', 'month', 'YYYY-MM', or 'YYYY-MM-DD'.", period), nil
		}
	}

	stats := t.tracker.GetRangeStats(from, to)
	return formatRangeStats(label, stats), nil
}

func formatRangeStats(label string, stats cost.RangeStats) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s: $%.4f\n", label, stats.CostUSD))
	b.WriteString(fmt.Sprintf("Requests: %d\n", stats.RequestCount))
	tokenLine := fmt.Sprintf("Tokens:   %d total (%d input, %d output", stats.TotalTokens, stats.InputTokens, stats.OutputTokens)
	if stats.CachedTokens > 0 && stats.TotalTokens > 0 {
		cacheRate := float64(stats.CachedTokens) / float64(stats.TotalTokens) * 100
		tokenLine += fmt.Sprintf(", %d cached %.1f%%", stats.CachedTokens, cacheRate)
	}
	b.WriteString(tokenLine + ")\n")
	b.WriteString(fmt.Sprintf("Period:   %s to %s\n",
		stats.From.Format("2006-01-02"), stats.To.Format("2006-01-02")))

	if len(stats.ByModel) > 0 {
		b.WriteString("\nBy model:\n")
		// Sort models for deterministic output
		models := make([]string, 0, len(stats.ByModel))
		for k := range stats.ByModel {
			models = append(models, k)
		}
		sort.Strings(models)
		for _, name := range models {
			ms := stats.ByModel[name]
			line := fmt.Sprintf("  %s: $%.4f (%d reqs, %d tokens: %d in / %d out", ms.Model, ms.CostUSD, ms.RequestCount, ms.TotalTokens, ms.InputTokens, ms.OutputTokens)
			if ms.CachedTokens > 0 {
				cacheRate := float64(ms.CachedTokens) / float64(ms.TotalTokens) * 100
				line += fmt.Sprintf(", %d cached %.1f%%", ms.CachedTokens, cacheRate)
			}
			b.WriteString(line + ")\n")
		}
	}

	return b.String()
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}
