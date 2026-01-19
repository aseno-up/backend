package main

import (
	"github.com/aseno/backend/internal/logger"
)

func main() {
	// Initialize logger (optional but recommended)
	logger.Init()

	// Examples of colored logging
	logger.Printf("[RabbitMQ] Connected to queue: %s", "notifications")
	logger.Printf("[Redis] Connected successfully")
	logger.Printf("[Schedule] Processing appointment for patient ID: %s", "12345")
	logger.Printf("[Reschedule] Failed to auto-confirm appointment: %v", "slot unavailable")

	// Worker logs
	logger.Printf("[ScheduledMessageWorker] Processing %d patients", 42)
	logger.Printf("[MessageSender] Successfully sent %s message to %s", "confirmation", "+1234567890")
	logger.Printf("[RateLimiter] Same patient (%s), no delay needed", "patient-123")

	// Service logs
	logger.Printf("[EMAIL] Sending invite to user@example.com")
	logger.Printf("[WhatsApp] Message sent successfully")
	logger.Printf("[AUTH] User logged in: %s", "john@example.com")
	logger.Printf("[Payment] Processing payment of $%.2f", 150.00)

	// Database and infrastructure
	logger.Printf("[Migration] Running migration: %s", "20240101_create_users")
	logger.Printf("[PostgreSQL] Query executed in %dms", 45)
	logger.Printf("[Database] Connection pool: %d/%d connections", 5, 10)

	// Error levels using helper functions
	logger.Infof("Application started successfully")
	logger.Successf("All %d appointments synced", 100)
	logger.Warnf("Rate limit approaching: %d/%d requests", 95, 100)
	logger.Errorf("Failed to process booking: %v", "invalid date")
	logger.Debugf("Cache hit for key: %s", "user:12345")

	// Multiple brackets in one message
	logger.Printf("[Schedule] [INFO] Found %d available slots for [Appointment]", 5)

	// Custom message with existing brackets
	logger.Printf("[Booking] Creating booking for clinic [ClinicA] with patient [PatientB]")
}
