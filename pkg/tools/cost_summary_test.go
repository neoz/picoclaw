package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cost"
)

func newTestCostTracker(t *testing.T) *cost.CostTracker {
	t.Helper()
	ct, err := cost.NewCostTracker(&config.CostConfig{Enabled: true}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

func TestCostSummaryTool_Name(t *testing.T) {
	tool := NewCostSummaryTool(nil)
	if tool.Name() != "cost_summary" {
		t.Errorf("name = %q", tool.Name())
	}
}

func TestCostSummaryTool_NilTracker(t *testing.T) {
	tool := NewCostSummaryTool(nil)
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "not enabled") {
		t.Errorf("expected disabled message, got %q", result)
	}
}

func TestCostSummaryTool_DefaultOverview(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("test-model", 100, 50, 0)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Session:") {
		t.Error("missing Session line")
	}
	if !strings.Contains(result, "Today:") {
		t.Error("missing Today line")
	}
	if !strings.Contains(result, "Month:") {
		t.Error("missing Month line")
	}
	if !strings.Contains(result, "test-model") {
		t.Error("missing model breakdown")
	}
}

func TestCostSummaryTool_PeriodToday(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("model-a", 500, 250, 0)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"period": "today",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Today:") {
		t.Error("missing Today label")
	}
	if !strings.Contains(result, "Requests:") {
		t.Error("missing Requests line")
	}
	if !strings.Contains(result, "Tokens:") {
		t.Error("missing Tokens line")
	}
	if !strings.Contains(result, "input") && !strings.Contains(result, "output") {
		t.Error("missing input/output token breakdown")
	}
}

func TestCostSummaryTool_PeriodMonth(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("model-a", 100, 50, 0)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"period": "month",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if !strings.Contains(result, now.Month().String()) {
		t.Errorf("expected current month name in output, got %q", result)
	}
	if !strings.Contains(result, "Requests:") {
		t.Error("missing Requests line")
	}
}

func TestCostSummaryTool_PeriodSpecificMonth(t *testing.T) {
	ct := newTestCostTracker(t)
	tool := NewCostSummaryTool(ct)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"period": "2026-03",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "2026-03") {
		t.Errorf("expected 2026-03 label, got %q", result)
	}
	if !strings.Contains(result, "Period:") {
		t.Error("missing Period line")
	}
}

func TestCostSummaryTool_PeriodSpecificDay(t *testing.T) {
	ct := newTestCostTracker(t)
	tool := NewCostSummaryTool(ct)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"period": "2026-03-10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "2026-03-10") {
		t.Errorf("expected 2026-03-10 label, got %q", result)
	}
}

func TestCostSummaryTool_InvalidPeriod(t *testing.T) {
	ct := newTestCostTracker(t)
	tool := NewCostSummaryTool(ct)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"period": "garbage",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Invalid period") {
		t.Errorf("expected error message, got %q", result)
	}
}

func TestCostSummaryTool_LastDays(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("model-a", 200, 100, 0)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"last_days": float64(7),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Last 7 days") {
		t.Errorf("expected 'Last 7 days' label, got %q", result)
	}
	if !strings.Contains(result, "Requests: 1") {
		t.Errorf("expected 1 request, got %q", result)
	}
}

func TestCostSummaryTool_LastMonths(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("model-b", 300, 150, 0)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"last_months": float64(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Last 3 months") {
		t.Errorf("expected 'Last 3 months' label, got %q", result)
	}
	if !strings.Contains(result, "Requests: 1") {
		t.Error("expected 1 request")
	}
	if !strings.Contains(result, "model-b") {
		t.Error("missing model in output")
	}
}

func TestCostSummaryTool_LastYears(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("model-c", 100, 50, 0)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"last_years": float64(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Last 1 years") {
		t.Errorf("expected 'Last 1 years' label, got %q", result)
	}
}

func TestCostSummaryTool_TokenBreakdownInRange(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("model-a", 500, 250, 0)
	ct.RecordUsage("model-a", 300, 100, 0)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"period": "today",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should show total 1150 tokens (800 input + 350 output)
	if !strings.Contains(result, "1150 total") {
		t.Errorf("expected 1150 total tokens, got %q", result)
	}
	if !strings.Contains(result, "800 input") {
		t.Errorf("expected 800 input tokens, got %q", result)
	}
	if !strings.Contains(result, "350 output") {
		t.Errorf("expected 350 output tokens, got %q", result)
	}
}

func TestCostSummaryTool_PerModelBreakdownInRange(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("alpha", 100, 50, 0)
	ct.RecordUsage("beta", 200, 100, 0)
	ct.RecordUsage("alpha", 300, 150, 0)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"period": "today",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "alpha") || !strings.Contains(result, "beta") {
		t.Errorf("missing model names in output: %q", result)
	}
	// alpha should appear before beta (sorted)
	alphaIdx := strings.Index(result, "alpha")
	betaIdx := strings.Index(result, "beta")
	if alphaIdx > betaIdx {
		t.Error("models should be sorted alphabetically")
	}
}

func TestCostSummaryTool_ToIntConversions(t *testing.T) {
	tests := []struct {
		input interface{}
		want  int
	}{
		{float64(7), 7},
		{int(3), 3},
		{"5", 5},
		{nil, 0},
		{true, 0},
		{"abc", 0},
	}
	for _, tt := range tests {
		got := toInt(tt.input)
		if got != tt.want {
			t.Errorf("toInt(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCostSummaryTool_CacheHitOverview(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("model-a", 1000, 100, 600)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "600 cached") {
		t.Errorf("expected cached tokens in output, got %q", result)
	}
	if !strings.Contains(result, "54.5%") {
		t.Errorf("expected cache rate percentage, got %q", result)
	}
}

func TestCostSummaryTool_CacheHitRange(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("model-a", 2000, 200, 1500)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"period": "today",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "1500 cached") {
		t.Errorf("expected cached tokens in range output, got %q", result)
	}
	if !strings.Contains(result, "68.2%") {
		t.Errorf("expected cache rate in range output, got %q", result)
	}
}

func TestCostSummaryTool_NoCacheHitHidden(t *testing.T) {
	ct := newTestCostTracker(t)
	ct.RecordUsage("model-a", 1000, 100, 0)

	tool := NewCostSummaryTool(ct)
	result, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "cached") {
		t.Errorf("should not show cached info when zero, got %q", result)
	}
}

func TestCostSummaryTool_Parameters(t *testing.T) {
	tool := NewCostSummaryTool(nil)
	params := tool.Parameters()
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties not a map")
	}
	for _, key := range []string{"period", "last_days", "last_months", "last_years"} {
		if _, exists := props[key]; !exists {
			t.Errorf("missing parameter %q", key)
		}
	}
}
