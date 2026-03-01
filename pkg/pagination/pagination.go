package pagination

import (
	"math"

	"gorm.io/gorm"
)

// Meta contains pagination metadata.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// Filter represents the standard fields for listing.
type Filter struct {
	Limit  int    `form:"limit"`
	Page   int    `form:"page"`
	SortBy string `form:"sort_by"`
	Search string `form:"search"`
	Status string `form:"status"`
}

// GetLimit ensures a valid limit.
func (f *Filter) GetLimit() int {
	if f.Limit <= 0 {
		return 10
	}
	if f.Limit > 100 {
		return 100
	}
	return f.Limit
}

// GetPage ensures a valid page.
func (f *Filter) GetPage() int {
	if f.Page <= 0 {
		return 1
	}
	return f.Page
}

// GetOffset calculates the SQL offset.
func (f *Filter) GetOffset() int {
	return (f.GetPage() - 1) * f.GetLimit()
}

// Paginate generates the GORM scope for limits and offsets.
func Paginate(f *Filter) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(f.GetOffset()).Limit(f.GetLimit())
	}
}

// Response genetic structure
type Response[T any] struct {
	Data []T  `json:"data"`
	Meta Meta `json:"meta"`
}

// NewMeta calculates the total pages and creates the meta response.
func NewMeta(page, limit int, total int64) Meta {
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages == 0 && total > 0 {
		totalPages = 1
	} else if totalPages == 0 {
		totalPages = 0
	}
	return Meta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}
}
