package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Repository defines the interface for conversation context persistence
type Repository interface {
	// Create creates a new conversation context
	Create(ctx context.Context, conversation *ConversationContext) error

	// GetByID retrieves a conversation by ID
	GetByID(ctx context.Context, id uuid.UUID) (*ConversationContext, error)

	// GetActiveByPhone retrieves the active conversation for a patient phone in a clinic
	GetActiveByPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*ConversationContext, error)

	// Update updates an existing conversation
	Update(ctx context.Context, conversation *ConversationContext) error

	// Delete deletes a conversation
	Delete(ctx context.Context, id uuid.UUID) error

	// ExpireOld expires old conversations based on ExpiresAt
	ExpireOld(ctx context.Context) error

	// LogToolCall logs a tool execution
	LogToolCall(ctx context.Context, log *ToolCallLog) error

	// GetToolCallLogs retrieves tool call logs for a conversation
	GetToolCallLogs(ctx context.Context, conversationID uuid.UUID) ([]*ToolCallLog, error)
}

// conversationRepository implements Repository with Redis cache and PostgreSQL fallback
type conversationRepository struct {
	db          *gorm.DB
	redis       *redis.Client
	redisTTL    time.Duration
	postgresTTL time.Duration
}

// NewRepository creates a new conversation repository
func NewRepository(db *gorm.DB, redisClient *redis.Client, redisTTL, postgresTTL time.Duration) Repository {
	return &conversationRepository{
		db:          db,
		redis:       redisClient,
		redisTTL:    redisTTL,
		postgresTTL: postgresTTL,
	}
}

// Create creates a new conversation context
func (r *conversationRepository) Create(ctx context.Context, conversation *ConversationContext) error {
	// Set default values
	if conversation.ID == uuid.Nil {
		conversation.ID = uuid.New()
	}
	if conversation.CreatedAt.IsZero() {
		conversation.CreatedAt = time.Now()
	}
	if conversation.UpdatedAt.IsZero() {
		conversation.UpdatedAt = time.Now()
	}
	if conversation.ExpiresAt.IsZero() {
		conversation.ExpiresAt = time.Now().Add(r.postgresTTL)
	}
	if conversation.History == nil {
		conversation.History = []ConversationTurn{}
	}
	if conversation.PendingActions == nil {
		conversation.PendingActions = map[string]interface{}{}
	}
	if conversation.Metadata == nil {
		conversation.Metadata = map[string]interface{}{}
	}

	// Save to PostgreSQL
	if err := r.db.WithContext(ctx).Create(conversation).Error; err != nil {
		return fmt.Errorf("failed to create conversation in database: %w", err)
	}

	// Cache in Redis
	if r.redis != nil {
		if err := r.cacheConversation(ctx, conversation); err != nil {
			// Log error but don't fail - Redis is a cache
			fmt.Printf("Warning: failed to cache conversation in Redis: %v\n", err)
		}
	}

	return nil
}

// GetByID retrieves a conversation by ID
func (r *conversationRepository) GetByID(ctx context.Context, id uuid.UUID) (*ConversationContext, error) {
	// Try Redis first
	if r.redis != nil {
		if conversation, err := r.getFromCache(ctx, id); err == nil {
			return conversation, nil
		}
	}

	// Fallback to PostgreSQL
	var conversation ConversationContext
	if err := r.db.WithContext(ctx).First(&conversation, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// Cache in Redis for next time
	if r.redis != nil {
		_ = r.cacheConversation(ctx, &conversation)
	}

	return &conversation, nil
}

// GetActiveByPhone retrieves the active conversation for a patient phone in a clinic
func (r *conversationRepository) GetActiveByPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*ConversationContext, error) {
	// Try to get from Redis using a secondary key
	cacheKey := r.phoneKey(clinicID, phone)
	if r.redis != nil {
		// Get conversation ID from phone key
		idStr, err := r.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			if id, err := uuid.Parse(idStr); err == nil {
				if conversation, err := r.getFromCache(ctx, id); err == nil && !conversation.IsExpired() {
					return conversation, nil
				}
			}
		}
	}

	// Fallback to PostgreSQL
	var conversation ConversationContext
	err := r.db.WithContext(ctx).
		Where("clinic_id = ? AND patient_phone = ? AND expires_at > ?", clinicID, phone, time.Now()).
		Order("created_at DESC").
		First(&conversation).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no active conversation found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// Cache in Redis for next time
	if r.redis != nil {
		_ = r.cacheConversation(ctx, &conversation)
		_ = r.redis.Set(ctx, cacheKey, conversation.ID.String(), r.redisTTL).Err()
	}

	return &conversation, nil
}

