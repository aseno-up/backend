package worker

import (
	"context"
	"time"

	"encoding/json"

	"github.com/aseno/backend/internal/logger"
	"gorm.io/gorm"

	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	systemlog_model "github.com/aseno/backend/pkg/systemlog/model"
)

// PaymentTimeoutWorker cancels appointments stuck in awaiting_payment status
// after the configured timeout period
type PaymentTimeoutWorker struct {
	db              *gorm.DB
	timeoutMinutes  int
	intervalMinutes int
	stopCh          chan struct{}
}

func NewPaymentTimeoutWorker(db *gorm.DB, timeoutMinutes int) *PaymentTimeoutWorker {
	return &PaymentTimeoutWorker{
		db:              db,
		timeoutMinutes:  timeoutMinutes,
		intervalMinutes: 5,
		stopCh:          make(chan struct{}),
	}
}

func (w *PaymentTimeoutWorker) Start() {
	logger.Printf("Starting payment timeout worker (interval: %dm, timeout: %dm)", w.intervalMinutes, w.timeoutMinutes)

	// Run immediately
	w.processExpired()

	ticker := time.NewTicker(time.Duration(w.intervalMinutes) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processExpired()
		case <-w.stopCh:
			logger.Println("Payment timeout worker stopped")
			return
		}
	}
}

func (w *PaymentTimeoutWorker) Stop() {
	close(w.stopCh)
}

func (w *PaymentTimeoutWorker) processExpired() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	now := time.Now()

	var appointments []appointment_model.Appointment
	result := w.db.WithContext(ctx).
		Where("status = ? AND payment_expires_at IS NOT NULL AND payment_expires_at < ?", appointment_model.AppointmentStatusAwaitingPayment, now).
		Limit(100).
		Find(&appointments)

	if result.Error != nil {
		logger.Printf("Payment timeout worker: error querying: %v", result.Error)
		return
	}

	if len(appointments) == 0 {
		logger.Println("Payment timeout worker: no expired appointments found")
		return
	}

	logger.Printf("Payment timeout worker: canceling %d expired awaiting_payment appointments", len(appointments))

	canceled := 0
	for _, apt := range appointments {
		err := w.db.WithContext(ctx).Model(&appointment_model.Appointment{}).
			Where("id = ? AND status = ?", apt.ID, appointment_model.AppointmentStatusAwaitingPayment).
			Updates(map[string]interface{}{
				"status":        appointment_model.AppointmentStatusCanceled,
				"cancel_reason": "Pagamento nao realizado dentro do prazo",
				"canceled_at":   time.Now(),
			}).Error

		if err != nil {
			logger.Printf("Payment timeout worker: failed to cancel appointment %s: %v", apt.ID, err)
			continue
		}

		// Log to system_logs
		payload, _ := json.Marshal(map[string]interface{}{"appointment_id": apt.ID.String(), "timeout_minutes": w.timeoutMinutes})
		w.db.WithContext(ctx).Create(&systemlog_model.SystemLog{
			ClinicID: &apt.ClinicID,
			Service:  "marketplace",
			Event:    "payment_timeout_cancel",
			Status:   systemlog_model.LogStatusSuccess,
			Payload:  payload,
		})

		canceled++
	}

	if canceled > 0 {
		logger.Printf("Payment timeout worker: %d appointments canceled", canceled)
	}
}
