package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	paymentmethod_model "github.com/aseno/backend/pkg/paymentmethod/model"
)

// clinicRepoAdapter adapts the real ClinicRepository to the marketplace's ClinicRepository interface
type clinicRepoAdapter struct {
	db *gorm.DB
}

func NewClinicRepoAdapter(db *gorm.DB) ClinicRepository {
	return &clinicRepoAdapter{db: db}
}

func (a *clinicRepoAdapter) GetByID(ctx context.Context, id uuid.UUID) (ClinicInfo, error) {
	var clinic clinic_model.Clinic
	result := a.db.WithContext(ctx).Where("id = ?", id).First(&clinic)
	if result.Error != nil {
		return nil, result.Error
	}
	return &clinicInfoImpl{clinic: &clinic}, nil
}

type clinicInfoImpl struct {
	clinic *clinic_model.Clinic
}

func (c *clinicInfoImpl) GetID() uuid.UUID                    { return c.clinic.ID }
func (c *clinicInfoImpl) GetAccountID() uuid.UUID              { return c.clinic.AccountID }
func (c *clinicInfoImpl) GetRequirePaymentBeforeBooking() bool { return c.clinic.Settings.RequirePaymentBeforeBooking }
func (c *clinicInfoImpl) GetPlanSlug() string {
	if c.clinic.Account != nil && c.clinic.Account.Plan != nil {
		return c.clinic.Account.Plan.Slug
	}
	return "basic"
}
func (c *clinicInfoImpl) GetSubscriptionStatus() string {
	if c.clinic.Account != nil {
		return string(c.clinic.Account.SubscriptionStatus)
	}
	return ""
}

// appointmentRepoAdapter adapts the real AppointmentRepository
type appointmentRepoAdapter struct {
	db *gorm.DB
}

func NewAppointmentRepoAdapter(db *gorm.DB) AppointmentRepository {
	return &appointmentRepoAdapter{db: db}
}

func (a *appointmentRepoAdapter) GetByID(ctx context.Context, appointmentID uuid.UUID, clinicID uuid.UUID) (AppointmentInfo, error) {
	var apt appointment_model.Appointment
	result := a.db.WithContext(ctx).Where("id = ? AND clinic_id = ?", appointmentID, clinicID).First(&apt)
	if result.Error != nil {
		return nil, result.Error
	}
	return &appointmentInfoImpl{apt: &apt}, nil
}

func (a *appointmentRepoAdapter) GetByPaymentToken(ctx context.Context, token string) (AppointmentInfo, error) {
	var apt appointment_model.Appointment
	result := a.db.WithContext(ctx).Where("payment_token = ?", token).First(&apt)
	if result.Error != nil {
		return nil, result.Error
	}
	return &appointmentInfoImpl{apt: &apt}, nil
}

func (a *appointmentRepoAdapter) UpdatePaymentStatus(ctx context.Context, appointmentID uuid.UUID, status string) error {
	return a.db.WithContext(ctx).Model(&appointment_model.Appointment{}).
		Where("id = ?", appointmentID).Update("payment_status", status).Error
}

func (a *appointmentRepoAdapter) UpdateStatus(ctx context.Context, appointmentID uuid.UUID, status string) error {
	return a.db.WithContext(ctx).Model(&appointment_model.Appointment{}).
		Where("id = ?", appointmentID).Update("status", status).Error
}

type appointmentInfoImpl struct {
	apt *appointment_model.Appointment
}

func (a *appointmentInfoImpl) GetID() uuid.UUID              { return a.apt.ID }
func (a *appointmentInfoImpl) GetClinicID() uuid.UUID        { return a.apt.ClinicID }
func (a *appointmentInfoImpl) GetPatientID() uuid.UUID { return a.apt.PatientID }
func (a *appointmentInfoImpl) GetPrice() float64              { return a.apt.Price }
func (a *appointmentInfoImpl) GetStatus() string              { return string(a.apt.Status) }
func (a *appointmentInfoImpl) GetPaymentToken() string        { return a.apt.PaymentToken }
func (a *appointmentInfoImpl) GetPaymentExpiresAt() *time.Time { return a.apt.PaymentExpiresAt }

// appointmentPaymentRepoAdapter adapts AppointmentPayment operations
type appointmentPaymentRepoAdapter struct {
	db *gorm.DB
}

func NewAppointmentPaymentRepoAdapter(db *gorm.DB) AppointmentPaymentRepository {
	return &appointmentPaymentRepoAdapter{db: db}
}

func (a *appointmentPaymentRepoAdapter) SumByAppointment(appointmentID uuid.UUID) (float64, error) {
	var total float64
	result := a.db.Model(&appointment_model.AppointmentPayment{}).
		Where("appointment_id = ?", appointmentID).
		Select("COALESCE(SUM(amount), 0)").Scan(&total)
	if result.Error != nil {
		return 0, result.Error
	}
	return total, nil
}

func (a *appointmentPaymentRepoAdapter) CreateFromWebhook(ctx context.Context, clinicID, appointmentID uuid.UUID, paymentMethodID *uuid.UUID, amount float64, paidAt time.Time) error {
	// Find or create a "PIX Online" payment method for the clinic
	var pmID uuid.UUID
	if paymentMethodID != nil {
		pmID = *paymentMethodID
	} else {
		var pm paymentmethod_model.PaymentMethod
		result := a.db.WithContext(ctx).
			Where("clinic_id = ? AND name = ? AND type = ?", clinicID, "PIX Online (Asaas)", "pix").
			First(&pm)
		if result.Error != nil {
			pm = paymentmethod_model.PaymentMethod{
				ClinicID: clinicID,
				Name:     "PIX Online (Asaas)",
				Type:     "pix",
				Active:   true,
			}
			if err := a.db.WithContext(ctx).Create(&pm).Error; err != nil {
				return err
			}
		}
		pmID = pm.ID
	}

	aptPayment := &appointment_model.AppointmentPayment{
		ClinicID:        clinicID,
		AppointmentID:   appointmentID,
		PaymentMethodID: pmID,
		Amount:          amount,
		PaidAt:          paidAt,
	}

	return a.db.WithContext(ctx).Create(aptPayment).Error
}
