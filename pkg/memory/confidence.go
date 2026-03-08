package memory

import (
	"math"
	"time"
)

// categoryDecayRates maps memory categories to their exponential decay lambda.
// Higher lambda = faster decay. Lambda = ln(2) / half_life_days.
// core has lambda=0 (never decays), matching the existing "permanent" behavior.
var categoryDecayRates = map[string]float64{
	"core":         0,      // permanent, never decays
	"daily":        0.0231, // ~30-day half-life (ln(2)/30)
	"conversation": 0.0990, // ~7-day half-life  (ln(2)/7)
	"custom":       0.0077, // ~90-day half-life (ln(2)/90)
}

// ComputeConfidence calculates the decayed confidence for a memory entry.
//
// Formula: conf(t) = conf0 * exp(-lambda * days) * (1 + 0.1 * ln(1 + accessCount))
//
// - conf0:       initial confidence score [0, 1]
// - createdAt:   when the memory was created
// - now:         current time
// - accessCount: number of times the memory has been accessed/reinforced
// - category:    memory category (determines decay rate)
//
// The result is clamped to [0, 1].
func ComputeConfidence(conf0 float64, createdAt, now time.Time, accessCount int, category string) float64 {
	lambda, ok := categoryDecayRates[category]
	if !ok {
		lambda = categoryDecayRates["custom"] // default to custom rate
	}

	// No decay for lambda=0 categories (core)
	if lambda == 0 {
		return clampConfidence(conf0 * corroborationBoost(accessCount))
	}

	days := now.Sub(createdAt).Hours() / 24
	if days < 0 {
		days = 0
	}

	decay := math.Exp(-lambda * days)
	boost := corroborationBoost(accessCount)

	return clampConfidence(conf0 * decay * boost)
}

// corroborationBoost returns a multiplicative boost based on access count.
// More frequently accessed memories decay slower.
// boost = 1.0 + 0.1 * ln(1 + accessCount)
func corroborationBoost(accessCount int) float64 {
	if accessCount <= 0 {
		return 1.0
	}
	return 1.0 + 0.1*math.Log(float64(1+accessCount))
}

func clampConfidence(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
