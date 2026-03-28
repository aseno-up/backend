package repository

import (
	professional_model "github.com/aseno/backend/pkg/professional/model"
	"github.com/aseno/backend/pkg/specialty/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SpecialtyRepository interface {
	Create(specialty *model.Specialty) error
	GetByID(id, clinicID uuid.UUID) (*model.Specialty, error)
	GetByName(name string, clinicID uuid.UUID) (*model.Specialty, error)
	Update(specialty *model.Specialty) error
	Delete(id, clinicID uuid.UUID) error
	ListByClinic(clinicID uuid.UUID) ([]*model.Specialty, error)
	ListActiveByClinic(clinicID uuid.UUID) ([]*model.Specialty, error)
	CountByClinic(clinicID uuid.UUID) (int64, error)
	// N:N professional-specialty methods
	GetByProfessionalID(professionalID uuid.UUID) ([]*model.Specialty, error)
	GetByProfessionalIDs(professionalIDs []uuid.UUID) (map[uuid.UUID][]*model.Specialty, error)
	SetProfessionalSpecialties(professionalID uuid.UUID, specialtyIDs []uuid.UUID) error
}

type specialtyRepository struct {
	db *gorm.DB
}

func NewSpecialtyRepository(db *gorm.DB) SpecialtyRepository {
	return &specialtyRepository{db: db}
}

func (r *specialtyRepository) Create(specialty *model.Specialty) error {
	return r.db.Create(specialty).Error
}

func (r *specialtyRepository) GetByID(id, clinicID uuid.UUID) (*model.Specialty, error) {
	var specialty model.Specialty
	err := r.db.Where("id = ? AND clinic_id = ?", id, clinicID).First(&specialty).Error
	return &specialty, err
}

func (r *specialtyRepository) GetByName(name string, clinicID uuid.UUID) (*model.Specialty, error) {
	var specialty model.Specialty
	err := r.db.Where("name = ? AND clinic_id = ?", name, clinicID).First(&specialty).Error
	return &specialty, err
}

func (r *specialtyRepository) Update(specialty *model.Specialty) error {
	return r.db.Where("id = ? AND clinic_id = ?", specialty.ID, specialty.ClinicID).Updates(specialty).Error
}

func (r *specialtyRepository) Delete(id, clinicID uuid.UUID) error {
	return r.db.Where("id = ? AND clinic_id = ?", id, clinicID).Delete(&model.Specialty{}).Error
}

func (r *specialtyRepository) ListByClinic(clinicID uuid.UUID) ([]*model.Specialty, error) {
	var specialties []*model.Specialty
	err := r.db.Where("clinic_id = ?", clinicID).Order("name ASC").Find(&specialties).Error
	return specialties, err
}

func (r *specialtyRepository) ListActiveByClinic(clinicID uuid.UUID) ([]*model.Specialty, error) {
	var specialties []*model.Specialty
	err := r.db.Where("clinic_id = ? AND active = true", clinicID).Order("name ASC").Find(&specialties).Error
	return specialties, err
}

func (r *specialtyRepository) CountByClinic(clinicID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.Specialty{}).Where("clinic_id = ?", clinicID).Count(&count).Error
	return count, err
}

// GetByProfessionalID returns specialties for a single professional
func (r *specialtyRepository) GetByProfessionalID(professionalID uuid.UUID) ([]*model.Specialty, error) {
	var specialties []*model.Specialty
	err := r.db.
		Joins("JOIN professional_specialties ps ON ps.specialty_id = specialties.id").
		Where("ps.professional_id = ?", professionalID).
		Find(&specialties).Error
	return specialties, err
}

// GetByProfessionalIDs returns a map of professional ID → specialties (batch)
func (r *specialtyRepository) GetByProfessionalIDs(professionalIDs []uuid.UUID) (map[uuid.UUID][]*model.Specialty, error) {
	if len(professionalIDs) == 0 {
		return map[uuid.UUID][]*model.Specialty{}, nil
	}

	type row struct {
		ProfessionalID uuid.UUID
		model.Specialty
	}
	var rows []row

	err := r.db.
		Table("specialties").
		Select("professional_specialties.professional_id, specialties.*").
		Joins("JOIN professional_specialties ON professional_specialties.specialty_id = specialties.id").
		Where("professional_specialties.professional_id IN ?", professionalIDs).
		Where("specialties.deleted_at IS NULL").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID][]*model.Specialty)
	for i := range rows {
		spec := rows[i].Specialty
		result[rows[i].ProfessionalID] = append(result[rows[i].ProfessionalID], &spec)
	}
	return result, nil
}

// SetProfessionalSpecialties replaces all specialties for a professional
func (r *specialtyRepository) SetProfessionalSpecialties(professionalID uuid.UUID, specialtyIDs []uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing
		if err := tx.Where("professional_id = ?", professionalID).Delete(&professional_model.ProfessionalSpecialty{}).Error; err != nil {
			return err
		}
		// Insert new
		if len(specialtyIDs) == 0 {
			return nil
		}
		records := make([]professional_model.ProfessionalSpecialty, len(specialtyIDs))
		for i, sid := range specialtyIDs {
			records[i] = professional_model.ProfessionalSpecialty{
				ProfessionalID: professionalID,
				SpecialtyID:    sid,
			}
		}
		return tx.Create(&records).Error
	})
}
