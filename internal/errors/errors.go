package errors

import (
	"fmt"
	"net/http"

	"gorm.io/gorm"
)

// Error types constants
const (
	ErrInvalid               = "INVALID"
	ErrNotFound              = "NOT_FOUND"
	ErrUnauthorized          = "UNAUTHORIZED"
	ErrForbidden             = "FORBIDDEN"
	ErrInternal              = "INTERNAL"
	ErrConflict              = "CONFLICT"
	ErrBadRequest            = "BAD_REQUEST"
	ErrValidation            = "VALIDATION"
	ErrTenantNotFound        = "TENANT_NOT_FOUND"
	ErrUserNotFound          = "USER_NOT_FOUND"
	ErrClinicNotFound        = "CLINIC_NOT_FOUND"
	ErrPatientNotFound       = "PATIENT_NOT_FOUND"
	ErrProfessionalNotFound  = "PROFESSIONAL_NOT_FOUND"
	ErrAppointmentNotFound   = "APPOINTMENT_NOT_FOUND"
	ErrInvalidCredentials    = "INVALID_CREDENTIALS"
	ErrEmailAlreadyExists    = "EMAIL_ALREADY_EXISTS"
	ErrTokenExpired          = "TOKEN_EXPIRED"
	ErrTokenInvalid          = "TOKEN_INVALID"
	ErrSlotUnavailable       = "SLOT_UNAVAILABLE"
	ErrWhatsAppConnection    = "WHATSAPP_CONNECTION"
	ErrWhatsAppSendFailed    = "WHATSAPP_SEND_FAILED"
	ErrInvalidPayload        = "INVALID_PAYLOAD"
	ErrPaymentGateway        = "PAYMENT_GATEWAY"
	ErrPaymentNotFound       = "PAYMENT_NOT_FOUND"
	ErrSubscriptionNotFound  = "SUBSCRIPTION_NOT_FOUND"
	ErrCustomerNotFound      = "CUSTOMER_NOT_FOUND"
	ErrCalendarConnection    = "CALENDAR_CONNECTION"
	ErrCalendarSyncFailed    = "CALENDAR_SYNC_FAILED"
	ErrCalendarNotConfigured = "CALENDAR_NOT_CONFIGURED"
	ErrInviteNotFound        = "INVITE_NOT_FOUND"
	ErrInviteExpired         = "INVITE_EXPIRED"
	ErrInviteAlreadyAccepted = "INVITE_ALREADY_ACCEPTED"
	ErrInvitePending         = "INVITE_PENDING"
	ErrCannotChangeOwnRole   = "CANNOT_CHANGE_OWN_ROLE"
	ErrCannotDeactivateSelf  = "CANNOT_DEACTIVATE_SELF"
	ErrCannotRemoveOwner     = "CANNOT_REMOVE_OWNER"
	ErrPatientRecordNotFound = "PATIENT_RECORD_NOT_FOUND"
	ErrSessionNoteNotFound   = "SESSION_NOTE_NOT_FOUND"
	ErrUpgradeRequired       = "UPGRADE_REQUIRED"
	ErrSubscriptionExpired   = "SUBSCRIPTION_EXPIRED"
	ErrLeadNotFound          = "LEAD_NOT_FOUND"
	ErrLeadAlreadyConverted  = "LEAD_ALREADY_CONVERTED"
)

