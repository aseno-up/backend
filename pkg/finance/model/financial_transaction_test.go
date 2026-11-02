package model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Regression: ToResponse must surface the category name/color so the
// Lançamentos table on the finance screen can render the categoria column.
// Prior to the fix the response struct defined the fields but ToResponse
// never populated them, so every row arrived as blank.
func TestToResponse_PopulatesCategoryFieldsWhenRelationLoaded(t *testing.T) {
	catID := uuid.New()
	tx := &FinancialTransaction{
		ID:         uuid.New(),
		ClinicID:   uuid.New(),
		Type:       TransactionTypeExpense,
		CategoryID: &catID,
		Amount:     250,
		Category: &FinancialCategory{
			ID:    catID,
			Name:  "Aluguel",
			Type:  TransactionTypeExpense,
			Color: "#FF0000",
		},
	}

	resp := tx.ToResponse()

	assert.Equal(t, &catID, resp.CategoryID)
	assert.Equal(t, "Aluguel", resp.CategoryName)
	assert.Equal(t, "#FF0000", resp.CategoryColor)
}

func TestToResponse_LeavesCategoryFieldsEmptyWhenRelationMissing(t *testing.T) {
	tx := &FinancialTransaction{
		ID:       uuid.New(),
		ClinicID: uuid.New(),
		Type:     TransactionTypeIncome,
		Amount:   100,
	}

	resp := tx.ToResponse()

	assert.Nil(t, resp.CategoryID)
	assert.Empty(t, resp.CategoryName)
	assert.Empty(t, resp.CategoryColor)
}
