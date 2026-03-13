package cost

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

// newTestTracker creates a CostTracker backed by a temp directory.
func newTestTracker(t *testing.T) *CostTracker {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.CostConfig{Enabled: true}
	ct, err := NewCostTracker(cfg, dir)
	if err != nil {
		t.Fatalf("NewCostTracker: %v", err)
	}
	return ct
}

// writeRecord writes a CostRecord directly to the JSONL file (bypasses RecordUsage
// so we can control timestamps).
func writeRecord(t *testing.T, ct *CostTracker, model string, inputTokens, outputTokens int, costUSD float64, ts time.Time) {
	t.Helper()
	r := CostRecord{
		ID: newID(),
		Usage: TokenUsage{
			Model:        model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
			CostUSD:      costUSD,
			Timestamp:    ts,
		},
	}
	if err := ct.appendRecord(r); err != nil {
		t.Fatalf("appendRecord: %v", err)
	}
}

func TestNewCostTracker_Disabled(t *testing.T) {
	ct, err := NewCostTracker(&config.CostConfig{Enabled: false}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ct != nil {
		t.Error("expected nil tracker when disabled")
	}
}

func TestNewCostTracker_Nil(t *testing.T) {
	ct, err := NewCostTracker(nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ct != nil {
		t.Error("expected nil tracker for nil config")
	}
}

func TestRecordUsage_WritesJSONL(t *testing.T) {
	ct := newTestTracker(t)
	ct.RecordUsage("test-model", 100, 50)

	data, err := os.ReadFile(ct.storagePath)
	if err != nil {
		t.Fatal(err)
	}
	var r CostRecord
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Usage.Model != "test-model" {
		t.Errorf("model = %q, want test-model", r.Usage.Model)
	}
	if r.Usage.InputTokens != 100 || r.Usage.OutputTokens != 50 {
		t.Errorf("tokens = %d/%d, want 100/50", r.Usage.InputTokens, r.Usage.OutputTokens)
	}
	if r.Usage.TotalTokens != 150 {
		t.Errorf("total = %d, want 150", r.Usage.TotalTokens)
	}
}

func TestRecordUsage_NilTracker(t *testing.T) {
	var ct *CostTracker
	ct.RecordUsage("model", 100, 50) // should not panic
}

func TestGetSummary_SessionAggregation(t *testing.T) {
	ct := newTestTracker(t)
	ct.RecordUsage("model-a", 100, 50)
	ct.RecordUsage("model-a", 200, 100)
	ct.RecordUsage("model-b", 300, 150)

	s := ct.GetSummary()
	if s.RequestCount != 3 {
		t.Errorf("requests = %d, want 3", s.RequestCount)
	}
	if s.TotalTokens != 900 {
		t.Errorf("tokens = %d, want 900", s.TotalTokens)
	}
	if len(s.ByModel) != 2 {
		t.Errorf("models = %d, want 2", len(s.ByModel))
	}
	if ms, ok := s.ByModel["model-a"]; !ok || ms.RequestCount != 2 {
		t.Errorf("model-a requests = %v", s.ByModel["model-a"])
	}
}

func TestGetSummary_NilTracker(t *testing.T) {
	var ct *CostTracker
	s := ct.GetSummary()
	if s.ByModel == nil {
		t.Error("ByModel should be non-nil even for nil tracker")
	}
}

func TestGetRangeStats_BasicRange(t *testing.T) {
	ct := newTestTracker(t)

	base := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	writeRecord(t, ct, "model-a", 100, 50, 0.01, base)
	writeRecord(t, ct, "model-a", 200, 100, 0.02, base.Add(time.Hour))
	writeRecord(t, ct, "model-b", 300, 150, 0.03, base.AddDate(0, 0, 1))
	// Outside range
	writeRecord(t, ct, "model-a", 400, 200, 0.04, base.AddDate(0, 0, 5))

	from := base
	to := base.AddDate(0, 0, 2) // covers day 10 and 11

	stats := ct.GetRangeStats(from, to)

	if stats.RequestCount != 3 {
		t.Errorf("requests = %d, want 3", stats.RequestCount)
	}
	if stats.InputTokens != 600 {
		t.Errorf("input = %d, want 600", stats.InputTokens)
	}
	if stats.OutputTokens != 300 {
		t.Errorf("output = %d, want 300", stats.OutputTokens)
	}
	if stats.TotalTokens != 900 {
		t.Errorf("total = %d, want 900", stats.TotalTokens)
	}
	if math.Abs(stats.CostUSD-0.06) > 1e-9 {
		t.Errorf("cost = %f, want 0.06", stats.CostUSD)
	}
	if len(stats.ByModel) != 2 {
		t.Errorf("models = %d, want 2", len(stats.ByModel))
	}
	msA := stats.ByModel["model-a"]
	if msA.RequestCount != 2 || msA.InputTokens != 300 || msA.OutputTokens != 150 {
		t.Errorf("model-a stats: %+v", msA)
	}
	msB := stats.ByModel["model-b"]
	if msB.RequestCount != 1 || msB.InputTokens != 300 || msB.OutputTokens != 150 {
		t.Errorf("model-b stats: %+v", msB)
	}
}

func TestGetRangeStats_EmptyRange(t *testing.T) {
	ct := newTestTracker(t)
	writeRecord(t, ct, "model", 100, 50, 0.01, time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC))

	// Query a range that doesn't contain the record
	stats := ct.GetRangeStats(
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	)
	if stats.RequestCount != 0 {
		t.Errorf("requests = %d, want 0", stats.RequestCount)
	}
	if stats.CostUSD != 0 {
		t.Errorf("cost = %f, want 0", stats.CostUSD)
	}
}

func TestGetRangeStats_NilTracker(t *testing.T) {
	var ct *CostTracker
	stats := ct.GetRangeStats(time.Now(), time.Now().Add(time.Hour))
	if stats.ByModel == nil {
		t.Error("ByModel should be non-nil even for nil tracker")
	}
	if stats.RequestCount != 0 {
		t.Error("expected zero requests for nil tracker")
	}
}

func TestGetRangeStats_BoundaryInclusive(t *testing.T) {
	ct := newTestTracker(t)
	exact := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	writeRecord(t, ct, "model", 100, 50, 0.01, exact)

	// from == timestamp should be included (inclusive lower bound)
	stats := ct.GetRangeStats(exact, exact.Add(time.Second))
	if stats.RequestCount != 1 {
		t.Errorf("inclusive lower: requests = %d, want 1", stats.RequestCount)
	}

	// to == timestamp should be excluded (exclusive upper bound)
	stats = ct.GetRangeStats(exact.Add(-time.Second), exact)
	if stats.RequestCount != 0 {
		t.Errorf("exclusive upper: requests = %d, want 0", stats.RequestCount)
	}
}

func TestGetRangeStats_MonthSpan(t *testing.T) {
	ct := newTestTracker(t)
	// Records across 3 months
	writeRecord(t, ct, "model", 100, 50, 0.01, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC))
	writeRecord(t, ct, "model", 200, 100, 0.02, time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC))
	writeRecord(t, ct, "model", 300, 150, 0.03, time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))

	// Query last 2 months from end of March
	from := time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC) // after Jan record
	to := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	stats := ct.GetRangeStats(from, to)
	if stats.RequestCount != 2 {
		t.Errorf("requests = %d, want 2", stats.RequestCount)
	}
	if stats.InputTokens != 500 {
		t.Errorf("input = %d, want 500", stats.InputTokens)
	}
}

