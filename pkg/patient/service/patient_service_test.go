package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/pagination"
	"github.com/aseno/backend/pkg/patient/model"
)

// ---------------------------------------------------------------------------
// Mock repository that captures created patients
// ---------------------------------------------------------------------------

type capturingPatientRepo struct {
	created *model.Patient
}

func (r *capturingPatientRepo) Create(_ context.Context, p *model.Patient) error {
	r.created = p
	p.ID = uuid.New()
	return nil
}

func (r *capturingPatientRepo) GetByID(_ context.Context, _, _ uuid.UUID) (*model.Patient, error) {
	return nil, nil
}
func (r *capturingPatientRepo) List(_ context.Context, _ uuid.UUID, _ pagination.Filter) ([]*model.Patient, int64, error) {
	return nil, 0, nil
}
func (r *capturingPatientRepo) Update(_ context.Context, _ *model.Patient) error { return nil }
func (r *capturingPatientRepo) Delete(_ context.Context, _, _ uuid.UUID) error   { return nil }
func (r *capturingPatientRepo) BulkDelete(_ context.Context, _ []uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (r *capturingPatientRepo) GetByPhone(_ context.Context, _ string, _ uuid.UUID) (*model.Patient, error) {
	return nil, nil
}
func (r *capturingPatientRepo) GetByDocument(_ context.Context, _ string, _ uuid.UUID) (*model.Patient, error) {
	return nil, nil
}
func (r *capturingPatientRepo) GetAppointmentStats(_ context.Context, _, _ uuid.UUID) (*model.PatientAppointmentStats, error) {
	return nil, nil
}
func (r *capturingPatientRepo) UpdateRiskScore(_ context.Context, _, _ uuid.UUID, _ model.NoShowRisk, _ *model.PatientAppointmentStats) error {
	return nil
}
func (r *capturingPatientRepo) GetAllByClinicID(_ context.Context, _ uuid.UUID) ([]*model.Patient, error) {
	return nil, nil
}
func (r *capturingPatientRepo) ListBySource(_ context.Context, _ uuid.UUID, _ model.PatientSource, _ pagination.Filter) ([]*model.Patient, int64, error) {
	return nil, 0, nil
}
func (r *capturingPatientRepo) MergePatients(_ context.Context, _, _, _ uuid.UUID) error { return nil }
func (r *capturingPatientRepo) GetUnlinkedProvisionalPatients(_ context.Context, _ uuid.UUID) ([]*model.Patient, error) {
	return nil, nil
}
func (r *capturingPatientRepo) SearchPatientsExcludingProvisional(_ context.Context, _ uuid.UUID, _ string, _ int) ([]*model.Patient, error) {
	return nil, nil
}
func (r *capturingPatientRepo) MergePatientsBulk(_ context.Context, _, _ uuid.UUID, _ []uuid.UUID, _ uuid.UUID) (int64, error) {
	return 0, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCreate_DefaultSource_IsInternal(t *testing.T) {
	repo := &capturingPatientRepo{}
	svc := NewPatientService(repo, nil)

	req := &model.CreatePatientRequest{
		Name:  "Test Patient",
		Phone: "+5511999990001",
	}

	_, err := svc.Create(context.Background(), uuid.New(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.created.Source != model.PatientSourceInternal {
		t.Errorf("expected Source %q when not specified, got %q", model.PatientSourceInternal, repo.created.Source)
	}
}

func TestCreate_WhatsAppSource_IsPreserved(t *testing.T) {
	repo := &capturingPatientRepo{}
	svc := NewPatientService(repo, nil)

	req := &model.CreatePatientRequest{
		Name:   "WhatsApp Contact",
		Phone:  "+5511999990002",
		Source: model.PatientSourceWhatsApp,
	}

	_, err := svc.Create(context.Background(), uuid.New(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.created.Source != model.PatientSourceWhatsApp {
		t.Errorf("expected Source %q, got %q", model.PatientSourceWhatsApp, repo.created.Source)
	}
}

func TestCreate_SelfRegisteredSource_IsPreserved(t *testing.T) {
	repo := &capturingPatientRepo{}
	svc := NewPatientService(repo, nil)

	req := &model.CreatePatientRequest{
		Name:   "Portal Patient",
		Phone:  "+5511999990003",
		Source: model.PatientSourceSelfRegistered,
	}

	_, err := svc.Create(context.Background(), uuid.New(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.created.Source != model.PatientSourceSelfRegistered {
		t.Errorf("expected Source %q, got %q", model.PatientSourceSelfRegistered, repo.created.Source)
	}
}
