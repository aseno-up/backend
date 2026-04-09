package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/patientrecord/model"
)

// Mock repository
type mockPatientRecordRepo struct {
	mock.Mock
}

func (m *mockPatientRecordRepo) CreateRecord(ctx context.Context, record *model.PatientRecord) error {
	return m.Called(ctx, record).Error(0)
}

func (m *mockPatientRecordRepo) GetRecordByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRecord, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PatientRecord), args.Error(1)
}

func (m *mockPatientRecordRepo) GetLatestRecordByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientRecord, error) {
	args := m.Called(ctx, patientID, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PatientRecord), args.Error(1)
}

func (m *mockPatientRecordRepo) GetRecordHistoryByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) ([]*model.PatientRecord, error) {
	args := m.Called(ctx, patientID, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.PatientRecord), args.Error(1)
}

func (m *mockPatientRecordRepo) DeleteRecord(ctx context.Context, id, clinicID uuid.UUID) error {
	return m.Called(ctx, id, clinicID).Error(0)
}

func (m *mockPatientRecordRepo) CreateTemplate(ctx context.Context, template *model.AnamneseTemplate) error {
	return m.Called(ctx, template).Error(0)
}

func (m *mockPatientRecordRepo) GetTemplateByID(ctx context.Context, id, clinicID uuid.UUID) (*model.AnamneseTemplate, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AnamneseTemplate), args.Error(1)
}

func (m *mockPatientRecordRepo) ListTemplates(ctx context.Context, clinicID uuid.UUID) ([]*model.AnamneseTemplate, error) {
	args := m.Called(ctx, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.AnamneseTemplate), args.Error(1)
}

func (m *mockPatientRecordRepo) UpdateTemplate(ctx context.Context, template *model.AnamneseTemplate) error {
	return m.Called(ctx, template).Error(0)
}

func (m *mockPatientRecordRepo) DeleteTemplate(ctx context.Context, id, clinicID uuid.UUID) error {
	return m.Called(ctx, id, clinicID).Error(0)
}

func (m *mockPatientRecordRepo) CreateTemplateField(ctx context.Context, field *model.AnamneseTemplateField) error {
	return m.Called(ctx, field).Error(0)
}

func (m *mockPatientRecordRepo) UpdateTemplateField(ctx context.Context, field *model.AnamneseTemplateField) error {
	return m.Called(ctx, field).Error(0)
}

func (m *mockPatientRecordRepo) DeleteTemplateField(ctx context.Context, id, templateID uuid.UUID) error {
	return m.Called(ctx, id, templateID).Error(0)
}

func (m *mockPatientRecordRepo) ReorderTemplateFields(ctx context.Context, templateID uuid.UUID, fieldIDs []uuid.UUID) error {
	return m.Called(ctx, templateID, fieldIDs).Error(0)
}

func (m *mockPatientRecordRepo) SetTemplateSpecialties(ctx context.Context, templateID uuid.UUID, specialtyIDs []uuid.UUID) error {
	return m.Called(ctx, templateID, specialtyIDs).Error(0)
}

func (m *mockPatientRecordRepo) GetTemplateBySpecialty(ctx context.Context, clinicID, specialtyID uuid.UUID) (*model.AnamneseTemplate, error) {
	args := m.Called(ctx, clinicID, specialtyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AnamneseTemplate), args.Error(1)
}

func (m *mockPatientRecordRepo) GetDefaultTemplate(ctx context.Context, clinicID uuid.UUID) (*model.AnamneseTemplate, error) {
	args := m.Called(ctx, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AnamneseTemplate), args.Error(1)
}

func (m *mockPatientRecordRepo) CreateNote(ctx context.Context, note *model.SessionNote) error {
	return m.Called(ctx, note).Error(0)
}

func (m *mockPatientRecordRepo) GetNoteByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SessionNote, error) {
	args := m.Called(ctx, id, clinicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.SessionNote), args.Error(1)
}

func (m *mockPatientRecordRepo) GetNotesByPatientID(ctx context.Context, patientID, clinicID uuid.UUID, limit, offset int) ([]*model.SessionNote, int64, error) {
	args := m.Called(ctx, patientID, clinicID, limit, offset)
	return args.Get(0).([]*model.SessionNote), int64(args.Int(1)), args.Error(2)
}

func (m *mockPatientRecordRepo) GetNotesByProfessionalID(ctx context.Context, professionalID, clinicID uuid.UUID, limit, offset int) ([]*model.SessionNote, int64, error) {
	args := m.Called(ctx, professionalID, clinicID, limit, offset)
	return args.Get(0).([]*model.SessionNote), int64(args.Int(1)), args.Error(2)
}

func (m *mockPatientRecordRepo) UpdateNote(ctx context.Context, note *model.SessionNote) error {
	return m.Called(ctx, note).Error(0)
}

func (m *mockPatientRecordRepo) DeleteNote(ctx context.Context, id, clinicID uuid.UUID) error {
	return m.Called(ctx, id, clinicID).Error(0)
}

func TestPatientRecordService_CreateRecord(t *testing.T) {
	repo := new(mockPatientRecordRepo)
	svc := NewPatientRecordService(repo, nil)
	ctx := context.Background()

	clinicID := uuid.New()
	patientID := uuid.New()
	professionalID := uuid.New()
	templateID := uuid.New()

	req := &model.CreatePatientRecordRequest{
		TemplateID:   &templateID,
		CustomFields: model.JSON{"pain_level": 5.0, "description": "Feeling better"},
	}

	repo.On("CreateRecord", ctx, mock.MatchedBy(func(r *model.PatientRecord) bool {
		return r.ClinicID == clinicID &&
			r.PatientID == patientID &&
			*r.TemplateID == templateID &&
			r.CustomFields["pain_level"] == 5.0 &&
			r.RecordedBy == professionalID
	})).Return(nil)

	result, err := svc.CreateRecord(ctx, clinicID, patientID, professionalID, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, templateID, *result.TemplateID)
	assert.Equal(t, float64(5), result.CustomFields["pain_level"]) // JSON numbers are float64
	repo.AssertExpectations(t)
}

