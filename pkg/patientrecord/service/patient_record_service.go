package service

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/errors"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	"github.com/aseno/backend/pkg/patientrecord/model"
	"github.com/aseno/backend/pkg/patientrecord/repository"
)

// PatientRecordService defines the interface for patient record business logic
type PatientRecordService interface {
	// PatientRecord operations
	CreateRecord(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, req *model.CreatePatientRecordRequest) (*model.PatientRecordResponse, error)
	GetRecordByID(ctx context.Context, id, clinicID uuid.UUID) (*model.PatientRecordResponse, error)
	GetRecordByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientRecordResponse, error)
	GetRecordHistory(ctx context.Context, patientID, clinicID uuid.UUID) ([]*model.PatientRecordResponse, error)
	DeleteRecord(ctx context.Context, id, clinicID uuid.UUID) error

	// Template operations
	CreateTemplate(ctx context.Context, clinicID uuid.UUID, template *model.AnamneseTemplate) (*model.AnamneseTemplate, error)
	GetTemplate(ctx context.Context, id, clinicID uuid.UUID) (*model.AnamneseTemplate, error)
	ListTemplates(ctx context.Context, clinicID uuid.UUID) ([]*model.AnamneseTemplate, error)
	UpdateTemplate(ctx context.Context, id, clinicID uuid.UUID, template *model.AnamneseTemplate) (*model.AnamneseTemplate, error)
	DeleteTemplate(ctx context.Context, id, clinicID uuid.UUID) error
	ResolveTemplate(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, appointmentID *uuid.UUID) (*model.AnamneseTemplate, error)

	// Template Field operations
	CreateField(ctx context.Context, templateID, clinicID uuid.UUID, field *model.AnamneseTemplateField) (*model.AnamneseTemplateField, error)
	UpdateField(ctx context.Context, id, templateID, clinicID uuid.UUID, field *model.AnamneseTemplateField) (*model.AnamneseTemplateField, error)
	DeleteField(ctx context.Context, id, templateID, clinicID uuid.UUID) error
	ReorderFields(ctx context.Context, templateID, clinicID uuid.UUID, fieldIDs []uuid.UUID) error

	// Specialty Mapping
	SetTemplateSpecialties(ctx context.Context, templateID, clinicID uuid.UUID, specialtyIDs []uuid.UUID) error

	// SessionNote operations
	CreateNote(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, req *model.CreateSessionNoteRequest) (*model.SessionNoteResponse, error)
	GetNoteByID(ctx context.Context, id, clinicID uuid.UUID) (*model.SessionNoteResponse, error)
	ListNotesByPatient(ctx context.Context, patientID, clinicID uuid.UUID, page, perPage int) (*model.SessionNoteListResponse, error)
	ListNotesByProfessional(ctx context.Context, professionalID, clinicID uuid.UUID, page, perPage int) (*model.SessionNoteListResponse, error)
	UpdateNote(ctx context.Context, id, clinicID, professionalID uuid.UUID, req *model.UpdateSessionNoteRequest) (*model.SessionNoteResponse, error)
	DeleteNote(ctx context.Context, id, clinicID, professionalID uuid.UUID) error

	// Full record with notes
	GetFullRecord(ctx context.Context, patientID, clinicID uuid.UUID, professionalID *uuid.UUID, appointmentID *uuid.UUID) (*model.FullPatientRecordResponse, error)

	// Seed default templates for a new clinic
	SeedDefaultTemplates(ctx context.Context, clinicID uuid.UUID) error
}

// AppointmentInfoProvider provides appointment information to avoid circular dependency
type AppointmentInfoProvider interface {
	GetByID(ctx context.Context, id, clinicID uuid.UUID) (*appointment_model.Appointment, error)
}

// patientRecordService implements PatientRecordService
type patientRecordService struct {
	repo            repository.PatientRecordRepository
	appointmentRepo AppointmentInfoProvider
}

// NewPatientRecordService creates a new patient record service
func NewPatientRecordService(repo repository.PatientRecordRepository, appointmentRepo AppointmentInfoProvider) PatientRecordService {
	return &patientRecordService{
		repo:            repo,
		appointmentRepo: appointmentRepo,
	}
}

