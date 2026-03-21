// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

const maxRetries = 3

type HTTPProvider struct {
	apiKey     string
	apiBase    string
	userAgent  string
	httpClient *http.Client
}

func NewHTTPProvider(apiKey, apiBase, userAgent string) *HTTPProvider {
	return &HTTPProvider{
		apiKey:    apiKey,
		apiBase:   apiBase,
		userAgent: userAgent,
		httpClient: &http.Client{
			Timeout: 3 * time.Minute,
		},
	}
}

func (p *HTTPProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error) {
	if p.apiBase == "" {
		return nil, fmt.Errorf("API base not configured")
	}

	requestBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   true,
		"stream_options": map[string]interface{}{
			"include_usage": true,
		},
	}

	if len(tools) > 0 {
		requestBody["tools"] = tools
		requestBody["tool_choice"] = "auto"
	}

	if maxTokens, ok := options["max_tokens"].(int); ok {
		lowerModel := strings.ToLower(model)
		if strings.Contains(lowerModel, "glm") || strings.Contains(lowerModel, "o1") {
			requestBody["max_completion_tokens"] = maxTokens
		} else {
			requestBody["max_tokens"] = maxTokens
		}
	}

	if temperature, ok := options["temperature"].(float64); ok {
		requestBody["temperature"] = temperature
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiBase+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	if p.userAgent != "" {
		req.Header.Set("User-Agent", p.userAgent)
	}

	var body []byte
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			req, err = http.NewRequestWithContext(ctx, "POST", p.apiBase+"/chat/completions", bytes.NewReader(jsonData))
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			if p.apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+p.apiKey)
			}
			if p.userAgent != "" {
				req.Header.Set("User-Agent", p.userAgent)
			}
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			return p.parseResponse(body)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			delay := parseRetryDelay(resp.Header.Get("Retry-After"), body)
			log.Printf("[provider] Rate limited (429), retrying in %v (attempt %d/%d)", delay, attempt+1, maxRetries)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		return nil, fmt.Errorf("API error: %s", string(body))
	}

	return nil, fmt.Errorf("API error after %d retries: %s", maxRetries, string(body))
}

func (p *HTTPProvider) parseResponse(body []byte) (*LLMResponse, error) {
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function *struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *UsageInfo `json:"usage"`
	}

	// Some providers return SSE "data: {...}" format even when stream=false.
	// Extract the first JSON object from the SSE envelope if needed.
	parseBody := body
	if len(parseBody) > 0 && parseBody[0] != '{' && parseBody[0] != '[' {
		if extracted := extractSSEJSON(parseBody); extracted != nil {
			parseBody = extracted
		}
	}

	if err := json.Unmarshal(parseBody, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		// Try Anthropic-style response format: {content: [{type:"text", text:"..."}], usage: {...}}
		var anthropicResp struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			StopReason string     `json:"stop_reason"`
			Usage      *UsageInfo `json:"usage"`
		}
		if err := json.Unmarshal(parseBody, &anthropicResp); err == nil && len(anthropicResp.Content) > 0 {
			var text string
			for _, c := range anthropicResp.Content {
				if c.Type == "text" {
					text = c.Text
					break
				}
			}
			usage := anthropicResp.Usage
			if usage == nil {
				usage = apiResponse.Usage
			}
			return &LLMResponse{
				Content:      text,
				FinishReason: "stop",
				Usage:        usage,
			}, nil
		}
		return nil, fmt.Errorf("empty response from provider (no choices and no content): %s", string(parseBody))
	}

	choice := apiResponse.Choices[0]

	toolCalls := make([]ToolCall, 0, len(choice.Message.ToolCalls))
	for _, tc := range choice.Message.ToolCalls {
		arguments := make(map[string]interface{})
		name := ""

		// Handle OpenAI format with nested function object
		if tc.Type == "function" && tc.Function != nil {
			name = tc.Function.Name
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &arguments); err != nil {
					arguments["raw"] = tc.Function.Arguments
				}
			}
		} else if tc.Function != nil {
			// Legacy format without type field
			name = tc.Function.Name
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &arguments); err != nil {
					arguments["raw"] = tc.Function.Arguments
				}
			}
		}

		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      name,
			Arguments: arguments,
		})
	}

	content := stripThinkTags(choice.Message.Content)
	if content == "" && choice.Message.ReasoningContent != "" {
		content = stripThinkTags(choice.Message.ReasoningContent)
	}

	return &LLMResponse{
		Content:          content,
		ReasoningContent: choice.Message.ReasoningContent,
		ToolCalls:        toolCalls,
		FinishReason:     choice.FinishReason,
		Usage:            apiResponse.Usage,
	}, nil
}