func TestGetDailyCost(t *testing.T) {
	ct := newTestTracker(t)
	day := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	writeRecord(t, ct, "model", 100, 50, 0.01, day.Add(2*time.Hour))
	writeRecord(t, ct, "model", 100, 50, 0.02, day.Add(5*time.Hour))
	writeRecord(t, ct, "model", 100, 50, 0.05, day.AddDate(0, 0, 1))

	got := ct.GetDailyCost(day)
	if math.Abs(got-0.03) > 1e-9 {
		t.Errorf("daily cost = %f, want 0.03", got)
	}
}

func TestGetMonthlyCost(t *testing.T) {
	ct := newTestTracker(t)
	writeRecord(t, ct, "model", 100, 50, 0.01, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	writeRecord(t, ct, "model", 100, 50, 0.02, time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))
	writeRecord(t, ct, "model", 100, 50, 0.05, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))

	got := ct.GetMonthlyCost(2026, time.March)
	if math.Abs(got-0.03) > 1e-9 {
		t.Errorf("monthly cost = %f, want 0.03", got)
	}
}

func TestCheckBudget_DailyExceeded(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.CostConfig{
		Enabled:       true,
		DailyLimitUSD: 0.05,
		WarnAtPercent: 80,
	}
	ct, err := NewCostTracker(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write a record for today with cost exceeding limit
	now := time.Now().UTC()
	writeRecord(t, ct, "model", 100, 50, 0.06, now)
	ct.rebuildAggregates()

	check := ct.CheckBudget(0)
	if check.Status != BudgetExceeded {
		t.Errorf("status = %d, want BudgetExceeded", check.Status)
	}
	if check.Period != PeriodDay {
		t.Errorf("period = %v, want PeriodDay", check.Period)
	}
}

func TestCheckBudget_NilTracker(t *testing.T) {
	var ct *CostTracker
	check := ct.CheckBudget(0)
	if check.Status != BudgetAllowed {
		t.Error("nil tracker should always allow")
	}
}

func TestStoragePath(t *testing.T) {
	dir := t.TempDir()
	ct, _ := NewCostTracker(&config.CostConfig{Enabled: true}, dir)
	expected := filepath.Join(dir, "state", "costs.jsonl")
	if ct.storagePath != expected {
		t.Errorf("path = %q, want %q", ct.storagePath, expected)
	}
}