// CreateRecord creates a new patient record (append-only)
func (s *patientRecordService) CreateRecord(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, req *model.CreatePatientRecordRequest) (*model.PatientRecordResponse, error) {
	record := &model.PatientRecord{
		ClinicID:     clinicID,
		PatientID:    patientID,
		TemplateID:   req.TemplateID,
		CustomFields: req.CustomFields,
		RecordedAt:   time.Now(),
		RecordedBy:   professionalID,
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

// GetRecordByPatientID retrieves the latest patient record by patient ID
func (s *patientRecordService) GetRecordByPatientID(ctx context.Context, patientID, clinicID uuid.UUID) (*model.PatientRecordResponse, error) {
	record, err := s.repo.GetLatestRecordByPatientID(ctx, patientID, clinicID)
	if err != nil {
		return nil, err
	}
	resp := record.ToResponse()

	// Load template if exists
	if record.TemplateID != nil {
		template, err := s.repo.GetTemplateByID(ctx, *record.TemplateID, clinicID)
		if err == nil {
			resp.Template = template
		}
	}

	return resp, nil
}

// GetRecordHistory retrieves all versions of a patient record
func (s *patientRecordService) GetRecordHistory(ctx context.Context, patientID, clinicID uuid.UUID) ([]*model.PatientRecordResponse, error) {
	records, err := s.repo.GetRecordHistoryByPatientID(ctx, patientID, clinicID)
	if err != nil {
		return nil, err
	}

	resps := make([]*model.PatientRecordResponse, len(records))
	for i, r := range records {
		resps[i] = r.ToResponse()
	}
	return resps, nil
}

// DeleteRecord deletes a patient record version
func (s *patientRecordService) DeleteRecord(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.DeleteRecord(ctx, id, clinicID)
}

// CreateTemplate creates a new anamnese template
func (s *patientRecordService) CreateTemplate(ctx context.Context, clinicID uuid.UUID, template *model.AnamneseTemplate) (*model.AnamneseTemplate, error) {
	template.ClinicID = clinicID
	if err := s.repo.CreateTemplate(ctx, template); err != nil {
		return nil, err
	}
	return template, nil
}

// GetTemplate retrieves a template by ID
func (s *patientRecordService) GetTemplate(ctx context.Context, id, clinicID uuid.UUID) (*model.AnamneseTemplate, error) {
	return s.repo.GetTemplateByID(ctx, id, clinicID)
}

// ListTemplates lists templates for a clinic
func (s *patientRecordService) ListTemplates(ctx context.Context, clinicID uuid.UUID) ([]*model.AnamneseTemplate, error) {
	return s.repo.ListTemplates(ctx, clinicID)
}

// UpdateTemplate updates template metadata
func (s *patientRecordService) UpdateTemplate(ctx context.Context, id, clinicID uuid.UUID, template *model.AnamneseTemplate) (*model.AnamneseTemplate, error) {
	template.ID = id
	template.ClinicID = clinicID
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return nil, err
	}
	return template, nil
}

// DeleteTemplate deletes a template
func (s *patientRecordService) DeleteTemplate(ctx context.Context, id, clinicID uuid.UUID) error {
	return s.repo.DeleteTemplate(ctx, id, clinicID)
}

// ResolveTemplate finds the best template for the current context
func (s *patientRecordService) ResolveTemplate(ctx context.Context, clinicID uuid.UUID, professionalID *uuid.UUID, appointmentID *uuid.UUID) (*model.AnamneseTemplate, error) {
	// 1. If appointmentID is provided, check its specialties
	if appointmentID != nil && s.appointmentRepo != nil {
		appointment, err := s.appointmentRepo.GetByID(ctx, *appointmentID, clinicID)
		if err == nil && len(appointment.Specialties) > 0 {
			// Try to find a template for each specialty of the appointment
			for _, spec := range appointment.Specialties {
				template, err := s.repo.GetTemplateBySpecialty(ctx, clinicID, spec.SpecialtyID)
				if err == nil && template != nil {
					return template, nil
				}
			}
		}
	}

	// 2. Fallback to clinic's default
	template, err := s.repo.GetDefaultTemplate(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	return template, nil
}

// CreateField adds a field to a template
func (s *patientRecordService) CreateField(ctx context.Context, templateID, clinicID uuid.UUID, field *model.AnamneseTemplateField) (*model.AnamneseTemplateField, error) {
	// Verify template belongs to clinic
	_, err := s.repo.GetTemplateByID(ctx, templateID, clinicID)
	if err != nil {
		return nil, err
	}

	field.TemplateID = templateID
	if err := s.repo.CreateTemplateField(ctx, field); err != nil {
		return nil, err
	}
	return field, nil
}

// UpdateField updates a template field
func (s *patientRecordService) UpdateField(ctx context.Context, id, templateID, clinicID uuid.UUID, field *model.AnamneseTemplateField) (*model.AnamneseTemplateField, error) {
	_, err := s.repo.GetTemplateByID(ctx, templateID, clinicID)
	if err != nil {
		return nil, err
	}

	field.ID = id
	field.TemplateID = templateID
	if err := s.repo.UpdateTemplateField(ctx, field); err != nil {
		return nil, err
	}
	return field, nil
}

// DeleteField deletes a template field
func (s *patientRecordService) DeleteField(ctx context.Context, id, templateID, clinicID uuid.UUID) error {
	_, err := s.repo.GetTemplateByID(ctx, templateID, clinicID)
	if err != nil {
		return err
	}
	return s.repo.DeleteTemplateField(ctx, id, templateID)
}

// ReorderFields updates display order
func (s *patientRecordService) ReorderFields(ctx context.Context, templateID, clinicID uuid.UUID, fieldIDs []uuid.UUID) error {
	_, err := s.repo.GetTemplateByID(ctx, templateID, clinicID)
	if err != nil {
		return err
	}
	return s.repo.ReorderTemplateFields(ctx, templateID, fieldIDs)
}

// SetTemplateSpecialties manages specialty mapping
func (s *patientRecordService) SetTemplateSpecialties(ctx context.Context, templateID, clinicID uuid.UUID, specialtyIDs []uuid.UUID) error {
	_, err := s.repo.GetTemplateByID(ctx, templateID, clinicID)
	if err != nil {
		return err
	}
	return s.repo.SetTemplateSpecialties(ctx, templateID, specialtyIDs)
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
func (s *patientRecordService) GetFullRecord(ctx context.Context, patientID, clinicID uuid.UUID, professionalID *uuid.UUID, appointmentID *uuid.UUID) (*model.FullPatientRecordResponse, error) {
	// Get the latest patient record
	record, err := s.repo.GetLatestRecordByPatientID(ctx, patientID, clinicID)
	var recordResp *model.PatientRecordResponse

	if err != nil {
		if errors.IsNotFound(err) {
			// Resolve template for a new record (empty state)
			template, _ := s.ResolveTemplate(ctx, clinicID, professionalID, appointmentID)
			recordResp = &model.PatientRecordResponse{
				ClinicID:  clinicID,
				PatientID: patientID,
				Template:  template,
			}
			if template != nil {
				tid := template.ID
				recordResp.TemplateID = &tid
			}
		} else {
			return nil, err
		}
	} else {
		recordResp = record.ToResponse()
		// Load template if exists
		if record.TemplateID != nil {
			template, err := s.repo.GetTemplateByID(ctx, *record.TemplateID, clinicID)
			if err == nil {
				recordResp.Template = template
			}
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
		Record:       recordResp,
		SessionNotes: noteResponses,
	}, nil
}

// Fixed UUIDs for default templates (deterministic per clinic via namespace)
var (
	defaultTemplateNamespace = uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
)

// deterministicUUID generates a deterministic UUID from clinicID + name
func deterministicUUID(clinicID uuid.UUID, name string) uuid.UUID {
	return uuid.NewSHA1(defaultTemplateNamespace, []byte(clinicID.String()+":"+name))
}

// SeedDefaultTemplates creates the 4 default anamnese templates for a new clinic
func (s *patientRecordService) SeedDefaultTemplates(ctx context.Context, clinicID uuid.UUID) error {
	type fieldDef struct {
		Label     string
		FieldKey  string
		FieldType string
		Options   []string
		Required  bool
	}

	type templateDef struct {
		Name      string
		IsDefault bool
		Fields    []fieldDef
	}

	templates := []templateDef{
		{
			Name:      "Geral",
			IsDefault: true,
			Fields: []fieldDef{
				{Label: "Queixa Principal", FieldKey: "queixa_principal", FieldType: "textarea", Required: true},
				{Label: "Histórico Médico", FieldKey: "historico_medico", FieldType: "textarea"},
				{Label: "Histórico Cirúrgico", FieldKey: "historico_cirurgico", FieldType: "textarea"},
				{Label: "Alergias", FieldKey: "alergias", FieldType: "textarea"},
				{Label: "Medicamentos em Uso", FieldKey: "medicamentos_em_uso", FieldType: "textarea"},
				{Label: "Histórico Familiar", FieldKey: "historico_familiar", FieldType: "textarea"},
				{Label: "Hábitos de Vida", FieldKey: "habitos_de_vida", FieldType: "textarea"},
				{Label: "Histórico Social", FieldKey: "historico_social", FieldType: "textarea"},
				{Label: "Exame Físico", FieldKey: "exame_fisico", FieldType: "textarea"},
				{Label: "Observações", FieldKey: "observacoes", FieldType: "textarea"},
			},
		},
		{
			Name: "Estética",
			Fields: []fieldDef{
				{Label: "Queixa Principal", FieldKey: "queixa_principal", FieldType: "textarea", Required: true},
				{Label: "Alergias", FieldKey: "alergias", FieldType: "textarea"},
				{Label: "Medicamentos em Uso", FieldKey: "medicamentos_em_uso", FieldType: "textarea"},
				{Label: "Fototipo", FieldKey: "fototipo", FieldType: "select", Options: []string{"I", "II", "III", "IV", "V", "VI"}},
				{Label: "Região a Tratar", FieldKey: "regiao_a_tratar", FieldType: "multiselect", Options: []string{"Face", "Abdômen", "Flancos", "Coxas", "Braços", "Glúteos", "Costas", "Pescoço"}},
				{Label: "Procedimentos Anteriores", FieldKey: "procedimentos_anteriores", FieldType: "textarea"},
				{Label: "Contraindicações", FieldKey: "contraindicacoes", FieldType: "textarea"},
				{Label: "Observações", FieldKey: "observacoes", FieldType: "textarea"},
			},
		},
		{
			Name: "Fisioterapia",
			Fields: []fieldDef{
				{Label: "Queixa Principal", FieldKey: "queixa_principal", FieldType: "textarea", Required: true},
				{Label: "Histórico Médico", FieldKey: "historico_medico", FieldType: "textarea"},
				{Label: "Histórico Cirúrgico", FieldKey: "historico_cirurgico", FieldType: "textarea"},
				{Label: "Região", FieldKey: "regiao", FieldType: "text"},
				{Label: "Lateralidade", FieldKey: "lateralidade", FieldType: "select", Options: []string{"Direita", "Esquerda", "Bilateral"}},
				{Label: "EVA Dor", FieldKey: "eva_dor", FieldType: "scale"},
				{Label: "Amplitude de Movimento", FieldKey: "amplitude_de_movimento", FieldType: "textarea"},
				{Label: "Plano de Tratamento", FieldKey: "plano_de_tratamento", FieldType: "textarea"},
			},
		},
		{
			Name: "Psicologia",
			Fields: []fieldDef{
				{Label: "Queixa Principal", FieldKey: "queixa_principal", FieldType: "textarea", Required: true},
				{Label: "Histórico Médico", FieldKey: "historico_medico", FieldType: "textarea"},
				{Label: "Histórico Familiar", FieldKey: "historico_familiar", FieldType: "textarea"},
				{Label: "Medicamentos em Uso", FieldKey: "medicamentos_em_uso", FieldType: "textarea"},
				{Label: "Histórico Social", FieldKey: "historico_social", FieldType: "textarea"},
				{Label: "Observações", FieldKey: "observacoes", FieldType: "textarea"},
			},
		},
	}

	for _, tmplDef := range templates {
		templateID := deterministicUUID(clinicID, tmplDef.Name)

		// Check if already exists (idempotent)
		existing, _ := s.repo.GetTemplateByID(ctx, templateID, clinicID)
		if existing != nil {
			continue
		}

		tmpl := &model.AnamneseTemplate{
			ID:        templateID,
			ClinicID:  clinicID,
			Name:      tmplDef.Name,
			IsDefault: tmplDef.IsDefault,
		}

		if err := s.repo.CreateTemplate(ctx, tmpl); err != nil {
			return err
		}

		for i, f := range tmplDef.Fields {
			field := &model.AnamneseTemplateField{
				TemplateID:   templateID,
				Label:        f.Label,
				FieldKey:     f.FieldKey,
				FieldType:    f.FieldType,
				Required:     f.Required,
				DisplayOrder: i,
			}
			if len(f.Options) > 0 {
				field.Options = model.NewRawJSON(f.Options)
			}
			if err := s.repo.CreateTemplateField(ctx, field); err != nil {
				return err
			}
		}
	}

	return nil
}
