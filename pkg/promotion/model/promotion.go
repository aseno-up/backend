package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Promotion represents a global discount promotion shown on landing pages.
// Not per-clinic: promotions are SaaS-level marketing campaigns.
//
// Invariant: at most one row can have is_active = true (enforced by partial
// unique index in migration; service also enforces it transactionally).
type Promotion struct {
	ID              uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name            string         `json:"name" gorm:"type:varchar(120);not null"`
	Description     string         `json:"description" gorm:"type:text"`
	DiscountPercent int            `json:"discount_percent" gorm:"not null"`
	StartsAt        time.Time      `json:"starts_at" gorm:"not null"`
	ExpiresAt       time.Time      `json:"expires_at" gorm:"not null"`
	IsActive        bool           `json:"is_active" gorm:"not null;default:false;uniqueIndex:idx_one_active_promo,where:is_active = true"`
	PlansAffected   pq.StringArray `json:"plans_affected" gorm:"type:text[];not null;default:'{}'"`
	BannerLabel     string         `json:"banner_label" gorm:"type:varchar(120);not null"`
	BannerShowTimer bool           `json:"banner_show_timer" gorm:"not null;default:true"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func (Promotion) TableName() string {
	return "promotions"
}

// IsWithinWindow returns true when the given time falls inside [StartsAt, ExpiresAt).
func (p *Promotion) IsWithinWindow(now time.Time) bool {
	return !now.Before(p.StartsAt) && now.Before(p.ExpiresAt)
}

// AppliesToPlan returns true when the given plan slug should receive the discount.
func (p *Promotion) AppliesToPlan(planSlug string) bool {
	for _, slug := range p.PlansAffected {
		if slug == planSlug || slug == "all" {
			return true
		}
	}
	return false
}

type CreatePromotionRequest struct {
	Name            string    `json:"name" binding:"required,min=2,max=120"`
	Description     string    `json:"description"`
	DiscountPercent int       `json:"discount_percent" binding:"required,min=1,max=90"`
	StartsAt        time.Time `json:"starts_at" binding:"required"`
	ExpiresAt       time.Time `json:"expires_at" binding:"required"`
	IsActive        bool      `json:"is_active"`
	PlansAffected   []string  `json:"plans_affected" binding:"required,min=1"`
	BannerLabel     string    `json:"banner_label" binding:"required,min=2,max=120"`
	BannerShowTimer bool      `json:"banner_show_timer"`
}

type UpdatePromotionRequest struct {
	Name            *string    `json:"name,omitempty"`
	Description     *string    `json:"description,omitempty"`
	DiscountPercent *int       `json:"discount_percent,omitempty"`
	StartsAt        *time.Time `json:"starts_at,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	IsActive        *bool      `json:"is_active,omitempty"`
	PlansAffected   *[]string  `json:"plans_affected,omitempty"`
	BannerLabel     *string    `json:"banner_label,omitempty"`
	BannerShowTimer *bool      `json:"banner_show_timer,omitempty"`
}

type PromotionListResponse struct {
	Data []Promotion `json:"data"`
}
