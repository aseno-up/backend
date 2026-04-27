package agent

import (
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aseno/backend/internal/logger"
)

// MetricsCollector collects and tracks AI agent metrics
type MetricsCollector struct {
	mu sync.RWMutex

	// Conversation metrics
	conversationsStarted   int64
	conversationsCompleted int64
	conversationsAbandoned int64
	totalConversationTurns int64

	// Tool metrics
	toolCallsTotal   int64
	toolCallsSuccess int64
	toolCallsFailed  int64
	toolLatencies    map[string][]time.Duration // tool name -> latencies

	// AI metrics
	totalTokensUsed   int64
	totalCost         float64 // in currency
	responseLatencies []time.Duration
	fallbackTriggers  int64

	// Response generation
	generationsTotal   int64
	generationsSuccess int64
	generationsFailed  int64
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		toolLatencies:     make(map[string][]time.Duration),
		responseLatencies: make([]time.Duration, 0, 1000),
	}
}

// RecordConversationStarted records a new conversation started
func (m *MetricsCollector) RecordConversationStarted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conversationsStarted++
}

// RecordConversationCompleted records a conversation completed successfully
func (m *MetricsCollector) RecordConversationCompleted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conversationsCompleted++
}

// RecordConversationAbandoned records a conversation abandoned
func (m *MetricsCollector) RecordConversationAbandoned() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conversationsAbandoned++
}

// RecordConversationTurns records the number of turns in a conversation
func (m *MetricsCollector) RecordConversationTurns(turns int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalConversationTurns += int64(turns)
}

// RecordToolCall records a tool execution
func (m *MetricsCollector) RecordToolCall(toolName string, success bool, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.toolCallsTotal++
	if success {
		m.toolCallsSuccess++
	} else {
		m.toolCallsFailed++
	}

	if m.toolLatencies[toolName] == nil {
		m.toolLatencies[toolName] = make([]time.Duration, 0, 100)
	}
	m.toolLatencies[toolName] = append(m.toolLatencies[toolName], latency)
}

// RecordGeneration records an AI generation attempt
func (m *MetricsCollector) RecordGeneration(success bool, tokensUsed int, cost float64, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.generationsTotal++
	if success {
		m.generationsSuccess++
		m.totalTokensUsed += int64(tokensUsed)
		m.totalCost += cost
		m.responseLatencies = append(m.responseLatencies, latency)
	} else {
		m.generationsFailed++
	}
}

// RecordFallback records a fallback to traditional handler
func (m *MetricsCollector) RecordFallback() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallbackTriggers++
}

// GetMetrics returns a snapshot of current metrics
func (m *MetricsCollector) GetMetrics() *Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &Metrics{
		ConversationsStarted:     m.conversationsStarted,
		ConversationsCompleted:   m.conversationsCompleted,
		ConversationsAbandoned:   m.conversationsAbandoned,
		AverageConversationTurns: m.calculateAverageConversationTurns(),

		ToolCallsTotal:   m.toolCallsTotal,
		ToolCallsSuccess: m.toolCallsSuccess,
		ToolCallsFailed:  m.toolCallsFailed,
		ToolSuccessRate:  m.calculateToolSuccessRate(),

		TotalTokensUsed:    m.totalTokensUsed,
		TotalCost:          m.totalCost,
		AverageCostPerConv: m.calculateAverageCostPerConversation(),
		ResponseLatencyP50: m.calculatePercentile(m.responseLatencies, 0.50),
		ResponseLatencyP95: m.calculatePercentile(m.responseLatencies, 0.95),
		ResponseLatencyP99: m.calculatePercentile(m.responseLatencies, 0.99),

		GenerationsTotal:      m.generationsTotal,
		GenerationsSuccess:    m.generationsSuccess,
		GenerationsFailed:     m.generationsFailed,
		GenerationSuccessRate: m.calculateGenerationSuccessRate(),

		FallbackTriggers: m.fallbackTriggers,
	}
}

