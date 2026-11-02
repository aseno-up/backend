package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/user/model"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// SQLite doesn't support 'gen_random_uuid()' default
	err = db.Exec(`CREATE TABLE users (
		id text PRIMARY KEY,
		clinic_id text NOT NULL,
		email text NOT NULL UNIQUE,
		password_hash text,
		name text NOT NULL,
		phone text,
		avatar_url text,
		role text NOT NULL DEFAULT 'receptionist',
		google_id text,
		refresh_token text,
		email_verified numeric DEFAULT 0,
		is_active numeric DEFAULT 1,
		last_login_at datetime,
		professional_id text,
		tutorials_completed text DEFAULT '{}',
		created_at datetime,
		updated_at datetime,
		deleted_at datetime
	)`).Error
	require.NoError(t, err)

	return db
}

func TestUserRepository_GetByClinicID_PaginationAndFilters(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	clinicA := uuid.New()
	clinicB := uuid.New()

	// Seed data
	usersA := []*model.User{
		{ID: uuid.New(), ClinicID: clinicA, Name: "Alice", Email: "alice@test.com", Role: "admin", IsActive: true, CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: uuid.New(), ClinicID: clinicA, Name: "Bob", Email: "bob@test.com", Role: "professional", IsActive: true, CreatedAt: time.Now().Add(-1 * time.Hour)},
		{ID: uuid.New(), ClinicID: clinicA, Name: "Charlie", Email: "charlie@test.com", Role: "receptionist", IsActive: true, CreatedAt: time.Now()},
	}

	for _, u := range usersA {
		db.Create(u)
	}

	// Clinic B user to ensure isolation
	db.Create(&model.User{ID: uuid.New(), ClinicID: clinicB, Name: "David", Email: "david@test.com", Role: "admin", IsActive: true})

	t.Run("Isolamento por ClinicID e contagem total", func(t *testing.T) {
		filter := pagination.Filter{Page: 1, Limit: 10}
		res, total, err := repo.GetByClinicID(ctx, clinicA, filter)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, res, 3)
	})

	t.Run("Paginação limit", func(t *testing.T) {
		filter := pagination.Filter{Page: 1, Limit: 2}
		res, total, err := repo.GetByClinicID(ctx, clinicA, filter)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total) // Total is still 3
		assert.Len(t, res, 2)            // Only 2 returned
	})

	t.Run("Busca por termo", func(t *testing.T) {
		filter := pagination.Filter{Page: 1, Limit: 10, Search: "Bob"}
		res, total, err := repo.GetByClinicID(ctx, clinicA, filter)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "Bob", res[0].Name)
	})

	t.Run("Filtro por papel (Status/Role)", func(t *testing.T) {
		filter := pagination.Filter{Page: 1, Limit: 10, Status: "receptionist"}
		res, total, err := repo.GetByClinicID(ctx, clinicA, filter)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, "Charlie", res[0].Name)
	})

	t.Run("Ordenação crescente", func(t *testing.T) {
		filter := pagination.Filter{Page: 1, Limit: 10, SortBy: "name_asc"}
		res, _, err := repo.GetByClinicID(ctx, clinicA, filter)
		require.NoError(t, err)
		assert.Equal(t, "Alice", res[0].Name)
		assert.Equal(t, "Bob", res[1].Name)
		assert.Equal(t, "Charlie", res[2].Name)
	})

	t.Run("Ordenação decrescente", func(t *testing.T) {
		filter := pagination.Filter{Page: 1, Limit: 10, SortBy: "name_desc"}
		res, _, err := repo.GetByClinicID(ctx, clinicA, filter)
		require.NoError(t, err)
		assert.Equal(t, "Charlie", res[0].Name)
		assert.Equal(t, "Bob", res[1].Name)
		assert.Equal(t, "Alice", res[2].Name)
	})
}
