package service

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/pkg/audit/model"
	"github.com/aseno/backend/pkg/audit/repository"
)

// AuditService defines the interface for audit logging
type AuditService interface {
	Log(ctx context.Context, entry AuditEntry) error
	LogFromGin(c *gin.Context, action model.AuditAction, entityType model.AuditEntityType, entityID string, oldData, newData interface{}) error
	List(ctx context.Context, clinicID uuid.UUID, filter *model.AuditLogFilter) (*model.AuditLogResponse, error)
}

// AuditEntry represents a structured audit entry
type AuditEntry struct {
	ClinicID   uuid.UUID
	UserID     uuid.UUID
	UserEmail  string
	UserRole   string
	Action     model.AuditAction
	EntityType model.AuditEntityType
	EntityID   string
	OldData    interface{}
	NewData    interface{}
	IP         string
	UserAgent  string
}

type auditService struct {
	repo repository.AuditRepository
}

// NewAuditService creates a new audit service
func NewAuditService(repo repository.AuditRepository) AuditService {
	return &auditService{repo: repo}
}

func (s *auditService) Log(ctx context.Context, entry AuditEntry) error {
	log := &model.AuditLog{
		ClinicID:   entry.ClinicID,
		UserID:     entry.UserID,
		UserEmail:  entry.UserEmail,
		UserRole:   entry.UserRole,
		Action:     entry.Action,
		EntityType: entry.EntityType,
		EntityID:   entry.EntityID,
		IP:         entry.IP,
		UserAgent:  entry.UserAgent,
	}

	if entry.OldData != nil {
		data, err := json.Marshal(entry.OldData)
		if err == nil {
			log.OldData = data
		}
	}

	if entry.NewData != nil {
		data, err := json.Marshal(entry.NewData)
		if err == nil {
			log.NewData = data
		}
	}

	// Log asynchronously to avoid blocking the request
	go func() {
		if err := s.repo.Create(context.Background(), log); err != nil {
			logger.Printf("[Audit] Failed to save audit log: %v", err)
		}
	}()

	return nil
}

// LogFromGin extracts user context from gin and logs the audit entry
func (s *auditService) LogFromGin(c *gin.Context, action model.AuditAction, entityType model.AuditEntityType, entityID string, oldData, newData interface{}) error {
	clinicID, _ := c.Get("clinic_id")
	userID, _ := c.Get("user_id")
	email, _ := c.Get("email")
	role, _ := c.Get("user_role")

	entry := AuditEntry{
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldData:    oldData,
		NewData:    newData,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	}

	if cid, ok := clinicID.(uuid.UUID); ok {
		entry.ClinicID = cid
	}
	if uid, ok := userID.(uuid.UUID); ok {
		entry.UserID = uid
	}
	if e, ok := email.(string); ok {
		entry.UserEmail = e
	}
	if r, ok := role.(string); ok {
		entry.UserRole = r
	}

	return s.Log(c.Request.Context(), entry)
}

func (s *auditService) List(ctx context.Context, clinicID uuid.UUID, filter *model.AuditLogFilter) (*model.AuditLogResponse, error) {
	return s.repo.List(ctx, clinicID, filter)
}
