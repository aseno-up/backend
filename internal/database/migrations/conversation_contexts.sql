-- Migration: Create conversation_contexts and tool_call_logs tables
-- Description: Support AI-driven WhatsApp conversations with context management

-- Create conversation_contexts table
CREATE TABLE IF NOT EXISTS conversation_contexts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id UUID NOT NULL,
    patient_phone VARCHAR(50) NOT NULL,
    patient_id UUID,
    patient_name VARCHAR(255),
    state VARCHAR(50) NOT NULL DEFAULT 'NEW',
    intent VARCHAR(50),
    history JSONB NOT NULL DEFAULT '[]',
    pending_actions JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,

    -- Indexes
    CONSTRAINT idx_conversation_phone UNIQUE (clinic_id, patient_phone, created_at DESC),
    INDEX idx_patient_id (patient_id),
    INDEX idx_expires_at (expires_at)
);

-- Create composite index for active conversation lookup
CREATE INDEX IF NOT EXISTS idx_active_conversation
ON conversation_contexts(clinic_id, patient_phone, expires_at DESC)
WHERE expires_at > NOW();

-- Create tool_call_logs table
CREATE TABLE IF NOT EXISTS tool_call_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL,
    tool_name VARCHAR(100) NOT NULL,
    parameters JSONB DEFAULT '{}',
    result JSONB,
    error TEXT,
    duration_ms INT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Indexes
    INDEX idx_conversation_id (conversation_id),
    INDEX idx_tool_name (tool_name),
    INDEX idx_created_at (created_at),

    -- Foreign key
    CONSTRAINT fk_conversation FOREIGN KEY (conversation_id)
        REFERENCES conversation_contexts(id) ON DELETE CASCADE
);

-- Create updated_at trigger for conversation_contexts
CREATE OR REPLACE FUNCTION update_conversation_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_conversation_updated_at
    BEFORE UPDATE ON conversation_contexts
    FOR EACH ROW
    EXECUTE FUNCTION update_conversation_updated_at();

-- Add comments for documentation
COMMENT ON TABLE conversation_contexts IS 'Stores WhatsApp conversation contexts for AI-driven interactions';
COMMENT ON COLUMN conversation_contexts.state IS 'Current state: NEW, INTENT_IDENTIFIED, SCHEDULING, AWAITING_SLOT, AWAITING_CONFIRM, CANCELLING, RESCHEDULING, QUESTION_ANSWERING, COMPLETED, EXPIRED';
COMMENT ON COLUMN conversation_contexts.intent IS 'Identified intent: schedule, confirm, cancel, reschedule, question, unknown';
COMMENT ON COLUMN conversation_contexts.history IS 'Last 10 conversation turns with role (user/assistant), content, and tool calls';
COMMENT ON COLUMN conversation_contexts.pending_actions IS 'Temporary data for multi-turn flows (selected professional, slot options, etc)';
COMMENT ON COLUMN conversation_contexts.metadata IS 'Additional metadata (provider, model, token usage, etc)';

COMMENT ON TABLE tool_call_logs IS 'Audit log of all tool/function calls made by the AI agent';
COMMENT ON COLUMN tool_call_logs.duration_ms IS 'Tool execution duration in milliseconds';
