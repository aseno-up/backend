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

	"github.com/aseno/backend/pkg/finance/model"
)

// setupFinanceDB creates an in-memory sqlite with just the columns the
// transaction repository touches. Full GORM AutoMigrate can't be used here
// because the production models rely on Postgres-only defaults (gen_random_uuid).
func setupFinanceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`CREATE TABLE financial_categories (
		id text PRIMARY KEY,
		clinic_id text NOT NULL,
		name text NOT NULL,
		type text NOT NULL,
		color text,
		is_default integer DEFAULT 0,
		created_at datetime,
		updated_at datetime,
		deleted_at datetime
	)`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE financial_transactions (
		id text PRIMARY KEY,
		clinic_id text NOT NULL,
		type text NOT NULL,
		category_id text,
		amount real NOT NULL,
		description text,
		source text NOT NULL DEFAULT 'manual',
		appointment_id text,
		appointment_payment_id text,
		session_plan_id text,
		session_plan_payment_id text,
		professional_id text,
		competence_date datetime NOT NULL,
		due_date datetime,
		paid_at datetime,
		payment_method_id text,
		created_by text,
		created_at datetime,
		updated_at datetime,
		deleted_at datetime
	)`).Error)

	return db
}

// Regression: when a Lançamento has a category, List() must return it with
// the category relation loaded so the frontend can render its name/color.
// Before the fix, Category was never preloaded (nor even declared on the
// model), so the "Categoria" column on the finance screen was always blank.
func TestTransactionRepository_List_PreloadsCategory(t *testing.T) {
	db := setupFinanceDB(t)
	repo := NewTransactionRepository(db)
	ctx := context.Background()

	clinicID := uuid.New()
	catID := uuid.New()
	txID := uuid.New()
	now := time.Now()

	require.NoError(t, db.Exec(
		`INSERT INTO financial_categories (id, clinic_id, name, type, color, is_default, created_at, updated_at)
		 VALUES (?, ?, 'Aluguel', 'expense', '#FF0000', 0, ?, ?)`,
		catID.String(), clinicID.String(), now, now,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO financial_transactions (id, clinic_id, type, category_id, amount, description, source, competence_date, created_at, updated_at)
		 VALUES (?, ?, 'expense', ?, 2500, 'Aluguel abril', 'manual', ?, ?, ?)`,
		txID.String(), clinicID.String(), catID.String(), now, now, now,
	).Error)

	out, err := repo.List(ctx, clinicID, model.TransactionFilter{})
	require.NoError(t, err)
	require.Len(t, out, 1)

	got := out[0]
	assert.Equal(t, txID, got.ID)
	require.NotNil(t, got.CategoryID)
	assert.Equal(t, catID, *got.CategoryID)

	require.NotNil(t, got.Category, "Category relation must be preloaded so the frontend can show the category column")
	assert.Equal(t, "Aluguel", got.Category.Name)
	assert.Equal(t, "#FF0000", got.Category.Color)
}

func TestTransactionRepository_GetByID_PreloadsCategory(t *testing.T) {
	db := setupFinanceDB(t)
	repo := NewTransactionRepository(db)
	ctx := context.Background()

	clinicID := uuid.New()
	catID := uuid.New()
	txID := uuid.New()
	now := time.Now()

	require.NoError(t, db.Exec(
		`INSERT INTO financial_categories (id, clinic_id, name, type, color, is_default, created_at, updated_at)
		 VALUES (?, ?, 'Consulta', 'income', '#00AA00', 1, ?, ?)`,
		catID.String(), clinicID.String(), now, now,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO financial_transactions (id, clinic_id, type, category_id, amount, description, source, competence_date, created_at, updated_at)
		 VALUES (?, ?, 'income', ?, 200, 'Consulta avulsa', 'manual', ?, ?, ?)`,
		txID.String(), clinicID.String(), catID.String(), now, now, now,
	).Error)

	got, err := repo.GetByID(ctx, txID, clinicID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Category, "Category relation must be preloaded on GetByID")
	assert.Equal(t, "Consulta", got.Category.Name)
}
