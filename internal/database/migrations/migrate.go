package migrations

import (
	"fmt"
	"github.com/aseno/backend/internal/logger"

	"gorm.io/gorm"
	"github.com/aseno/backend/pkg/whatsapp/conversation"
)

// MigrateConversations runs migrations for conversation-related tables
func MigrateConversations(db *gorm.DB) error {
	logger.Println("[Migration] Running conversation_contexts and tool_call_logs migrations...")

	// AutoMigrate conversation tables
	if err := db.AutoMigrate(
		&conversation.ConversationContext{},
		&conversation.ToolCallLog{},
	); err != nil {
		return fmt.Errorf("failed to migrate conversation tables: %w", err)
	}

	// Create composite index for efficient active conversation lookup
	// This index supports: clinic_id + patient_phone + expires_at filtering
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_active_conversation
		ON conversation_contexts(clinic_id, patient_phone, expires_at DESC)
		WHERE expires_at > NOW()
	`).Error; err != nil {
		logger.Printf("[Migration] Warning: failed to create idx_active_conversation: %v", err)
	}

	// Create trigger for auto-updating updated_at
	if err := db.Exec(`
		CREATE OR REPLACE FUNCTION update_conversation_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql
	`).Error; err != nil {
		logger.Printf("[Migration] Warning: failed to create update_conversation_updated_at function: %v", err)
	}

	if err := db.Exec(`
		DROP TRIGGER IF EXISTS trigger_conversation_updated_at ON conversation_contexts;
		CREATE TRIGGER trigger_conversation_updated_at
			BEFORE UPDATE ON conversation_contexts
			FOR EACH ROW
			EXECUTE FUNCTION update_conversation_updated_at()
	`).Error; err != nil {
		logger.Printf("[Migration] Warning: failed to create trigger_conversation_updated_at: %v", err)
	}

	logger.Println("[Migration] Conversation tables migrated successfully")
	return nil
}

// RollbackConversations rolls back conversation-related tables
func RollbackConversations(db *gorm.DB) error {
	logger.Println("[Migration] Rolling back conversation_contexts and tool_call_logs migrations...")

	// Drop trigger and function
	if err := db.Exec(`DROP TRIGGER IF EXISTS trigger_conversation_updated_at ON conversation_contexts`).Error; err != nil {
		logger.Printf("[Migration] Warning: failed to drop trigger: %v", err)
	}

	if err := db.Exec(`DROP FUNCTION IF EXISTS update_conversation_updated_at()`).Error; err != nil {
		logger.Printf("[Migration] Warning: failed to drop function: %v", err)
	}

	// Drop tables
	if err := db.Migrator().DropTable(&conversation.ToolCallLog{}, &conversation.ConversationContext{}); err != nil {
		return fmt.Errorf("failed to drop conversation tables: %w", err)
	}

	logger.Println("[Migration] Conversation tables rolled back successfully")
	return nil
}