// LogMetrics logs current metrics
func (m *MetricsCollector) LogMetrics() {
	metrics := m.GetMetrics()

	logger.Println("========== AI Agent Metrics ==========")
	logger.Printf("Conversations: Started=%d, Completed=%d, Abandoned=%d (Avg Turns: %.2f)",
		metrics.ConversationsStarted, metrics.ConversationsCompleted, metrics.ConversationsAbandoned, metrics.AverageConversationTurns)
	logger.Printf("Tool Calls: Total=%d, Success=%d, Failed=%d (Success Rate: %.2f%%)",
		metrics.ToolCallsTotal, metrics.ToolCallsSuccess, metrics.ToolCallsFailed, metrics.ToolSuccessRate*100)
	logger.Printf("Generations: Total=%d, Success=%d, Failed=%d (Success Rate: %.2f%%)",
		metrics.GenerationsTotal, metrics.GenerationsSuccess, metrics.GenerationsFailed, metrics.GenerationSuccessRate*100)
	logger.Printf("Tokens: Total=%d, Cost=R$ %.4f, Avg Cost/Conv=R$ %.4f",
		metrics.TotalTokensUsed, metrics.TotalCost, metrics.AverageCostPerConv)
	logger.Printf("Response Latency: p50=%.2fs, p95=%.2fs, p99=%.2fs",
		metrics.ResponseLatencyP50.Seconds(), metrics.ResponseLatencyP95.Seconds(), metrics.ResponseLatencyP99.Seconds())
	logger.Printf("Fallback Triggers: %d", metrics.FallbackTriggers)
	logger.Println("======================================")
}

// Helper methods

func (m *MetricsCollector) calculateAverageConversationTurns() float64 {
	if m.conversationsStarted == 0 {
		return 0
	}
	return float64(m.totalConversationTurns) / float64(m.conversationsStarted)
}

func (m *MetricsCollector) calculateToolSuccessRate() float64 {
	if m.toolCallsTotal == 0 {
		return 0
	}
	return float64(m.toolCallsSuccess) / float64(m.toolCallsTotal)
}

func (m *MetricsCollector) calculateGenerationSuccessRate() float64 {
	if m.generationsTotal == 0 {
		return 0
	}
	return float64(m.generationsSuccess) / float64(m.generationsTotal)
}

func (m *MetricsCollector) calculateAverageCostPerConversation() float64 {
	if m.conversationsStarted == 0 {
		return 0
	}
	return m.totalCost / float64(m.conversationsStarted)
}

func (m *MetricsCollector) calculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Simple percentile calculation (for production, use a proper library)
	index := int(float64(len(latencies)) * percentile)
	if index >= len(latencies) {
		index = len(latencies) - 1
	}

	return latencies[index]
}

// Metrics represents a snapshot of metrics
type Metrics struct {
	// Conversation metrics
	ConversationsStarted     int64
	ConversationsCompleted   int64
	ConversationsAbandoned   int64
	AverageConversationTurns float64

	// Tool metrics
	ToolCallsTotal   int64
	ToolCallsSuccess int64
	ToolCallsFailed  int64
	ToolSuccessRate  float64

	// AI metrics
	TotalTokensUsed    int64
	TotalCost          float64
	AverageCostPerConv float64
	ResponseLatencyP50 time.Duration
	ResponseLatencyP95 time.Duration
	ResponseLatencyP99 time.Duration

	// Generation metrics
	GenerationsTotal      int64
	GenerationsSuccess    int64
	GenerationsFailed     int64
	GenerationSuccessRate float64

	// Fallback metrics
	FallbackTriggers int64
}

// usdToBrl is the conversion factor used for cost estimates. It is read once
// at package init from the USD_TO_BRL_RATE env var so FX drift can be
// adjusted without rebuilding the binary; missing/invalid values fall back
// to the documented default so a bad config string cannot silently zero out
// the cost dashboard.
var usdToBrl = parseUSDToBRL(os.Getenv("USD_TO_BRL_RATE"))