// Update updates an existing conversation
func (r *conversationRepository) Update(ctx context.Context, conversation *ConversationContext) error {
	conversation.UpdatedAt = time.Now()

	// Update in PostgreSQL
	if err := r.db.WithContext(ctx).Save(conversation).Error; err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}

	// Update Redis cache
	if r.redis != nil {
		if err := r.cacheConversation(ctx, conversation); err != nil {
			fmt.Printf("Warning: failed to update conversation cache: %v\n", err)
		}
		// Update phone key mapping
		cacheKey := r.phoneKey(conversation.ClinicID, conversation.PatientPhone)
		_ = r.redis.Set(ctx, cacheKey, conversation.ID.String(), r.redisTTL).Err()
	}

	return nil
}

// Delete deletes a conversation
func (r *conversationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Get conversation first to clear phone key
	conversation, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete from PostgreSQL
	if err := r.db.WithContext(ctx).Delete(&ConversationContext{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	// Delete from Redis
	if r.redis != nil {
		_ = r.redis.Del(ctx, r.conversationKey(id)).Err()
		_ = r.redis.Del(ctx, r.phoneKey(conversation.ClinicID, conversation.PatientPhone)).Err()
	}

	return nil
}

// ExpireOld expires old conversations
func (r *conversationRepository) ExpireOld(ctx context.Context) error {
	result := r.db.WithContext(ctx).
		Model(&ConversationContext{}).
		Where("expires_at <= ? AND state != ?", time.Now(), ConversationStateExpired).
		Update("state", ConversationStateExpired)

	if result.Error != nil {
		return fmt.Errorf("failed to expire old conversations: %w", result.Error)
	}

	return nil
}

// LogToolCall logs a tool execution
func (r *conversationRepository) LogToolCall(ctx context.Context, log *ToolCallLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	// Convert Parameters to proper type for GORM
	if log.Parameters == nil {
		log.Parameters = map[string]interface{}{}
	}

	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to log tool call: %w", err)
	}

	return nil
}

// GetToolCallLogs retrieves tool call logs for a conversation
func (r *conversationRepository) GetToolCallLogs(ctx context.Context, conversationID uuid.UUID) ([]*ToolCallLog, error) {
	var logs []*ToolCallLog
	if err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get tool call logs: %w", err)
	}

	return logs, nil
}

// Helper methods

func (r *conversationRepository) conversationKey(id uuid.UUID) string {
	return fmt.Sprintf("conversation:%s", id.String())
}

func (r *conversationRepository) phoneKey(clinicID uuid.UUID, phone string) string {
	return fmt.Sprintf("conversation:phone:%s:%s", clinicID.String(), phone)
}

func (r *conversationRepository) cacheConversation(ctx context.Context, conversation *ConversationContext) error {
	data, err := json.Marshal(conversation)
	if err != nil {
		return err
	}

	key := r.conversationKey(conversation.ID)
	return r.redis.Set(ctx, key, data, r.redisTTL).Err()
}

func (r *conversationRepository) getFromCache(ctx context.Context, id uuid.UUID) (*ConversationContext, error) {
	key := r.conversationKey(id)
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var conversation ConversationContext
	if err := json.Unmarshal([]byte(data), &conversation); err != nil {
		return nil, err
	}

	return &conversation, nil
}
