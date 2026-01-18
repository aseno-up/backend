# AI Agent Integration Guide

This guide explains how to integrate the AI agent into the existing WhatsApp service.

## Overview

The AI agent system consists of:
1. **Conversation Manager** - Manages conversation state and history
2. **Tool Registry & Executor** - Defines and executes 10 tools for appointment management
3. **AI Agent (Gemini)** - Generates responses using function calling
4. **Agent Service** - Orchestrates the entire flow

## Integration Steps

### Step 1: Database Migration

Run the database migration to create required tables:

```bash
# Option 1: Use the SQL migration file
psql -U postgres -d aseno_db -f internal/database/migrations/conversation_contexts.sql

# Option 2: Use GORM AutoMigrate (add to your initialization code)
```

In your database initialization code, add:

```go
import "github.com/aseno/backend/internal/database/migrations"

// After setting up GORM db
if err := migrations.MigrateConversations(db); err != nil {
    log.Fatalf("Failed to migrate conversation tables: %v", err)
}
```

### Step 2: Update WhatsApp Service Initialization

Modify `pkg/whatsapp/service/whatsapp_service.go` to add AI agent dependencies:

```go
import (
    "github.com/aseno/backend/pkg/whatsapp/agent"
    "github.com/aseno/backend/pkg/whatsapp/conversation"
    "github.com/aseno/backend/pkg/whatsapp/tools"
)

type whatsAppService struct {
    // ... existing fields ...

    // AI Agent components (new)
    aiAgentService   agent.Service
    conversationMgr  conversation.Manager
    featureFlags     *agent.FeatureFlags
}

// Add setter method
func (s *whatsAppService) SetAIAgentService(agentSvc agent.Service, convMgr conversation.Manager, flags *agent.FeatureFlags) {
    s.aiAgentService = agentSvc
    s.conversationMgr = convMgr
    s.featureFlags = flags
}
```

### Step 3: Initialize AI Agent Stack

In your service initialization (e.g., `cmd/worker/main.go` or wherever you set up dependencies):

```go
import (
    "github.com/aseno/backend/pkg/whatsapp/agent"
    "github.com/aseno/backend/pkg/whatsapp/conversation"
    "github.com/aseno/backend/pkg/whatsapp/tools"
)

func initializeAIAgent(cfg *config.WorkerConfig, db *gorm.DB, redisClient *redis.Client) (agent.Service, error) {
    // Skip if AI agent is disabled
    if !cfg.AIAgentEnabled {
        return nil, nil
    }

    // 1. Create conversation repository
    conversationRepo := conversation.NewRepository(
        db,
        redisClient,
        cfg.ConversationRedisTTL,
        cfg.ConversationPostgresTTL,
    )

    // 2. Create conversation manager
    conversationMgr := conversation.NewManager(
        conversationRepo,
        cfg.ConversationRedisTTL,
        cfg.ConversationPostgresTTL,
    )

    // 3. Create tool registry (defines all 10 tools)
    toolRegistry := tools.NewRegistry()

    // 4. Create execution context (will be updated per request)
    execContext := &tools.ExecutionContext{
        // Service dependencies will be injected from WhatsApp service
        // These are the same dependencies used by the traditional flow
    }

    // 5. Create tool executor
    toolExecutor := tools.NewExecutor(toolRegistry)

    // 6. Create AI agent (Gemini)
    agentConfig := agent.Config{
        Provider:      cfg.AIAgentProvider,
        APIKey:        os.Getenv("GEMINI_API_KEY"),
        Model:         cfg.AIAgentModel,
        FallbackModel: cfg.AIAgentFallbackModel,
        MaxTokens:     cfg.AIAgentMaxTokens,
        Temperature:   cfg.AIAgentTemperature,
        Timeout:       cfg.AIAgentTimeout,
    }

    aiAgent, err := agent.NewGeminiAgent(agentConfig, toolRegistry, toolExecutor, execContext)
    if err != nil {
        return nil, fmt.Errorf("failed to create AI agent: %w", err)
    }

    // 7. Create feature flags
    featureFlags := &agent.FeatureFlags{
        SchedulingEnabled: cfg.AIAgentSchedulingEnabled,
        ConfirmEnabled:    cfg.AIAgentConfirmEnabled,
        CancelEnabled:     cfg.AIAgentCancelEnabled,
        RemindersEnabled:  cfg.AIAgentRemindersEnabled,
    }

    // 8. Create AI agent service
    agentService := agent.NewService(
        aiAgent,
        conversationMgr,
        toolRegistry,
        toolExecutor,
        execContext,
        featureFlags,
    )

    return agentService, nil
}
```