func TestPatientRecordService_GetRecordByPatientID(t *testing.T) {
	repo := new(mockPatientRecordRepo)
	svc := NewPatientRecordService(repo, nil)
	ctx := context.Background()

	clinicID := uuid.New()
	patientID := uuid.New()
	templateID := uuid.New()

	record := &model.PatientRecord{
		ID:           uuid.New(),
		ClinicID:     clinicID,
		PatientID:    patientID,
		TemplateID:   &templateID,
		CustomFields: model.JSON{"eval": "good"},
		RecordedAt:   time.Now(),
	}

	template := &model.AnamneseTemplate{
		ID:       templateID,
		ClinicID: clinicID,
		Name:     "Default Template",
	}

	repo.On("GetLatestRecordByPatientID", ctx, patientID, clinicID).Return(record, nil)
	repo.On("GetTemplateByID", ctx, templateID, clinicID).Return(template, nil)

	result, err := svc.GetRecordByPatientID(ctx, patientID, clinicID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, templateID, *result.TemplateID)
	assert.NotNil(t, result.Template)
	assert.Equal(t, "Default Template", result.Template.Name)
	repo.AssertExpectations(t)
}

func TestPatientRecordService_SeedDefaultTemplates(t *testing.T) {
	repo := new(mockPatientRecordRepo)
	svc := NewPatientRecordService(repo, nil)
	ctx := context.Background()
	clinicID := uuid.New()

	// All 4 templates should be checked (not found) then created
	repo.On("GetTemplateByID", ctx, mock.AnythingOfType("uuid.UUID"), clinicID).Return(nil, errors.ErrPatientRecordNotFoundInstance)
	repo.On("CreateTemplate", ctx, mock.MatchedBy(func(t *model.AnamneseTemplate) bool {
		return t.ClinicID == clinicID
	})).Return(nil)
	repo.On("CreateTemplateField", ctx, mock.AnythingOfType("*model.AnamneseTemplateField")).Return(nil)

	err := svc.SeedDefaultTemplates(ctx, clinicID)

	assert.NoError(t, err)

	// 4 templates: Geral, Estética, Fisioterapia, Psicologia
	createTemplateCalls := 0
	createFieldCalls := 0
	for _, call := range repo.Calls {
		if call.Method == "CreateTemplate" {
			createTemplateCalls++
		}
		if call.Method == "CreateTemplateField" {
			createFieldCalls++
		}
	}
	assert.Equal(t, 4, createTemplateCalls, "should create 4 templates")
	// Geral=10, Estética=8, Fisioterapia=8, Psicologia=6 = 32 fields
	assert.Equal(t, 32, createFieldCalls, "should create 32 fields total")
	repo.AssertExpectations(t)
}

func TestPatientRecordService_SeedDefaultTemplates_Idempotent(t *testing.T) {
	repo := new(mockPatientRecordRepo)
	svc := NewPatientRecordService(repo, nil)
	ctx := context.Background()
	clinicID := uuid.New()

	// All templates already exist
	repo.On("GetTemplateByID", ctx, mock.AnythingOfType("uuid.UUID"), clinicID).Return(&model.AnamneseTemplate{}, nil)

	err := svc.SeedDefaultTemplates(ctx, clinicID)

	assert.NoError(t, err)

	// No templates should be created
	for _, call := range repo.Calls {
		assert.NotEqual(t, "CreateTemplate", call.Method, "should not create templates when they already exist")
		assert.NotEqual(t, "CreateTemplateField", call.Method, "should not create fields when templates already exist")
	}
}

func TestPatientRecordService_SeedDefaultTemplates_DeterministicIDs(t *testing.T) {
	clinicID := uuid.New()

	// Same clinicID should produce same template IDs
	id1 := deterministicUUID(clinicID, "Geral")
	id2 := deterministicUUID(clinicID, "Geral")
	assert.Equal(t, id1, id2, "deterministic UUIDs should be stable")

	// Different names should produce different IDs
	id3 := deterministicUUID(clinicID, "Estética")
	assert.NotEqual(t, id1, id3, "different names should produce different UUIDs")

	// Different clinics should produce different IDs
	otherClinic := uuid.New()
	id4 := deterministicUUID(otherClinic, "Geral")
	assert.NotEqual(t, id1, id4, "different clinics should produce different UUIDs")
}

func TestPatientRecordService_GetFullRecord_NotFound_ResolvesTemplate(t *testing.T) {
	repo := new(mockPatientRecordRepo)
	svc := NewPatientRecordService(repo, nil)
	ctx := context.Background()

	clinicID := uuid.New()
	patientID := uuid.New()

	template := &model.AnamneseTemplate{
		ID:        uuid.New(),
		ClinicID:  clinicID,
		Name:      "Global Default",
		IsDefault: true,
	}

	repo.On("GetLatestRecordByPatientID", ctx, patientID, clinicID).Return(nil, errors.ErrPatientRecordNotFoundInstance)
	repo.On("GetDefaultTemplate", ctx, clinicID).Return(template, nil)
	repo.On("GetNotesByPatientID", ctx, patientID, clinicID, 50, 0).Return([]*model.SessionNote{}, 0, nil)

	result, err := svc.GetFullRecord(ctx, patientID, clinicID, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Record)
	assert.Equal(t, template.ID, *result.Record.TemplateID)
	assert.Equal(t, "Global Default", result.Record.Template.Name)
	repo.AssertExpectations(t)
}
