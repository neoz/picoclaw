package cost

import "strings"

// defaultPrice is a named entry in the default pricing table.
type defaultPrice struct {
	Key   string
	Price ModelPrice
}

// defaultPrices is an ordered list for substring matching.
// The first match wins, so more specific keys should come first.
var defaultPrices = []defaultPrice{
	// Anthropic
	{"claude-opus-4", ModelPrice{Input: 15.0, Output: 75.0}},
	{"claude-sonnet-4", ModelPrice{Input: 3.0, Output: 15.0}},
	{"claude-3.5-sonnet", ModelPrice{Input: 3.0, Output: 15.0}},
	{"claude-3-haiku", ModelPrice{Input: 0.25, Output: 1.25}},
	{"claude-haiku", ModelPrice{Input: 0.25, Output: 1.25}},

	// OpenAI
	{"gpt-4o-mini", ModelPrice{Input: 0.15, Output: 0.60}},
	{"gpt-4o", ModelPrice{Input: 5.0, Output: 15.0}},
	{"o1-preview", ModelPrice{Input: 15.0, Output: 60.0}},

	// Google
	{"gemini-2.0-flash", ModelPrice{Input: 0.10, Output: 0.40}},
	{"gemini-1.5-pro", ModelPrice{Input: 1.25, Output: 5.0}},

	// Zhipu
	{"glm-4", ModelPrice{Input: 0.14, Output: 0.14}},
}

// PriceForModel returns the pricing for a model name.
// User overrides are checked first (exact match), then the default table
// is searched by substring match.
func PriceForModel(model string, overrides map[string]ModelPrice) ModelPrice {
	if overrides != nil {
		if p, ok := overrides[model]; ok {
			return p
		}
	}
	lower := strings.ToLower(model)
	for _, dp := range defaultPrices {
		if strings.Contains(lower, dp.Key) {
			return dp.Price
		}
	}
	return ModelPrice{}
}
