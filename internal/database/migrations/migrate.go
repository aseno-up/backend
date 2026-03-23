package migrations

import (
	"fmt"

	"github.com/aseno/backend/internal/logger"

	appointmentModel "github.com/aseno/backend/pkg/appointment/model"
	clinicModel "github.com/aseno/backend/pkg/clinic/model"
	patientModel "github.com/aseno/backend/pkg/patient/model"
	paymentModel "github.com/aseno/backend/pkg/payment/model"
	planModel "github.com/aseno/backend/pkg/plan/model"
	procedureModel "github.com/aseno/backend/pkg/procedure/model"
	professionalModel "github.com/aseno/backend/pkg/professional/model"
	sessionPlanModel "github.com/aseno/backend/pkg/sessionplan/model"
	"github.com/aseno/backend/pkg/whatsapp/conversation"
	"gorm.io/gorm"
)

// MigrateConversations runs migrations for conversation-related tables
func MigrateConversations(db *gorm.DB) error {
	logger.Println("[Migration] Running conversation_contexts and tool_call_logs migrations...")

	// AutoMigrate conversation tables
	if err := db.AutoMigrate(
		&conversation.ConversationContext{},
		&conversation.ToolCallLog{},

		// Plan models
		&planModel.Plan{},
		&planModel.PlanCycle{},
		&planModel.Offer{},
		&planModel.OfferCycle{},

		// Clinic models
		&clinicModel.Clinic{},

		// Patient models
		&patientModel.Patient{},

		// Professional models
		&professionalModel.Professional{},

		// Session Plan models
		&sessionPlanModel.SessionPlan{},

		// Payment models
		&paymentModel.Payment{},
		&paymentModel.Customer{},
		&paymentModel.Subscription{},

		// Procedure models
		&procedureModel.Procedure{},
		&appointmentModel.AppointmentProcedure{},

		// Appointment models
		&appointmentModel.Appointment{},
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
