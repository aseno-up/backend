package service

import (
	"context"
	"math"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/patientrecord/model"
	"github.com/aseno/backend/pkg/patientrecord/repository"
)

// PatientRecordService defines the interface for patient record business logic
type PatientRecordService interface {
	// PatientRecord operations
	CreateRecord(ctx context.Context, clinicID, patientID uuid.UUID, req *model.CreatePatientRecordRequest) (*model.PatientRecordResponse, error)
	GetRecordByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRecordResponse, error)
	GetRecordByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientRecordResponse, error)
	GetOrCreateRecord(ctx context.Context, clinicID, patientID uuid.UUID) (*model.PatientRecordResponse, error)
	UpdateRecord(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdatePatientRecordRequest) (*model.PatientRecordResponse, error)
	DeleteRecord(ctx context.Context, id, clinicID uuid.UUID) error

	// SessionNote operations
	CreateNote(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, req *model.CreateSessionNoteRequest) (*model.SessionNoteResponse, error)
	GetNoteByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SessionNoteResponse, error)
	ListNotesByPatient(ctx context.Context, patientID, clinicID uuid.UUID, page, perPage int) (*model.SessionNoteListResponse, error)
	ListNotesByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID, page, perPage int) (*model.SessionNoteListResponse, error)
	UpdateNote(ctx context.Context, id, clinicID, professionalID uuid.UUID, req *model.UpdateSessionNoteRequest) (*model.SessionNoteResponse, error)
	DeleteNote(ctx context.Context, id, clinicID, professionalID uuid.UUID) error

	// Full record with notes
	GetFullRecord(ctx context.Context, patientID, clinicID uuid.UUID, professionalID *uuid.UUID) (*model.FullPatientRecordResponse, error)
}

// patientRecordService implements PatientRecordService
type patientRecordService struct {
	repo repository.PatientRecordRepository
}

// NewPatientRecordService creates a new patient record service
func NewPatientRecordService(repo repository.PatientRecordRepository) PatientRecordService {
	return &patientRecordService{repo: repo}
}

// CreateRecord creates a new patient record
func (s *patientRecordService) CreateRecord(ctx context.Context, clinicID, patientID uuid.UUID, req *model.CreatePatientRecordRequest) (*model.PatientRecordResponse, error) {
	record := &model.PatientRecord{
		ClinicID:         clinicID,
		PatientID:        patientID,
		ChiefComplaint:   req.ChiefComplaint,
		MedicalHistory:   req.MedicalHistory,
		FamilyHistory:    req.FamilyHistory,
		Allergies:        req.Allergies,
		Medications:      req.Medications,
		SurgicalHistory:  req.SurgicalHistory,
		SocialHistory:    req.SocialHistory,
		InitialDiagnosis: req.InitialDiagnosis,
		TreatmentPlan:    req.TreatmentPlan,
		Observations:     req.Observations,
	}

	if err := s.repo.CreateRecord(ctx, record); err != nil {
		return nil, err
	}

	return record.ToResponse(), nil
}

// GetRecordByID retrieves a patient record by ID
func (s *patientRecordService) GetRecordByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRecordResponse, error) {
	record, err := s.repo.GetRecordByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}
	return record.ToResponse(), nil
}

// GetRecordByPatientID retrieves a patient record by patient ID
func (s *patientRecordService) GetRecordByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientRecordResponse, error) {
	record, err := s.repo.GetRecordByPatientID(ctx, patientID, clinicID)
	if err != nil {
		return nil, err
	}
	return record.ToResponse(), nil
}

// GetOrCreateRecord retrieves or creates a patient record
func (s *patientRecordService) GetOrCreateRecord(ctx context.Context, clinicID, patientID uuid.UUID) (*model.PatientRecordResponse, error) {
	record, err := s.repo.GetRecordByPatientID(ctx, patientID, clinicID)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create a new empty record
			newRecord := &model.PatientRecord{
				ClinicID:  clinicID,
				PatientID: patientID,
			}
			if err := s.repo.CreateRecord(ctx, newRecord); err != nil {
				return nil, err
			}
			return newRecord.ToResponse(), nil
		}
		return nil, err
	}
	return record.ToResponse(), nil
}

// UpdateRecord updates a patient record
func (s *patientRecordService) UpdateRecord(ctx context.Context, id, clinicID uuid.UUID, req *model.UpdatePatientRecordRequest) (*model.PatientRecordResponse, error) {
	record, err := s.repo.GetRecordByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.ChiefComplaint != nil {
		record.ChiefComplaint = *req.ChiefComplaint
	}
	if req.MedicalHistory != nil {
		record.MedicalHistory = *req.MedicalHistory
	}
	if req.FamilyHistory != nil {
		record.FamilyHistory = *req.FamilyHistory
	}
	if req.Allergies != nil {
		record.Allergies = *req.Allergies
	}
	if req.Medications != nil {
		record.Medications = *req.Medications
	}
	if req.SurgicalHistory != nil {
		record.SurgicalHistory = *req.SurgicalHistory
	}
	if req.SocialHistory != nil {
		record.SocialHistory = *req.SocialHistory
	}
	if req.InitialDiagnosis != nil {
		record.InitialDiagnosis = *req.InitialDiagnosis
	}
	if req.TreatmentPlan != nil {
		record.TreatmentPlan = *req.TreatmentPlan
	}
	if req.Observations != nil {
		record.Observations = *req.Observations
	}

	if err := s.repo.UpdateRecord(ctx, record); err != nil {
		return nil, err
	}

	return record.ToResponse(), nil
}