// stripThinkTags removes <think>...</think> blocks from model output.
// Some reasoning models (e.g. MiniMax) embed their chain-of-thought inline
// in the content field rather than a separate reasoning_content field.
func stripThinkTags(s string) string {
	const openTag = "<think>"
	const closeTag = "</think>"

	result := strings.Builder{}
	rest := s
	for {
		start := strings.Index(rest, openTag)
		if start == -1 {
			result.WriteString(rest)
			break
		}
		result.WriteString(rest[:start])
		end := strings.Index(rest[start:], closeTag)
		if end == -1 {
			// Unclosed tag: drop the rest to avoid leaking partial reasoning.
			break
		}
		rest = rest[start+end+len(closeTag):]
	}
	// Strip orphaned </think> tags and any content before them (leaked reasoning).
	// This happens when models split reasoning across fields, leaving the
	// close tag (with trailing reasoning) at the start of the content field.
	out := result.String()
	if idx := strings.Index(out, closeTag); idx != -1 {
		out = out[idx+len(closeTag):]
	}
	return strings.TrimSpace(out)
}

// extractSSEJSON merges all SSE "data: {...}" chunks into a single non-streaming
// response JSON. Streaming responses use choices[].delta instead of choices[].message,
// and content/tool_calls are spread across multiple chunks that must be aggregated.
func extractSSEJSON(body []byte) []byte {
	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var finishReason string
	var usage json.RawMessage

	// toolCallAccum accumulates streamed tool call deltas by index.
	type toolCallAccum struct {
		ID       string
		Type     string
		Name     string
		ArgsJSON strings.Builder
	}
	toolCalls := make(map[int]*toolCallAccum)

	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		payload := bytes.TrimPrefix(line, []byte("data: "))
		if len(payload) == 0 || payload[0] != '{' {
			continue // skip "data: [DONE]" and non-JSON lines
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function *struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				// Non-streaming format fallback
				Message json.RawMessage `json:"message"`
				FinishReason string     `json:"finish_reason"`
			} `json:"choices"`
			Usage json.RawMessage `json:"usage"`
		}
		if err := json.Unmarshal(payload, &chunk); err != nil {
			continue
		}

		// If a chunk has "message" instead of "delta", it's not actually streamed —
		// return the raw payload as-is (single non-streaming response in SSE envelope).
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Message) > 0 {
			return payload
		}

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			contentBuilder.WriteString(delta.Content)
			reasoningBuilder.WriteString(delta.ReasoningContent)

			for _, tc := range delta.ToolCalls {
				acc, ok := toolCalls[tc.Index]
				if !ok {
					acc = &toolCallAccum{}
					toolCalls[tc.Index] = acc
				}
				if tc.ID != "" {
					acc.ID = tc.ID
				}
				if tc.Type != "" {
					acc.Type = tc.Type
				}
				if tc.Function != nil {
					if tc.Function.Name != "" {
						acc.Name = tc.Function.Name
					}
					acc.ArgsJSON.WriteString(tc.Function.Arguments)
				}
			}

			if chunk.Choices[0].FinishReason != "" {
				finishReason = chunk.Choices[0].FinishReason
			}
		}

		if len(chunk.Usage) > 0 && string(chunk.Usage) != "null" {
			usage = chunk.Usage
		}
	}

	// If we got nothing from the stream, return nil to let caller handle raw body.
	if contentBuilder.Len() == 0 && reasoningBuilder.Len() == 0 && len(toolCalls) == 0 && finishReason == "" {
		return nil
	}

	// Build aggregated tool_calls array sorted by index.
	var tcJSON []interface{}
	if len(toolCalls) > 0 {
		// Find max index
		maxIdx := 0
		for idx := range toolCalls {
			if idx > maxIdx {
				maxIdx = idx
			}
		}
		for i := 0; i <= maxIdx; i++ {
			acc, ok := toolCalls[i]
			if !ok {
				continue
			}
			tcJSON = append(tcJSON, map[string]interface{}{
				"id":   acc.ID,
				"type": acc.Type,
				"function": map[string]interface{}{
					"name":      acc.Name,
					"arguments": acc.ArgsJSON.String(),
				},
			})
		}
	}

	// Reconstruct a non-streaming response structure.
	message := map[string]interface{}{
		"content": contentBuilder.String(),
	}
	if reasoningBuilder.Len() > 0 {
		message["reasoning_content"] = reasoningBuilder.String()
	}
	if len(tcJSON) > 0 {
		message["tool_calls"] = tcJSON
	}

	result := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"message":       message,
				"finish_reason": finishReason,
			},
		},
	}
	if len(usage) > 0 {
		var u interface{}
		if json.Unmarshal(usage, &u) == nil {
			result["usage"] = u
		}
	}

	out, err := json.Marshal(result)
	if err != nil {
		return nil
	}
	return out
}

