package cost

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// CostTracker tracks API usage costs with JSONL persistence and budget enforcement.
type CostTracker struct {
	cfg          config.CostConfig
	storagePath  string
	mu           sync.Mutex
	sessionCosts []CostRecord
	dailyCost    float64
	monthlyCost  float64
	cachedDay    int // day of year
	cachedMonth  time.Month
	cachedYear   int
	priceOverrides map[string]ModelPrice
}

// NewCostTracker creates a new cost tracker. Returns (nil, nil) when cost tracking is disabled.
func NewCostTracker(cfg *config.CostConfig, workspace string) (*CostTracker, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}

	storagePath := filepath.Join(workspace, "state", "costs.jsonl")
	if err := os.MkdirAll(filepath.Dir(storagePath), 0755); err != nil {
		return nil, err
	}

	// Convert config price overrides to cost.ModelPrice
	overrides := make(map[string]ModelPrice, len(cfg.Prices))
	for k, v := range cfg.Prices {
		overrides[k] = ModelPrice{Input: v.Input, Output: v.Output}
	}

	ct := &CostTracker{
		cfg:            *cfg,
		storagePath:    storagePath,
		priceOverrides: overrides,
	}

	ct.rebuildAggregates()
	return ct, nil
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RecordUsage records token usage for a model. Never returns an error; logs and continues.
func (ct *CostTracker) RecordUsage(model string, inputTokens, outputTokens int) {
	if ct == nil {
		return
	}

	price := PriceForModel(model, ct.priceOverrides)
	usage := NewTokenUsage(model, inputTokens, outputTokens, price.Input, price.Output)
	record := CostRecord{
		ID:    newID(),
		Usage: usage,
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Append to JSONL file
	if err := ct.appendRecord(record); err != nil {
		logger.ErrorCF("cost", "Failed to write cost record",
			map[string]interface{}{"error": err.Error()})
		return
	}

	// Update in-memory state
	ct.sessionCosts = append(ct.sessionCosts, record)
	ct.ensurePeriodCurrent()
	now := usage.Timestamp
	if now.YearDay() == ct.cachedDay && now.Year() == ct.cachedYear {
		ct.dailyCost += usage.CostUSD
	}
	if now.Month() == ct.cachedMonth && now.Year() == ct.cachedYear {
		ct.monthlyCost += usage.CostUSD
	}

	logger.DebugCF("cost", "Recorded usage",
		map[string]interface{}{
			"model":         model,
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"cost_usd":      usage.CostUSD,
		})
}

// CheckBudget checks if a request is within budget.
func (ct *CostTracker) CheckBudget(estimatedCostUSD float64) BudgetCheck {
	if ct == nil {
		return BudgetCheck{Status: BudgetAllowed}
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.ensurePeriodCurrent()

	// Check daily limit
	if ct.cfg.DailyLimitUSD > 0 {
		projected := ct.dailyCost + estimatedCostUSD
		if projected > ct.cfg.DailyLimitUSD {
			return BudgetCheck{
				Status:     BudgetExceeded,
				CurrentUSD: ct.dailyCost,
				LimitUSD:   ct.cfg.DailyLimitUSD,
				Period:     PeriodDay,
			}
		}
		warnThreshold := ct.cfg.DailyLimitUSD * ct.cfg.WarnAtPercent / 100
		if projected >= warnThreshold {
			return BudgetCheck{
				Status:     BudgetWarning,
				CurrentUSD: ct.dailyCost,
				LimitUSD:   ct.cfg.DailyLimitUSD,
				Period:     PeriodDay,
			}
		}
	}

	// Check monthly limit
	if ct.cfg.MonthlyLimitUSD > 0 {
		projected := ct.monthlyCost + estimatedCostUSD
		if projected > ct.cfg.MonthlyLimitUSD {
			return BudgetCheck{
				Status:     BudgetExceeded,
				CurrentUSD: ct.monthlyCost,
				LimitUSD:   ct.cfg.MonthlyLimitUSD,
				Period:     PeriodMonth,
			}
		}
		warnThreshold := ct.cfg.MonthlyLimitUSD * ct.cfg.WarnAtPercent / 100
		if projected >= warnThreshold {
			return BudgetCheck{
				Status:     BudgetWarning,
				CurrentUSD: ct.monthlyCost,
				LimitUSD:   ct.cfg.MonthlyLimitUSD,
				Period:     PeriodMonth,
			}
		}
	}

	return BudgetCheck{Status: BudgetAllowed}
}

// GetSummary returns the current cost summary.
func (ct *CostTracker) GetSummary() CostSummary {
	if ct == nil {
		return CostSummary{ByModel: map[string]ModelStats{}}
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.ensurePeriodCurrent()

	var sessionCost float64
	var totalTokens int
	byModel := make(map[string]ModelStats)

	for _, r := range ct.sessionCosts {
		sessionCost += r.Usage.CostUSD
		totalTokens += r.Usage.TotalTokens
		ms := byModel[r.Usage.Model]
		ms.Model = r.Usage.Model
		ms.CostUSD += r.Usage.CostUSD
		ms.TotalTokens += r.Usage.TotalTokens
		ms.RequestCount++
		byModel[r.Usage.Model] = ms
	}

	return CostSummary{
		SessionCostUSD: sessionCost,
		DailyCostUSD:   ct.dailyCost,
		MonthlyCostUSD: ct.monthlyCost,
		TotalTokens:    totalTokens,
		RequestCount:   len(ct.sessionCosts),
		ByModel:        byModel,
	}
}

// GetDailyCost returns the total cost for a specific date by scanning the JSONL file.
func (ct *CostTracker) GetDailyCost(date time.Time) float64 {
	if ct == nil {
		return 0
	}
	ct.mu.Lock()
	defer ct.mu.Unlock()

	targetDay := date.YearDay()
	targetYear := date.Year()
	var total float64
	ct.forEachRecord(func(r CostRecord) {
		if r.Usage.Timestamp.YearDay() == targetDay && r.Usage.Timestamp.Year() == targetYear {
			total += r.Usage.CostUSD
		}
	})
	return total
}

// GetMonthlyCost returns the total cost for a specific month by scanning the JSONL file.
func (ct *CostTracker) GetMonthlyCost(year int, month time.Month) float64 {
	if ct == nil {
		return 0
	}
	ct.mu.Lock()
	defer ct.mu.Unlock()

	var total float64
	ct.forEachRecord(func(r CostRecord) {
		if r.Usage.Timestamp.Year() == year && r.Usage.Timestamp.Month() == month {
			total += r.Usage.CostUSD
		}
	})
	return total
}

// appendRecord writes a single cost record to the JSONL file.
func (ct *CostTracker) appendRecord(record CostRecord) error {
	f, err := os.OpenFile(ct.storagePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

// forEachRecord iterates over all records in the JSONL file.
func (ct *CostTracker) forEachRecord(fn func(CostRecord)) {
	f, err := os.Open(ct.storagePath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r CostRecord
		if json.Unmarshal(line, &r) == nil {
			fn(r)
		}
	}
}

// rebuildAggregates re-reads the JSONL file and computes daily/monthly totals.
func (ct *CostTracker) rebuildAggregates() {
	now := time.Now().UTC()
	ct.cachedDay = now.YearDay()
	ct.cachedMonth = now.Month()
	ct.cachedYear = now.Year()
	ct.dailyCost = 0
	ct.monthlyCost = 0

	ct.forEachRecord(func(r CostRecord) {
		ts := r.Usage.Timestamp
		if ts.YearDay() == ct.cachedDay && ts.Year() == ct.cachedYear {
			ct.dailyCost += r.Usage.CostUSD
		}
		if ts.Month() == ct.cachedMonth && ts.Year() == ct.cachedYear {
			ct.monthlyCost += r.Usage.CostUSD
		}
	})
}

// ensurePeriodCurrent rebuilds aggregates if the day or month has rolled over.
func (ct *CostTracker) ensurePeriodCurrent() {
	now := time.Now().UTC()
	if now.YearDay() != ct.cachedDay || now.Year() != ct.cachedYear || now.Month() != ct.cachedMonth {
		ct.rebuildAggregates()
	}
}