// DeleteRecord deletes a patient record
func (s *patientRecordService) DeleteRecord(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.DeleteRecord(ctx, id, clinicID)
}

// CreateNote creates a new session note
func (s *patientRecordService) CreateNote(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, req *model.CreateSessionNoteRequest) (*model.SessionNoteResponse, error) {
	note := &model.SessionNote{
		ClinicID:       clinicID,
		PatientID:      patientID,
		ProfessionalID: professionalID,
		AppointmentID:  req.AppointmentID,
		SessionDate:    req.SessionDate,
		SessionType:    req.SessionType,
		Subjective:     req.Subjective,
		Objective:      req.Objective,
		Assessment:     req.Assessment,
		Plan:           req.Plan,
		Notes:          req.Notes,
		IsPrivate:      req.IsPrivate,
	}

	if err := s.repo.CreateNote(ctx, note); err != nil {
		return nil, err
	}

	return note.ToResponse(), nil
}

// GetNoteByID retrieves a session note by ID
func (s *patientRecordService) GetNoteByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SessionNoteResponse, error) {
	note, err := s.repo.GetNoteByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}
	return note.ToResponse(), nil
}

// ListNotesByPatient retrieves session notes for a patient
func (s *patientRecordService) ListNotesByPatient(ctx context.Context, patientID, clinicID uuid.UUID, page, perPage int) (*model.SessionNoteListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	notes, total, err := s.repo.GetNotesByPatientID(ctx, patientID, clinicID, perPage, offset)
	if err != nil {
		return nil, err
	}

	data := make([]*model.SessionNoteResponse, len(notes))
	for i, n := range notes {
		data[i] = n.ToResponse()
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	return &model.SessionNoteListResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

// ListNotesByProfessional retrieves session notes for a professional
func (s *patientRecordService) ListNotesByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID, page, perPage int) (*model.SessionNoteListResponse, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	notes, total, err := s.repo.GetNotesByProfessionalID(ctx, professionalID, clinicID, perPage, offset)
	if err != nil {
		return nil, err
	}

	data := make([]*model.SessionNoteResponse, len(notes))
	for i, n := range notes {
		data[i] = n.ToResponse()
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	return &model.SessionNoteListResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

// UpdateNote updates a session note
func (s *patientRecordService) UpdateNote(ctx context.Context, id, clinicID, professionalID uuid.UUID, req *model.UpdateSessionNoteRequest) (*model.SessionNoteResponse, error) {
	note, err := s.repo.GetNoteByID(ctx, id, clinicID)
	if err != nil {
		return nil, err
	}

	// Only the professional who created the note can update it
	if note.ProfessionalID != professionalID {
		return nil, errors.ErrForbiddenInstance.WithMessage("You can only update your own session notes")
	}

	// Update fields if provided
	if req.SessionDate != nil {
		note.SessionDate = *req.SessionDate
	}
	if req.SessionType != nil {
		note.SessionType = *req.SessionType
	}
	if req.Subjective != nil {
		note.Subjective = *req.Subjective
	}
	if req.Objective != nil {
		note.Objective = *req.Objective
	}
	if req.Assessment != nil {
		note.Assessment = *req.Assessment
	}
	if req.Plan != nil {
		note.Plan = *req.Plan
	}
	if req.Notes != nil {
		note.Notes = *req.Notes
	}
	if req.IsPrivate != nil {
		note.IsPrivate = *req.IsPrivate
	}

	if err := s.repo.UpdateNote(ctx, note); err != nil {
		return nil, err
	}

	return note.ToResponse(), nil
}

// DeleteNote deletes a session note
func (s *patientRecordService) DeleteNote(ctx context.Context, id, clinicID, professionalID uuid.UUID) error {
	note, err := s.repo.GetNoteByID(ctx, id, clinicID)
	if err != nil {
		return err
	}

	// Only the professional who created the note can delete it
	if note.ProfessionalID != professionalID {
		return errors.ErrForbiddenInstance.WithMessage("You can only delete your own session notes")
	}

	return s.repo.DeleteNote(ctx, id, clinicID)
}

// GetFullRecord retrieves the full patient record with session notes
func (s *patientRecordService) GetFullRecord(ctx context.Context, patientID, clinicID uuid.UUID, professionalID *uuid.UUID) (*model.FullPatientRecordResponse, error) {
	// Get or create the patient record
	record, err := s.repo.GetRecordByPatientID(ctx, patientID, clinicID)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create empty record
			record = &model.PatientRecord{
				ClinicID:  clinicID,
				PatientID: patientID,
			}
			if err := s.repo.CreateRecord(ctx, record); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Get session notes (limit to 50 most recent)
	notes, _, err := s.repo.GetNotesByPatientID(ctx, patientID, clinicID, 50, 0)
	if err != nil {
		return nil, err
	}

	// Filter private notes if professionalID is provided
	noteResponses := make([]*model.SessionNoteResponse, 0, len(notes))
	for _, n := range notes {
		// Show note if not private, or if current user is the creator
		if !n.IsPrivate || (professionalID != nil && n.ProfessionalID == *professionalID) {
			noteResponses = append(noteResponses, n.ToResponse())
		}
	}

	return &model.FullPatientRecordResponse{
		Record:       record.ToResponse(),
		SessionNotes: noteResponses,
	}, nil
}