func (p *HTTPProvider) GetDefaultModel() string {
	return ""
}

// matchProviderByModel finds the best provider for a model name by matching
// against each provider's ModelPatterns. Patterns ending with "/" are prefix
// matches (higher priority). Other patterns are substring matches (lower priority).
// Returns provider name and config, or falls back to the provider marked Fallback,
// then to any provider with a bare api_base (like vllm).
func matchProviderByModel(model string, providers config.ProvidersConfig) (string, *config.ProviderConfig) {
	lowerModel := strings.ToLower(model)

	// Phase 1: prefix matches (patterns ending with "/")
	for name, p := range providers {
		if p.APIKey == "" && p.APIBase == "" {
			continue
		}
		for _, pattern := range p.ModelPatterns {
			if strings.HasSuffix(pattern, "/") && strings.HasPrefix(model, pattern) {
				return name, p
			}
		}
	}

	// Phase 2: contains matches (must have api_key to qualify)
	for name, p := range providers {
		if p.APIKey == "" {
			continue
		}
		for _, pattern := range p.ModelPatterns {
			if !strings.HasSuffix(pattern, "/") && strings.Contains(lowerModel, strings.ToLower(pattern)) {
				return name, p
			}
		}
	}

	// Phase 3: fallback provider (e.g. openrouter)
	for name, p := range providers {
		if p.Fallback && p.APIKey != "" {
			return name, p
		}
	}

	// Phase 4: bare api_base provider (e.g. vllm)
	for name, p := range providers {
		if p.APIBase != "" && len(p.ModelPatterns) == 0 {
			return name, p
		}
	}

	return "", nil
}

func CreateProviderForModel(model, providerName string, cfg *config.Config) (LLMProvider, error) {
	var apiKey, apiBase, userAgent string

	// If explicit provider name is given, use it directly via map lookup
	if providerName != "" {
		pcfg := cfg.GetProviderConfig(strings.ToLower(providerName))
		if pcfg == nil {
			return nil, fmt.Errorf("unknown provider: %s", providerName)
		}
		apiKey = pcfg.APIKey
		apiBase = pcfg.APIBase
		userAgent = pcfg.UserAgent
	} else {
		// Match by model name patterns
		_, matched := matchProviderByModel(model, cfg.Providers)
		if matched != nil {
			apiKey = matched.APIKey
			apiBase = matched.APIBase
			userAgent = matched.UserAgent
		} else {
			return nil, fmt.Errorf("no API key configured for model: %s", model)
		}
	}

	if apiKey == "" && !strings.HasPrefix(model, "bedrock/") {
		return nil, fmt.Errorf("no API key configured for provider (model: %s)", model)
	}

	if apiBase == "" {
		return nil, fmt.Errorf("no API base configured for provider (model: %s)", model)
	}

	return NewHTTPProvider(apiKey, apiBase, userAgent), nil
}

func CreateProvider(cfg *config.Config) (LLMProvider, error) {
	return CreateProviderForModel(cfg.Agents.Defaults.Model, cfg.Agents.Defaults.Provider, cfg)
}

// parseRetryDelay extracts retry delay from Retry-After header or response body.
func parseRetryDelay(retryAfter string, body []byte) time.Duration {
	// Try Retry-After header (seconds)
	if retryAfter != "" {
		if secs, err := strconv.Atoi(retryAfter); err == nil {
			return time.Duration(secs) * time.Second
		}
	}

	// Try parsing retryDelay from Google API error body
	var errResp struct {
		Error struct {
			Details []struct {
				Type       string `json:"@type"`
				RetryDelay string `json:"retryDelay"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil {
		for _, d := range errResp.Error.Details {
			if d.RetryDelay != "" {
				if dur, err := time.ParseDuration(d.RetryDelay); err == nil {
					return dur
				}
			}
		}
	}

	// Default backoff
	return 30 * time.Second
}