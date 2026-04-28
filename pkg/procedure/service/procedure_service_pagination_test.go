package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/procedure/model"
)

// stubProcedureRepoForPagination records the pagination filter the service
// passed down so tests can assert how the service sanitized the input.
type stubProcedureRepoForPagination struct {
	receivedFilter pagination.Filter
}

func (r *stubProcedureRepoForPagination) Create(_ *model.Procedure) error { return nil }
func (r *stubProcedureRepoForPagination) GetByID(_, _ uuid.UUID) (*model.Procedure, error) {
	return nil, nil
}
func (r *stubProcedureRepoForPagination) GetByIDs(_ []uuid.UUID, _ uuid.UUID) ([]*model.Procedure, error) {
	return nil, nil
}
func (r *stubProcedureRepoForPagination) Update(_ *model.Procedure) error            { return nil }
func (r *stubProcedureRepoForPagination) Delete(_, _ uuid.UUID) error                { return nil }
func (r *stubProcedureRepoForPagination) BulkDelete(_ []uuid.UUID, _ uuid.UUID) error { return nil }
func (r *stubProcedureRepoForPagination) Restore(_, _ uuid.UUID) error                { return nil }
func (r *stubProcedureRepoForPagination) List(_ context.Context, _ uuid.UUID, filter pagination.Filter) ([]*model.Procedure, int64, error) {
	r.receivedFilter = filter
	return nil, 0, nil
}
func (r *stubProcedureRepoForPagination) ListByClinicAndSpecialty(_ context.Context, _ uuid.UUID, _ *uuid.UUID) ([]*model.Procedure, error) {
	return nil, nil
}

// Real-world bug: AI agent's adapter passed Limit=200 expecting it to be
// honored or capped at 100; instead the old sanitizer reset to 10, silently
// dropping items past the 10th in name-ASC order — caused "RPG — Avaliação"
// (the 11th procedure in a 11-active clinic) to vanish from the AI's view.
func TestList_LimitOver100_CapsAt100_NotResetsTo10(t *testing.T) {
	repo := &stubProcedureRepoForPagination{}
	svc := NewProcedureService(repo)

	_, err := svc.List(context.Background(), uuid.New(), pagination.Filter{Page: 1, Limit: 200})
	require.NoError(t, err)

	assert.Equal(t, 100, repo.receivedFilter.Limit,
		"Limit > 100 must cap at 100 so callers asking for large pages get the configured maximum, not silently shrunk to 10")
}

func TestList_LimitZero_FallsBackToDefault10(t *testing.T) {
	repo := &stubProcedureRepoForPagination{}
	svc := NewProcedureService(repo)

	_, err := svc.List(context.Background(), uuid.New(), pagination.Filter{Page: 1, Limit: 0})
	require.NoError(t, err)

	assert.Equal(t, 10, repo.receivedFilter.Limit, "Limit < 1 must fall back to 10 (default page size)")
}

func TestList_LimitWithinRange_PassesThrough(t *testing.T) {
	for _, limit := range []int{1, 25, 50, 100} {
		repo := &stubProcedureRepoForPagination{}
		svc := NewProcedureService(repo)
		_, _ = svc.List(context.Background(), uuid.New(), pagination.Filter{Page: 1, Limit: limit})
		assert.Equal(t, limit, repo.receivedFilter.Limit, "Limit %d must pass through unchanged", limit)
	}
}

func TestList_PageZero_DefaultsTo1(t *testing.T) {
	repo := &stubProcedureRepoForPagination{}
	svc := NewProcedureService(repo)
	_, _ = svc.List(context.Background(), uuid.New(), pagination.Filter{Page: 0, Limit: 50})
	assert.Equal(t, 1, repo.receivedFilter.Page)
}