### Step 4: Wire AI Agent to WhatsApp Service

After creating the WhatsApp service, inject the AI agent:

```go
// Create WhatsApp service
whatsappService := service.NewWhatsAppService(client, messageRepo)

// ... set all existing dependencies (appointmentFinder, slotSuggester, etc.) ...

// Initialize AI agent stack
aiAgentService, err := initializeAIAgent(cfg, db, redisClient)
if err != nil {
    log.Fatalf("Failed to initialize AI agent: %v", err)
}

if aiAgentService != nil {
    // Get the execution context and inject service dependencies
    // These are the same dependencies the traditional flow uses
    execContext.ProfessionalLister = professionalService
    execContext.SlotFinder = slotService
    execContext.AppointmentCreator = appointmentService
    execContext.AppointmentFinder = appointmentService
    execContext.AppointmentConfirmer = appointmentService
    execContext.AppointmentCanceller = appointmentService
    execContext.AppointmentRescheduler = appointmentService
    execContext.ClinicInfoProvider = clinicService
    execContext.ProfessionalInfoProvider = professionalService
    execContext.PatientInfoProvider = patientService
    execContext.PatientFinderCreator = patientService

    // Set AI agent service
    whatsappService.SetAIAgentService(aiAgentService, conversationMgr, &agent.FeatureFlags{
        SchedulingEnabled: cfg.AIAgentSchedulingEnabled,
        ConfirmEnabled:    cfg.AIAgentConfirmEnabled,
        CancelEnabled:     cfg.AIAgentCancelEnabled,
        RemindersEnabled:  cfg.AIAgentRemindersEnabled,
    })
}
```

### Step 5: Modify HandleIncomingMessage

Update the `HandleIncomingMessage` method in `whatsapp_service.go`:

```go
func (s *whatsAppService) HandleIncomingMessage(ctx context.Context, clinicID uuid.UUID, msg *model.ParsedIncomingMessage) error {
    // Log incoming message
    s.logMessage(ctx, clinicID, nil, msg.Phone, model.MessageDirectionInbound, msg.Content, model.MessageStatusDelivered, msg.MessageID)

    // Classify intent (existing code)
    intent := s.ClassifyMessage(msg.Content)

    // ... handle button responses (existing code) ...

    // NEW: Check if AI agent should handle this message
    if s.aiAgentService != nil && s.aiAgentService.ShouldUseAIAgent(intent) {
        return s.handleWithAIAgent(ctx, clinicID, msg, intent)
    }

    // FALLBACK: Use traditional handler
    // ... existing switch case for intent handling ...
}

// NEW METHOD: Handle message with AI agent
func (s *whatsAppService) handleWithAIAgent(ctx context.Context, clinicID uuid.UUID, msg *model.ParsedIncomingMessage, intent model.MessageIntent) error {
    // Get clinic name (you'll need to add this method)
    clinicName := s.getClinicName(ctx, clinicID)
    if clinicName == "" {
        clinicName = "nossa clínica" // Fallback
    }

    // Create agent request
    agentReq := &agent.MessageRequest{
        ClinicID:     clinicID,
        ClinicName:   clinicName,
        PatientPhone: msg.Phone,
        PatientName:  msg.PushName, // WhatsApp push name
        MessageText:  msg.Content,
        Intent:       intent,
    }

    // Process with AI agent
    agentResp, err := s.aiAgentService.ProcessMessage(ctx, agentReq)
    if err != nil {
        log.Printf("[WhatsApp Service] AI agent failed: %v, falling back to traditional handler", err)
        // TODO: Implement fallback to traditional handler
        return err
    }

    // Send response segments with delays
    return s.sendSegmentedResponse(ctx, msg.Instance, msg.Phone, agentResp.Segments)
}

// NEW METHOD: Send segmented response
func (s *whatsAppService) sendSegmentedResponse(ctx context.Context, instance, phone string, segments []*model.MessageSegment) error {
    for _, segment := range segments {
        // Send segment
        if _, err := s.SendTextMessage(ctx, instance, phone, segment.Content); err != nil {
            return fmt.Errorf("failed to send segment: %w", err)
        }

        // Delay if specified
        if segment.DelayAfter > 0 {
            time.Sleep(segment.DelayAfter)
        }
    }

    return nil
}
```

## Environment Configuration

Add to your `.env` file:

```bash
# AI Agent Configuration
AI_AGENT_ENABLED=true
AI_AGENT_PROVIDER=gemini
GEMINI_API_KEY=your-gemini-api-key-here

AI_AGENT_MODEL=gemini-2.0-flash-exp
AI_AGENT_FALLBACK_MODEL=gemini-2.0-flash
AI_AGENT_MAX_TOKENS=2048
AI_AGENT_TEMPERATURE=0.7
AI_AGENT_TIMEOUT=30s

CONVERSATION_REDIS_TTL=30m
CONVERSATION_POSTGRES_TTL=168h

# Feature Flags - Start with scheduling only
AI_AGENT_SCHEDULING_ENABLED=true
AI_AGENT_CONFIRM_ENABLED=false
AI_AGENT_CANCEL_ENABLED=false
AI_AGENT_REMINDERS_ENABLED=false
```

## Service Interface Requirements

Your existing services need to implement these interfaces (defined in `tools/executor.go`):

- `ProfessionalLister` - List available professionals
- `SlotFinder` - Find available time slots
- `AppointmentCreator` - Create new appointments
- `AppointmentFinder` - Find patient appointments
- `AppointmentConfirmer` - Confirm appointments
- `AppointmentCanceller` - Cancel appointments
- `AppointmentRescheduler` - Reschedule appointments
- `ClinicInfoProvider` - Get clinic info
- `ProfessionalInfoProvider` - Get professional details
- `PatientInfoProvider` - Get patient info
- `PatientFinderCreator` - Find or create patient

Most of these likely already exist in your services - just ensure they match the interface signatures.

## Testing

### 1. Unit Tests

Test individual components:

```bash
go test ./pkg/whatsapp/conversation/...
go test ./pkg/whatsapp/tools/...
go test ./pkg/whatsapp/agent/...
```

### 2. Integration Test

Send a test message:

```
User: "Oi, quero marcar uma consulta"
Expected: AI agent responds with greeting and asks for specialty/professional
```

### 3. Monitor Logs

Watch for these log entries:

```
[Conversation Manager] Created new conversation {id} for phone {phone}
[Gemini Agent] Executing tool: get_available_professionals
[Gemini Agent] Generated response with {n} tool calls
[AI Agent Service] Processed message in {time}
```

## Gradual Rollout Strategy

### Phase 1: Internal Testing (Week 1)
```bash
AI_AGENT_ENABLED=true
AI_AGENT_SCHEDULING_ENABLED=true
# Test with your own clinic
```

### Phase 2: Beta Testing (Week 2-3)
```bash
# Enable for 3-5 test clinics
# Monitor metrics closely
```

### Phase 3: Production Rollout (Week 4-8)
```bash
# Week 4: 10% of clinics
# Week 5: 25% of clinics
# Week 6: 50% of clinics
# Week 8: 100% of clinics
```

## Monitoring

Track these metrics:

- **Conversation metrics**
  - Conversations started per day
  - Average conversation length (turns)
  - Completion rate
  - Abandonment rate

- **Tool metrics**
  - Tool call success rate per tool
  - Average tool execution time
  - Tool errors by type

- **AI metrics**
  - Tokens used per conversation
  - Average cost per conversation
  - Response latency (p50, p95, p99)
  - Fallback trigger rate

- **Business metrics**
  - Appointments created via AI
  - Confirmation rate
  - Cancellation rate

## Troubleshooting

### AI Agent Not Responding

1. Check `AI_AGENT_ENABLED=true`
2. Check feature flag for the intent type
3. Check Gemini API key is set
4. Check logs for errors

### Tool Execution Failures

1. Verify service dependencies are injected in `ExecutionContext`
2. Check tool parameters match expected format
3. Review tool call logs in `tool_call_logs` table

### High Latency

1. Check Gemini API response times
2. Verify Redis is connected (for conversation caching)
3. Consider using faster model (gemini-2.0-flash)

### Cost Too High

1. Review token usage per conversation
2. Reduce MAX_TOKENS if responses are too long
3. Optimize system prompts to be more concise

## Next Steps

After completing integration:

1. **Add Monitoring** - Implement metrics collection (see `MONITORING_GUIDE.md`)
2. **Add Fallback** - Implement robust fallback to traditional flow
3. **Add Tests** - Comprehensive unit and integration tests
4. **Optimize Prompts** - Refine system prompts based on real conversations
5. **A/B Testing** - Compare AI vs traditional completion rates

## Support

For issues or questions:
- Review logs in `conversation_contexts` and `tool_call_logs` tables
- Check the `.env.ai-agent.example` file for configuration examples
- Refer to the plan document for architectural details
