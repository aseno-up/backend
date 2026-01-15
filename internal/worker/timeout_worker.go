package worker

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/config"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
)

// TimeoutWorker scans for appointments with expired confirmation timeouts
// and marks them as no_response, notifying the clinic
type TimeoutWorker struct {
	config          *config.WorkerConfig
	appointmentRepo appointment_repository.AppointmentRepository
	stopCh          chan struct{}
}

// NewTimeoutWorker creates a new timeout worker
func NewTimeoutWorker(
	cfg *config.WorkerConfig,
	appointmentRepo appointment_repository.AppointmentRepository,
) *TimeoutWorker {
	return &TimeoutWorker{
		config:          cfg,
		appointmentRepo: appointmentRepo,
		stopCh:          make(chan struct{}),
	}
}

// Start starts the timeout worker
func (w *TimeoutWorker) Start() {
	if !w.config.TimeoutWorkerEnabled {
		log.Println("Timeout worker is disabled")
		return
	}

	log.Printf("Starting timeout worker (interval: %v, timeout duration: %v)",
		w.config.TimeoutWorkerInterval,
		w.config.ConfirmationTimeoutDuration)

	// Run immediately on start
	w.processTimeouts()

	ticker := time.NewTicker(w.config.TimeoutWorkerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processTimeouts()
		case <-w.stopCh:
			log.Println("Timeout worker stopped")
			return
		}
	}
}

// Stop stops the timeout worker
func (w *TimeoutWorker) Stop() {
	close(w.stopCh)
}

// processTimeouts processes all expired confirmation timeouts
func (w *TimeoutWorker) processTimeouts() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get appointments with expired confirmations
	appointments, err := w.appointmentRepo.GetExpiredConfirmations(ctx, 100)
	if err != nil {
		log.Printf("Error getting expired confirmations: %v", err)
		return
	}

	if len(appointments) == 0 {
		return
	}

	log.Printf("Processing %d expired confirmation timeouts...", len(appointments))

	processed := 0
	for _, apt := range appointments {
		if err := w.processExpiredAppointment(ctx, apt); err != nil {
			log.Printf("Error processing expired appointment %s: %v", apt.ID, err)
			continue
		}
		processed++
	}

	if processed > 0 {
		log.Printf("Timeout processing completed: %d appointments marked as no_response", processed)
	}
}

// processExpiredAppointment marks a single appointment as no_response
func (w *TimeoutWorker) processExpiredAppointment(ctx context.Context, apt *appointment_model.AppointmentWithClinic) error {
	// Mark as no_response
	if err := w.appointmentRepo.MarkConfirmationExpired(ctx, apt.ID); err != nil {
		return err
	}

	// Log the timeout event
	log.Printf("Appointment %s marked as no_response (patient: %s, clinic: %s)",
		apt.ID,
		apt.Patient.Name,
		apt.ClinicName)

	// TODO: Send notification to clinic (can be implemented later)
	// This could be:
	// - WhatsApp message to clinic owner
	// - Email notification
	// - Push notification to dashboard
	// - Internal notification in the system

	return nil
}

// Compile-time check that the repository has the required methods
var _ interface {
	GetExpiredConfirmations(ctx context.Context, limit int) ([]*appointment_model.AppointmentWithClinic, error)
	MarkConfirmationExpired(ctx context.Context, id uuid.UUID) error
} = (appointment_repository.AppointmentRepository)(nil)
