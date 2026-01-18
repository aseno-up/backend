package agent

import (
	"log"
	"sync"
	"time"
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
	totalTokensUsed int64
	totalCost       float64 // in currency
	responseLatencies []time.Duration
	fallbackTriggers int64

	// Response generation
	generationsTotal   int64
	generationsSuccess int64
	generationsFailed  int64
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		toolLatencies: make(map[string][]time.Duration),
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
		ConversationsStarted:   m.conversationsStarted,
		ConversationsCompleted: m.conversationsCompleted,
		ConversationsAbandoned: m.conversationsAbandoned,
		AverageConversationTurns: m.calculateAverageConversationTurns(),

		ToolCallsTotal:   m.toolCallsTotal,
		ToolCallsSuccess: m.toolCallsSuccess,
		ToolCallsFailed:  m.toolCallsFailed,
		ToolSuccessRate:  m.calculateToolSuccessRate(),

		TotalTokensUsed:       m.totalTokensUsed,
		TotalCost:             m.totalCost,
		AverageCostPerConv:    m.calculateAverageCostPerConversation(),
		ResponseLatencyP50:    m.calculatePercentile(m.responseLatencies, 0.50),
		ResponseLatencyP95:    m.calculatePercentile(m.responseLatencies, 0.95),
		ResponseLatencyP99:    m.calculatePercentile(m.responseLatencies, 0.99),

		GenerationsTotal:   m.generationsTotal,
		GenerationsSuccess: m.generationsSuccess,
		GenerationsFailed:  m.generationsFailed,
		GenerationSuccessRate: m.calculateGenerationSuccessRate(),

		FallbackTriggers: m.fallbackTriggers,
	}
}

// LogMetrics logs current metrics
func (m *MetricsCollector) LogMetrics() {
	metrics := m.GetMetrics()

	log.Println("========== AI Agent Metrics ==========")
	log.Printf("Conversations: Started=%d, Completed=%d, Abandoned=%d (Avg Turns: %.2f)",
		metrics.ConversationsStarted, metrics.ConversationsCompleted, metrics.ConversationsAbandoned, metrics.AverageConversationTurns)
	log.Printf("Tool Calls: Total=%d, Success=%d, Failed=%d (Success Rate: %.2f%%)",
		metrics.ToolCallsTotal, metrics.ToolCallsSuccess, metrics.ToolCallsFailed, metrics.ToolSuccessRate*100)
	log.Printf("Generations: Total=%d, Success=%d, Failed=%d (Success Rate: %.2f%%)",
		metrics.GenerationsTotal, metrics.GenerationsSuccess, metrics.GenerationsFailed, metrics.GenerationSuccessRate*100)
	log.Printf("Tokens: Total=%d, Cost=R$ %.4f, Avg Cost/Conv=R$ %.4f",
		metrics.TotalTokensUsed, metrics.TotalCost, metrics.AverageCostPerConv)
	log.Printf("Response Latency: p50=%.2fs, p95=%.2fs, p99=%.2fs",
		metrics.ResponseLatencyP50.Seconds(), metrics.ResponseLatencyP95.Seconds(), metrics.ResponseLatencyP99.Seconds())
	log.Printf("Fallback Triggers: %d", metrics.FallbackTriggers)
	log.Println("======================================")
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

// EstimateTokenCost estimates the cost of tokens for Gemini
// Pricing as of 2026 (update as needed)
func EstimateTokenCost(tokens int, model string) float64 {
	// Gemini pricing (example - update with actual pricing)
	// gemini-2.0-flash: $0.10 per 1M tokens
	// gemini-2.0-flash-exp: Free tier (for now)

	const usdToBrl = 5.0 // Approximate exchange rate

	switch model {
	case "gemini-2.0-flash":
		return (float64(tokens) / 1000000.0) * 0.10 * usdToBrl
	case "gemini-2.0-flash-exp":
		return 0 // Free tier
	case "gemini-1.5-flash":
		return (float64(tokens) / 1000000.0) * 0.075 * usdToBrl
	case "gemini-1.5-pro":
		return (float64(tokens) / 1000000.0) * 1.25 * usdToBrl
	default:
		return (float64(tokens) / 1000000.0) * 0.10 * usdToBrl
	}
}
