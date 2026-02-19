package cost

import (
	"math"
	"time"
)

// ModelPrice holds per-million-token pricing for a model.
type ModelPrice struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

// TokenUsage records token counts and calculated cost for a single API call.
type TokenUsage struct {
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	Timestamp    time.Time `json:"timestamp"`
}

func sanitizePrice(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) || v < 0 {
		return 0
	}
	return v
}

// NewTokenUsage creates a TokenUsage with cost calculated from per-million pricing.
func NewTokenUsage(model string, inputTokens, outputTokens int, inputPricePerMillion, outputPricePerMillion float64) TokenUsage {
	inputPricePerMillion = sanitizePrice(inputPricePerMillion)
	outputPricePerMillion = sanitizePrice(outputPricePerMillion)
	total := inputTokens + outputTokens
	costUSD := (float64(inputTokens)/1_000_000)*inputPricePerMillion +
		(float64(outputTokens)/1_000_000)*outputPricePerMillion
	return TokenUsage{
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  total,
		CostUSD:      costUSD,
		Timestamp:    time.Now().UTC(),
	}
}

// CostRecord is a single persisted cost entry.
type CostRecord struct {
	ID    string     `json:"id"`
	Usage TokenUsage `json:"usage"`
}

// BudgetStatus represents the result of a budget check.
type BudgetStatus int

const (
	BudgetAllowed  BudgetStatus = iota
	BudgetWarning
	BudgetExceeded
)

// UsagePeriod identifies a time-based aggregation window.
type UsagePeriod int

const (
	PeriodSession UsagePeriod = iota
	PeriodDay
	PeriodMonth
)

func (p UsagePeriod) String() string {
	switch p {
	case PeriodDay:
		return "daily"
	case PeriodMonth:
		return "monthly"
	default:
		return "session"
	}
}

// BudgetCheck holds the result of a budget enforcement check.
type BudgetCheck struct {
	Status     BudgetStatus
	CurrentUSD float64
	LimitUSD   float64
	Period     UsagePeriod
}

// CostSummary holds aggregated cost data for reporting.
type CostSummary struct {
	SessionCostUSD float64              `json:"session_cost_usd"`
	DailyCostUSD   float64              `json:"daily_cost_usd"`
	MonthlyCostUSD float64              `json:"monthly_cost_usd"`
	TotalTokens    int                  `json:"total_tokens"`
	RequestCount   int                  `json:"request_count"`
	ByModel        map[string]ModelStats `json:"by_model"`
}

// ModelStats holds per-model cost statistics.
type ModelStats struct {
	Model        string  `json:"model"`
	CostUSD      float64 `json:"cost_usd"`
	TotalTokens  int     `json:"total_tokens"`
	RequestCount int     `json:"request_count"`
}
