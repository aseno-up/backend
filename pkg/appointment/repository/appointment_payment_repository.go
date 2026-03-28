package repository

import (
	"github.com/aseno/backend/pkg/appointment/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AppointmentPaymentRepository interface {
	Create(payment *model.AppointmentPayment) error
	GetByID(id, clinicID uuid.UUID) (*model.AppointmentPayment, error)
	Update(payment *model.AppointmentPayment) error
	Delete(id, clinicID uuid.UUID) error
	ListByAppointment(appointmentID, clinicID uuid.UUID) ([]*model.AppointmentPayment, error)
	SumByAppointment(appointmentID uuid.UUID) (float64, error)
}

type appointmentPaymentRepository struct {
	db *gorm.DB
}

func NewAppointmentPaymentRepository(db *gorm.DB) AppointmentPaymentRepository {
	return &appointmentPaymentRepository{db: db}
}

func (r *appointmentPaymentRepository) Create(payment *model.AppointmentPayment) error {
	return r.db.Create(payment).Error
}

func (r *appointmentPaymentRepository) GetByID(id, clinicID uuid.UUID) (*model.AppointmentPayment, error) {
	var payment model.AppointmentPayment
	err := r.db.Where("id = ? AND clinic_id = ?", id, clinicID).First(&payment).Error
	return &payment, err
}

func (r *appointmentPaymentRepository) Update(payment *model.AppointmentPayment) error {
	return r.db.Where("id = ? AND clinic_id = ?", payment.ID, payment.ClinicID).
		Select("amount", "discount", "installments", "notes", "payment_method_id").
		Updates(payment).Error
}

func (r *appointmentPaymentRepository) Delete(id, clinicID uuid.UUID) error {
	return r.db.Where("id = ? AND clinic_id = ?", id, clinicID).Delete(&model.AppointmentPayment{}).Error
}

func (r *appointmentPaymentRepository) ListByAppointment(appointmentID, clinicID uuid.UUID) ([]*model.AppointmentPayment, error) {
	var payments []*model.AppointmentPayment
	err := r.db.
		Where("appointment_id = ? AND clinic_id = ?", appointmentID, clinicID).
		Preload("PaymentMethod").
		Order("paid_at DESC").
		Find(&payments).Error
	return payments, err
}

func (r *appointmentPaymentRepository) SumByAppointment(appointmentID uuid.UUID) (float64, error) {
	var sum float64
	err := r.db.Model(&model.AppointmentPayment{}).
		Where("appointment_id = ? AND deleted_at IS NULL", appointmentID).
		Select("COALESCE(SUM(amount + discount), 0)").
		Scan(&sum).Error
	return sum, err
}
