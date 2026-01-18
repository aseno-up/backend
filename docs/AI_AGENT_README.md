# AI-Driven WhatsApp Message System

A comprehensive AI agent system for handling WhatsApp conversations in Brazilian medical clinics using Gemini AI with function calling.

## Overview

This system replaces hardcoded message templates with dynamic AI-generated responses, enabling natural multi-turn conversations while maintaining full control over business logic through function calling.

### Key Features

✅ **10 AI-callable tools** for appointment management
✅ **Multi-turn conversations** with context management
✅ **Natural Portuguese** responses with appropriate emojis
✅ **Feature flags** for gradual rollout
✅ **Circuit breaker** fallback mechanism
✅ **Comprehensive metrics** and monitoring
✅ **Redis caching** for fast conversation retrieval
✅ **PostgreSQL** for persistent storage

## Architecture

```
User Message → Conversation Manager → AI Agent → Tool Execution → Response Generation
                     ↓                    ↓            ↓                 ↓
                  Redis Cache         Gemini API    Business Logic    Segmented Messages
                  PostgreSQL          (Function     (Your Services)    WhatsApp Queue
                                      Calling)
```

### Components

1. **Conversation Management** (`pkg/whatsapp/conversation/`)
   - `context.go` - Conversation state and history models
   - `repository.go` - Redis + PostgreSQL persistence
   - `manager.go` - Conversation lifecycle management

2. **Tool System** (`pkg/whatsapp/tools/`)
   - `registry.go` - 10 tool definitions with Gemini schemas
   - `executor.go` - Tool execution and service integration

3. **AI Agent** (`pkg/whatsapp/agent/`)
   - `agent.go` - Agent interface
   - `gemini_agent.go` - Gemini implementation with function calling
   - `prompt.go` - System prompt management
   - `service.go` - High-level AI service orchestration
   - `metrics.go` - Metrics collection and reporting
   - `fallback.go` - Circuit breaker and fallback handling

4. **Configuration** (`internal/config/`)
   - `worker_config.go` - AI agent configuration (extended)

5. **Database** (`internal/database/migrations/`)
   - `conversation_contexts.sql` - SQL migration
   - `migrate.go` - GORM-based migration

## Available Tools

The AI agent has access to 10 tools for managing appointments:

### Slot Management
1. **get_available_professionals** - List professionals by specialty
2. **get_available_slots** - Find available time slots

### Appointment Management
3. **create_appointment** - Book new appointment
4. **confirm_appointment** - Confirm pending appointment
5. **cancel_appointment** - Cancel appointment
6. **reschedule_appointment** - Reschedule to new time
7. **get_patient_appointments** - List patient's appointments

### Information
8. **get_clinic_info** - Get clinic details
9. **get_professional_info** - Get professional details
10. **get_patient_info** - Get current patient info

## Quick Start

### 1. Database Setup

Run the migration:

```bash
# Option 1: SQL
psql -U postgres -d aseno_db -f internal/database/migrations/conversation_contexts.sql

# Option 2: GORM (add to your init code)
migrations.MigrateConversations(db)
```

### 2. Environment Configuration

Add to `.env`:

```bash
# Enable AI Agent
AI_AGENT_ENABLED=true
AI_AGENT_PROVIDER=gemini
GEMINI_API_KEY=your-api-key-here

# Model Configuration
AI_AGENT_MODEL=gemini-2.0-flash-exp
AI_AGENT_FALLBACK_MODEL=gemini-2.0-flash
AI_AGENT_MAX_TOKENS=2048
AI_AGENT_TEMPERATURE=0.7
AI_AGENT_TIMEOUT=30s

# Conversation Storage
CONVERSATION_REDIS_TTL=30m
CONVERSATION_POSTGRES_TTL=168h

# Feature Flags (start with scheduling only)
AI_AGENT_SCHEDULING_ENABLED=true
AI_AGENT_CONFIRM_ENABLED=false
AI_AGENT_CANCEL_ENABLED=false
AI_AGENT_REMINDERS_ENABLED=false
```

### 3. Integration

See `INTEGRATION_GUIDE.md` for detailed integration steps.

## Example Conversations

### Scheduling Flow

```
User: "Oi, quero marcar uma consulta"
AI: "Olá! 😊 Como posso ajudar? Temos fisioterapeuta e psicólogo. Qual prefere?"

User: "fisio"
AI: [calls get_available_slots]
    "Encontrei estes horários com Dr. João:
     1. Segunda 22/01 às 14:00
     2. Quarta 24/01 às 10:00
     3. Sexta 26/01 às 16:00

     Qual prefere?"

User: "quarta"
AI: [calls create_appointment]
    "Perfeito! ✅ Seu agendamento está confirmado:
     📅 Quarta 24/01 às 10:00 com Dr. João
     🏥 Nossa Clínica

     Te aguardamos! 😊"
```

### Confirmation Flow

```
User: "Sim, confirmo"
AI: [calls get_patient_appointments, then confirm_appointment]
    "Ótimo! ✅ Sua consulta de Sexta 26/01 às 16:00 com Dra. Maria está confirmada.

     Nos vemos lá! 😊"
```

## Monitoring

### Metrics Collected

- **Conversations**: Started, completed, abandoned, avg turns
- **Tool Calls**: Total, success rate, latency per tool
- **AI Performance**: Tokens used, cost, response latency (p50/p95/p99)
- **Reliability**: Fallback triggers, error rates

### Logging Metrics

```go
metricsCollector.LogMetrics()
```