// AppError represents an application error with context
type AppError struct {
	Type       string            `json:"type"`
	Message    string            `json:"message"`
	StatusCode int               `json:"-"`
	Variables  map[string]string `json:"variables,omitempty"`
	Err        error             `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s - %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithError wraps an underlying error
func (e *AppError) WithError(err error) *AppError {
	return &AppError{
		Type:       e.Type,
		Message:    e.Message,
		StatusCode: e.StatusCode,
		Variables:  e.Variables,
		Err:        err,
	}
}

// WithVariables adds context variables to the error
func (e *AppError) WithVariables(vars map[string]string) *AppError {
	newVars := make(map[string]string)
	for k, v := range e.Variables {
		newVars[k] = v
	}
	for k, v := range vars {
		newVars[k] = v
	}
	return &AppError{
		Type:       e.Type,
		Message:    e.Message,
		StatusCode: e.StatusCode,
		Variables:  newVars,
		Err:        e.Err,
	}
}

// WithMessage creates a copy with a custom message
func (e *AppError) WithMessage(msg string) *AppError {
	return &AppError{
		Type:       e.Type,
		Message:    msg,
		StatusCode: e.StatusCode,
		Variables:  e.Variables,
		Err:        e.Err,
	}
}

// NewError creates a new AppError
func NewError(errType string, message string, statusCode int) *AppError {
	return &AppError{
		Type:       errType,
		Message:    message,
		StatusCode: statusCode,
	}
}

// Predefined error instances
var (
	ErrInvalidInstance      = NewError(ErrInvalid, "Invalid input", http.StatusBadRequest)
	ErrNotFoundInstance     = NewError(ErrNotFound, "Resource not found", http.StatusNotFound)
	ErrUnauthorizedInstance = NewError(ErrUnauthorized, "Unauthorized", http.StatusUnauthorized)
	ErrForbiddenInstance    = NewError(ErrForbidden, "Forbidden", http.StatusForbidden)
	ErrInternalInstance     = NewError(ErrInternal, "Internal server error", http.StatusInternalServerError)
	ErrConflictInstance     = NewError(ErrConflict, "Resource already exists", http.StatusConflict)
	ErrBadRequestInstance   = NewError(ErrBadRequest, "Bad request", http.StatusBadRequest)
	ErrValidationInstance   = NewError(ErrValidation, "Validation failed", http.StatusUnprocessableEntity)

	// Domain-specific errors
	ErrTenantNotFoundInstance        = NewError(ErrTenantNotFound, "Tenant not found", http.StatusNotFound)
	ErrUserNotFoundInstance          = NewError(ErrUserNotFound, "User not found", http.StatusNotFound)
	ErrClinicNotFoundInstance        = NewError(ErrClinicNotFound, "Clinic not found", http.StatusNotFound)
	ErrPatientNotFoundInstance       = NewError(ErrPatientNotFound, "Patient not found", http.StatusNotFound)
	ErrProfessionalNotFoundInstance  = NewError(ErrProfessionalNotFound, "Professional not found", http.StatusNotFound)
	ErrAppointmentNotFoundInstance   = NewError(ErrAppointmentNotFound, "Appointment not found", http.StatusNotFound)
	ErrInvalidCredentialsInstance    = NewError(ErrInvalidCredentials, "Invalid credentials", http.StatusUnauthorized)
	ErrEmailAlreadyExistsInstance    = NewError(ErrEmailAlreadyExists, "Email already exists", http.StatusConflict)
	ErrTokenExpiredInstance          = NewError(ErrTokenExpired, "Token has expired", http.StatusUnauthorized)
	ErrTokenInvalidInstance          = NewError(ErrTokenInvalid, "Token is invalid", http.StatusUnauthorized)
	ErrSlotUnavailableInstance       = NewError(ErrSlotUnavailable, "Time slot is not available", http.StatusConflict)
	ErrWhatsAppConnectionInstance    = NewError(ErrWhatsAppConnection, "WhatsApp connection failed", http.StatusServiceUnavailable)
	ErrWhatsAppSendFailedInstance    = NewError(ErrWhatsAppSendFailed, "Failed to send WhatsApp message", http.StatusBadGateway)
	ErrInvalidPayloadInstance        = NewError(ErrInvalidPayload, "Invalid request payload", http.StatusBadRequest)
	ErrPaymentGatewayInstance        = NewError(ErrPaymentGateway, "Payment gateway error", http.StatusBadGateway)
	ErrPaymentNotFoundInstance       = NewError(ErrPaymentNotFound, "Payment not found", http.StatusNotFound)
	ErrSubscriptionNotFoundInstance  = NewError(ErrSubscriptionNotFound, "Subscription not found", http.StatusNotFound)
	ErrCustomerNotFoundInstance      = NewError(ErrCustomerNotFound, "Customer not found", http.StatusNotFound)
	ErrCalendarConnectionInstance    = NewError(ErrCalendarConnection, "Calendar connection failed", http.StatusBadGateway)
	ErrCalendarSyncFailedInstance    = NewError(ErrCalendarSyncFailed, "Calendar sync failed", http.StatusBadGateway)
	ErrCalendarNotConfiguredInstance = NewError(ErrCalendarNotConfigured, "Calendar not configured", http.StatusBadRequest)
	ErrInviteNotFoundInstance        = NewError(ErrInviteNotFound, "Invite not found", http.StatusNotFound)
	ErrInviteExpiredInstance         = NewError(ErrInviteExpired, "Invite has expired", http.StatusGone)
	ErrInviteAlreadyAcceptedInstance = NewError(ErrInviteAlreadyAccepted, "Invite has already been accepted", http.StatusConflict)
	ErrInvitePendingInstance         = NewError(ErrInvitePending, "An invite is already pending for this email", http.StatusConflict)
	ErrCannotChangeOwnRoleInstance   = NewError(ErrCannotChangeOwnRole, "You cannot change your own role", http.StatusForbidden)
	ErrCannotDeactivateSelfInstance  = NewError(ErrCannotDeactivateSelf, "You cannot deactivate your own account", http.StatusForbidden)
	ErrCannotRemoveOwnerInstance     = NewError(ErrCannotRemoveOwner, "The clinic owner cannot be removed", http.StatusForbidden)
	ErrPatientRecordNotFoundInstance = NewError(ErrPatientRecordNotFound, "Patient record not found", http.StatusNotFound)
	ErrSessionNoteNotFoundInstance   = NewError(ErrSessionNoteNotFound, "Session note not found", http.StatusNotFound)
	ErrUpgradeRequiredInstance       = NewError(ErrUpgradeRequired, "Upgrade required to access this feature", http.StatusForbidden)
	ErrSubscriptionExpiredInstance   = NewError(ErrSubscriptionExpired, "Subscription has expired. Please renew to continue.", http.StatusPaymentRequired)
	ErrLeadNotFoundInstance          = NewError(ErrLeadNotFound, "Lead not found", http.StatusNotFound)
	ErrLeadAlreadyConvertedInstance  = NewError(ErrLeadAlreadyConverted, "Lead has already been converted", http.StatusConflict)
)

// IsNotFound checks if error is a not found error
func IsNotFound(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.StatusCode == http.StatusNotFound
	}
	// Also check for GORM's specific "record not found" error
	if err == gorm.ErrRecordNotFound {
		return true
	}
	return false
}

// IsUnauthorized checks if error is an unauthorized error
func IsUnauthorized(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// IsConflict checks if error is a conflict error
func IsConflict(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.StatusCode == http.StatusConflict
	}
	return false
}

// IsValidation checks if error is a validation error
func IsValidation(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrValidation
	}
	return false
}

// IsSlotUnavailable checks if error is a slot unavailable error
func IsSlotUnavailable(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrSlotUnavailable
	}
	return false
}

// GetStatusCode returns the HTTP status code for an error
func GetStatusCode(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.StatusCode
	}
	return http.StatusInternalServerError
}

// ToResponse converts an error to a response map
func ToResponse(err error) map[string]interface{} {
	if appErr, ok := err.(*AppError); ok {
		response := map[string]interface{}{
			"error": map[string]interface{}{
				"type":    appErr.Type,
				"message": appErr.Message,
			},
		}
		if len(appErr.Variables) > 0 {
			response["error"].(map[string]interface{})["variables"] = appErr.Variables
		}
		return response
	}
	return map[string]interface{}{
		"error": map[string]interface{}{
			"type":    ErrInternal,
			"message": "An unexpected error occurred",
		},
	}
}