const defaultUSDToBRL = 5.0

// parseUSDToBRL turns the raw env-var string into a positive float. Anything
// non-numeric, zero, or negative is treated as "not set" and the documented
// default is used. Exposed (lowercase) so tests can drive it directly without
// needing to reach into the OS env.
func parseUSDToBRL(raw string) float64 {
	if raw == "" {
		return defaultUSDToBRL
	}
	f, err := strconv.ParseFloat(raw, 64)
	// strconv.ParseFloat accepts "nan", "inf", "+inf" without error — those
	// would silently break the cost dashboard, so reject explicitly.
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
		return defaultUSDToBRL
	}
	return f
}

// modelPricing maps a model name to its USD-per-1M-tokens cost.
// Sourced from OpenRouter's listing for each model. All models in the
// production primary + fallback chain are covered explicitly; everything
// else falls back to fallbackPricing, which is set conservatively to
// gpt-4o-mini's rate (our primary).
type modelRate struct {
	inputPer1M  float64
	outputPer1M float64
}

var modelPricing = map[string]modelRate{
	// Production primary + fallbacks (k8s/secrets.yaml AI_AGENT_MODEL chain)
	"openai/gpt-4o-mini":               {0.15, 0.60},
	"google/gemini-2.0-flash-001":      {0.10, 0.40},
	"google/gemini-2.0-flash-lite-001": {0.075, 0.30},
	"qwen/qwen-2.5-72b-instruct":       {0.13, 0.39},

	// Common alternates we may switch to without redeploying this code
	"openai/gpt-4o":                {2.50, 10.00},
	"openai/gpt-4-turbo":           {10.00, 30.00},
	"openai/gpt-3.5-turbo":         {0.50, 1.50},
	"anthropic/claude-3.5-sonnet":  {3.00, 15.00},
	"anthropic/claude-3.5-haiku":   {0.80, 4.00},
	"anthropic/claude-3-opus":      {15.00, 75.00},
	"google/gemini-1.5-flash":      {0.075, 0.30},
	"google/gemini-1.5-pro":        {1.25, 5.00},
	"meta-llama/llama-3.1-70b-instruct": {0.30, 0.30},
	"mistralai/mistral-7b-instruct":     {0.06, 0.06},

	// Legacy keys preserved for backwards compatibility with non-OpenRouter
	// callers (older Gemini direct integrations used these short names).
	"gemini-2.0-flash":     {0.10, 0.40},
	"gemini-2.0-flash-exp": {0, 0},
	"gemini-1.5-flash":     {0.075, 0.30},
	"gemini-1.5-pro":       {1.25, 5.00},
}

// fallbackPricing is used when the model name is not in modelPricing.
// We use the primary model's rate so unknown models do not silently report
// zero cost on the admin dashboard.
var fallbackPricing = modelRate{0.15, 0.60}

// EstimateTokenCostSplit returns the BRL cost of a single LLM call with a
// real input/output split. This is the function billing/observability code
// should use; OpenRouter and Gemini both report the split.
func EstimateTokenCostSplit(inputTokens, outputTokens int, model string) float64 {
	rate, ok := modelPricing[model]
	if !ok {
		rate = fallbackPricing
	}
	inCost := float64(inputTokens) / 1_000_000.0 * rate.inputPer1M
	outCost := float64(outputTokens) / 1_000_000.0 * rate.outputPer1M
	return (inCost + outCost) * usdToBrl
}

// EstimateTokenCost is the legacy single-number signature kept for callers
// that do not have the in/out split. Approximates a chat-typical 80/20
// input/output ratio (system prompt + history dwarfs the reply).
func EstimateTokenCost(tokens int, model string) float64 {
	in := tokens * 80 / 100
	out := tokens - in
	return EstimateTokenCostSplit(in, out, model)
}