Output:
```
========== AI Agent Metrics ==========
Conversations: Started=150, Completed=135, Abandoned=15 (Avg Turns: 3.2)
Tool Calls: Total=450, Success=445, Failed=5 (Success Rate: 98.89%)
Generations: Total=150, Success=148, Failed=2 (Success Rate: 98.67%)
Tokens: Total=45000, Cost=R$ 0.00, Avg Cost/Conv=R$ 0.00
Response Latency: p50=1.2s, p95=2.8s, p99=4.5s
Fallback Triggers: 2
======================================
```

## Fallback Mechanism

The system includes a circuit breaker that automatically falls back to traditional handlers when:

- AI agent fails 5+ times consecutively
- Circuit breaker stays open for 5 minutes
- Individual requests can retry up to 3 times

```go
fallbackHandler := agent.NewFallbackHandler(5, 5*time.Minute)

if fallbackHandler.ShouldUseFallback() {
    // Use traditional handler
} else {
    // Use AI agent
}
```

## Cost Management

### Estimated Costs

- **gemini-2.0-flash-exp**: Free tier (currently)
- **gemini-2.0-flash**: ~R$ 0.03 per conversation
- **gemini-1.5-flash**: ~R$ 0.04 per conversation

Average conversation: ~300-500 tokens

### Cost Tracking

```go
cost := agent.EstimateTokenCost(tokensUsed, model)
metricsCollector.RecordGeneration(true, tokensUsed, cost, latency)
```

## Gradual Rollout Strategy

### Phase 1: Internal Testing (Week 1)
- Enable only scheduling flow
- Test with 1-2 internal clinics
- Monitor closely for issues

### Phase 2: Beta Testing (Weeks 2-3)
- Enable scheduling + confirmation
- Expand to 3-5 beta clinics
- Gather feedback, refine prompts

### Phase 3: Production Rollout (Weeks 4-8)
- Week 4: 10% of clinics
- Week 5: 25% of clinics
- Week 6: 50% of clinics
- Week 8: 100% rollout

At each phase, verify:
- Error rate < 5%
- Latency p95 < 5s
- Appointment completion rate ≥ baseline
- No critical bugs for 1 week

## Troubleshooting

### AI Agent Not Working

1. Check `AI_AGENT_ENABLED=true`
2. Verify Gemini API key is set
3. Check feature flags for the specific flow
4. Review logs for error messages

### High Latency

1. Check Gemini API response times
2. Verify Redis is connected
3. Consider using `gemini-2.0-flash` instead of `exp`
4. Check if circuit breaker is open

### Cost Spike

1. Review `metricsCollector.TotalCost`
2. Check `AI_AGENT_MAX_TOKENS` setting
3. Review system prompts - make them more concise
4. Monitor token usage per conversation

### Tool Failures

1. Check `tool_call_logs` table for errors
2. Verify service dependencies are correctly injected
3. Validate tool parameter formats
4. Check business logic errors (e.g., no slots available)

## Development

### Running Tests

```bash
# Unit tests
go test ./pkg/whatsapp/conversation/...
go test ./pkg/whatsapp/tools/...
go test ./pkg/whatsapp/agent/...

# Integration tests (TODO)
go test ./test/integration/...
```

### Adding New Tools

1. Define tool in `tools/registry.go`:
```go
r.register(&Tool{
    Name:        "my_new_tool",
    Description: "Description in Portuguese",
    Parameters:  &genai.Schema{...},
})
```

2. Implement handler in `tools/executor.go`:
```go
case "my_new_tool":
    return e.myNewTool(execCtx, params)
```

3. Add service interface and inject dependency

### Customizing System Prompt

Edit `agent/prompt.go` to customize:
- Personality and tone
- Conversation flow guidelines
- Tool usage instructions
- Formatting rules

## Files Created

```
pkg/whatsapp/
├── conversation/
│   ├── context.go        - Conversation models
│   ├── repository.go     - Redis + PostgreSQL storage
│   └── manager.go        - Conversation management
├── tools/
│   ├── registry.go       - Tool definitions
│   └── executor.go       - Tool execution
└── agent/
    ├── agent.go          - Agent interface
    ├── gemini_agent.go   - Gemini implementation
    ├── prompt.go         - System prompts
    ├── service.go        - Agent service
    ├── metrics.go        - Metrics collection
    └── fallback.go       - Fallback handling

internal/
├── config/
│   └── worker_config.go  - Extended with AI config
└── database/migrations/
    ├── conversation_contexts.sql  - SQL migration
    └── migrate.go                 - GORM migration

Documentation:
├── AI_AGENT_README.md           - This file
├── INTEGRATION_GUIDE.md         - Integration instructions
└── .env.ai-agent.example        - Environment config example
```

## Next Steps

1. **Complete Integration** - Wire AI agent into WhatsApp service
2. **Add Tests** - Comprehensive unit and integration tests
3. **Monitor Metrics** - Set up dashboards (Grafana, etc.)
4. **Optimize Prompts** - Refine based on real conversations
5. **A/B Testing** - Compare AI vs traditional completion rates
6. **Add Claude/OpenAI** - Implement alternative providers
7. **Smart Segmentation** - Improve message segmentation logic
8. **Cost Optimization** - Fine-tune token usage

## Support

For questions or issues:
- Review `INTEGRATION_GUIDE.md` for setup
- Check conversation logs in `conversation_contexts` table
- Review tool call logs in `tool_call_logs` table
- Monitor metrics with `metricsCollector.LogMetrics()`

---

**Status**: ✅ Phase 1 (Foundation) Complete
**Next**: Integration and testing
**Version**: 1.0.0
**Last Updated**: 2026-01-18
