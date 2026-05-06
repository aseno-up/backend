package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aseno/backend/internal/config"
	"github.com/aseno/backend/internal/database"
	"github.com/aseno/backend/internal/logger"
	"github.com/aseno/backend/internal/queue"
	redisClient "github.com/aseno/backend/internal/redis"
	"github.com/aseno/backend/internal/worker"
	account_model "github.com/aseno/backend/pkg/account/model"
	account_repository "github.com/aseno/backend/pkg/account/repository"
	aitokenusage_handler "github.com/aseno/backend/pkg/aitokenusage/handler"
	aitokenusage_model "github.com/aseno/backend/pkg/aitokenusage/model"
	aitokenusage_repository "github.com/aseno/backend/pkg/aitokenusage/repository"
	aitokenusage_service "github.com/aseno/backend/pkg/aitokenusage/service"
	analytics_handler "github.com/aseno/backend/pkg/analytics/handler"
	analytics_repository "github.com/aseno/backend/pkg/analytics/repository"
	analytics_service "github.com/aseno/backend/pkg/analytics/service"
	appointment_handler "github.com/aseno/backend/pkg/appointment/handler"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
	appointment_service "github.com/aseno/backend/pkg/appointment/service"
	attendance_handler "github.com/aseno/backend/pkg/attendance/handler"
	attendance_model "github.com/aseno/backend/pkg/attendance/model"
	attendance_repository "github.com/aseno/backend/pkg/attendance/repository"
	attendance_service "github.com/aseno/backend/pkg/attendance/service"
	audit_handler "github.com/aseno/backend/pkg/audit/handler"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_repository "github.com/aseno/backend/pkg/audit/repository"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	auth_google "github.com/aseno/backend/pkg/auth/client/google"
	auth_handler "github.com/aseno/backend/pkg/auth/handler"
	auth_middleware "github.com/aseno/backend/pkg/auth/middleware"
	auth_model "github.com/aseno/backend/pkg/auth/model"
	auth_repository "github.com/aseno/backend/pkg/auth/repository"
	auth_service "github.com/aseno/backend/pkg/auth/service"
	booking_handler "github.com/aseno/backend/pkg/booking/handler"
	booking_service "github.com/aseno/backend/pkg/booking/service"
	calendar_google "github.com/aseno/backend/pkg/calendar/client/google"
	calendar_handler "github.com/aseno/backend/pkg/calendar/handler"
	calendar_model "github.com/aseno/backend/pkg/calendar/model"
	calendar_repository "github.com/aseno/backend/pkg/calendar/repository"
	calendar_service "github.com/aseno/backend/pkg/calendar/service"
	clinic_handler "github.com/aseno/backend/pkg/clinic/handler"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	clinic_service "github.com/aseno/backend/pkg/clinic/service"
	clinicmember_model "github.com/aseno/backend/pkg/clinicmember/model"
	clinicmember_repository "github.com/aseno/backend/pkg/clinicmember/repository"
	commission_handler "github.com/aseno/backend/pkg/commission/handler"
	commission_model "github.com/aseno/backend/pkg/commission/model"
	commission_repository "github.com/aseno/backend/pkg/commission/repository"
	commission_service "github.com/aseno/backend/pkg/commission/service"
	dashboard_handler "github.com/aseno/backend/pkg/dashboard/handler"
	dashboard_repository "github.com/aseno/backend/pkg/dashboard/repository"
	dashboard_service "github.com/aseno/backend/pkg/dashboard/service"
	email_service "github.com/aseno/backend/pkg/email/service"
	enrollment_handler "github.com/aseno/backend/pkg/enrollment/handler"
	enrollment_model "github.com/aseno/backend/pkg/enrollment/model"
	enrollment_repository "github.com/aseno/backend/pkg/enrollment/repository"
	enrollment_service "github.com/aseno/backend/pkg/enrollment/service"
	featureflag_handler "github.com/aseno/backend/pkg/featureflag/handler"
	featureflag_model "github.com/aseno/backend/pkg/featureflag/model"
	featureflag_repository "github.com/aseno/backend/pkg/featureflag/repository"
	featureflag_service "github.com/aseno/backend/pkg/featureflag/service"
	finance_handler "github.com/aseno/backend/pkg/finance/handler"
	finance_model "github.com/aseno/backend/pkg/finance/model"
	finance_repository "github.com/aseno/backend/pkg/finance/repository"
	finance_service "github.com/aseno/backend/pkg/finance/service"
	health_handler "github.com/aseno/backend/pkg/health/handler"
	invite_handler "github.com/aseno/backend/pkg/invite/handler"
	invite_model "github.com/aseno/backend/pkg/invite/model"
	invite_repository "github.com/aseno/backend/pkg/invite/repository"
	invite_service "github.com/aseno/backend/pkg/invite/service"
	lead_crm "github.com/aseno/backend/pkg/lead/crm"
	lead_notion "github.com/aseno/backend/pkg/lead/crm/notion"
	lead_handler "github.com/aseno/backend/pkg/lead/handler"
	lead_model "github.com/aseno/backend/pkg/lead/model"
	lead_repository "github.com/aseno/backend/pkg/lead/repository"
	lead_service "github.com/aseno/backend/pkg/lead/service"
	marketplace_handler "github.com/aseno/backend/pkg/marketplace/handler"
	marketplace_model "github.com/aseno/backend/pkg/marketplace/model"
	marketplace_repository "github.com/aseno/backend/pkg/marketplace/repository"
	marketplace_service "github.com/aseno/backend/pkg/marketplace/service"
	notification_handler "github.com/aseno/backend/pkg/notification/handler"
	notification_model "github.com/aseno/backend/pkg/notification/model"
	notification_repository "github.com/aseno/backend/pkg/notification/repository"
	notification_service "github.com/aseno/backend/pkg/notification/service"
	"github.com/aseno/backend/pkg/pagination"
	partnernetwork_handler "github.com/aseno/backend/pkg/partnernetwork/handler"
	partnernetwork_model "github.com/aseno/backend/pkg/partnernetwork/model"
	partnernetwork_repository "github.com/aseno/backend/pkg/partnernetwork/repository"
	partnernetwork_service "github.com/aseno/backend/pkg/partnernetwork/service"
	patient_handler "github.com/aseno/backend/pkg/patient/handler"
	patient_model "github.com/aseno/backend/pkg/patient/model"
	patient_repository "github.com/aseno/backend/pkg/patient/repository"
	patient_service "github.com/aseno/backend/pkg/patient/service"
	patientrecord_handler "github.com/aseno/backend/pkg/patientrecord/handler"
	patientrecord_model "github.com/aseno/backend/pkg/patientrecord/model"
	patientrecord_repository "github.com/aseno/backend/pkg/patientrecord/repository"
	patientrecord_service "github.com/aseno/backend/pkg/patientrecord/service"
	payment_asaas "github.com/aseno/backend/pkg/payment/client/asaas"
	payment_handler "github.com/aseno/backend/pkg/payment/handler"
	payment_model "github.com/aseno/backend/pkg/payment/model"
	payment_repository "github.com/aseno/backend/pkg/payment/repository"
	payment_service "github.com/aseno/backend/pkg/payment/service"
	paymentmethod_handler "github.com/aseno/backend/pkg/paymentmethod/handler"
	paymentmethod_model "github.com/aseno/backend/pkg/paymentmethod/model"
	paymentmethod_repository "github.com/aseno/backend/pkg/paymentmethod/repository"
	paymentmethod_service "github.com/aseno/backend/pkg/paymentmethod/service"
	plan_handler "github.com/aseno/backend/pkg/plan/handler"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	plan_repository "github.com/aseno/backend/pkg/plan/repository"
	plan_service "github.com/aseno/backend/pkg/plan/service"
	portalaccount_handler "github.com/aseno/backend/pkg/portalaccount/handler"
	portalaccount_middleware "github.com/aseno/backend/pkg/portalaccount/middleware"
	portalaccount_model "github.com/aseno/backend/pkg/portalaccount/model"
	portalaccount_repository "github.com/aseno/backend/pkg/portalaccount/repository"
	portalaccount_service "github.com/aseno/backend/pkg/portalaccount/service"
	procedure_handler "github.com/aseno/backend/pkg/procedure/handler"
	procedure_model "github.com/aseno/backend/pkg/procedure/model"
	procedure_repository "github.com/aseno/backend/pkg/procedure/repository"
	procedure_service "github.com/aseno/backend/pkg/procedure/service"
	professional_handler "github.com/aseno/backend/pkg/professional/handler"
	professional_model "github.com/aseno/backend/pkg/professional/model"
	professional_repository "github.com/aseno/backend/pkg/professional/repository"
	professional_service "github.com/aseno/backend/pkg/professional/service"
	public_handler "github.com/aseno/backend/pkg/public/handler"
	salespackage_handler "github.com/aseno/backend/pkg/salespackage/handler"
	salespackage_model "github.com/aseno/backend/pkg/salespackage/model"
	salespackage_repository "github.com/aseno/backend/pkg/salespackage/repository"
	salespackage_service "github.com/aseno/backend/pkg/salespackage/service"
	sessionplan_handler "github.com/aseno/backend/pkg/sessionplan/handler"
	sessionplan_migration "github.com/aseno/backend/pkg/sessionplan/migration"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
	sessionplan_repository "github.com/aseno/backend/pkg/sessionplan/repository"
	sessionplan_service "github.com/aseno/backend/pkg/sessionplan/service"
	settings_handler "github.com/aseno/backend/pkg/settings/handler"
	settings_model "github.com/aseno/backend/pkg/settings/model"
	settings_repository "github.com/aseno/backend/pkg/settings/repository"
	settings_service "github.com/aseno/backend/pkg/settings/service"
	specialty_handler "github.com/aseno/backend/pkg/specialty/handler"
	specialty_model "github.com/aseno/backend/pkg/specialty/model"
	specialty_repository "github.com/aseno/backend/pkg/specialty/repository"
	specialty_service "github.com/aseno/backend/pkg/specialty/service"
	systemlog_handler "github.com/aseno/backend/pkg/systemlog/handler"
	systemlog_middleware "github.com/aseno/backend/pkg/systemlog/middleware"
	systemlog_model "github.com/aseno/backend/pkg/systemlog/model"
	systemlog_repository "github.com/aseno/backend/pkg/systemlog/repository"
	systemlog_service "github.com/aseno/backend/pkg/systemlog/service"
	usagemetric_handler "github.com/aseno/backend/pkg/usagemetric/handler"
	usagemetric_model "github.com/aseno/backend/pkg/usagemetric/model"
	usagemetric_repository "github.com/aseno/backend/pkg/usagemetric/repository"
	usagemetric_service "github.com/aseno/backend/pkg/usagemetric/service"
	user_handler "github.com/aseno/backend/pkg/user/handler"
	user_model "github.com/aseno/backend/pkg/user/model"
	user_repository "github.com/aseno/backend/pkg/user/repository"
	user_service "github.com/aseno/backend/pkg/user/service"
	waitlist_handler "github.com/aseno/backend/pkg/waitlist/handler"
	waitlist_model "github.com/aseno/backend/pkg/waitlist/model"
	waitlist_repository "github.com/aseno/backend/pkg/waitlist/repository"
	waitlist_service "github.com/aseno/backend/pkg/waitlist/service"
	"github.com/aseno/backend/pkg/whatsapp/agent"
	whatsapp_classifier "github.com/aseno/backend/pkg/whatsapp/classifier"
	whatsapp_client "github.com/aseno/backend/pkg/whatsapp/client"
	whatsapp_evolution "github.com/aseno/backend/pkg/whatsapp/client/evolution"
	whatsapp_gupshup "github.com/aseno/backend/pkg/whatsapp/client/gupshup"
	whatsapp_waha "github.com/aseno/backend/pkg/whatsapp/client/waha"
	"github.com/aseno/backend/pkg/whatsapp/conversation"
	whatsapp_handler "github.com/aseno/backend/pkg/whatsapp/handler"
	whatsapp_model "github.com/aseno/backend/pkg/whatsapp/model"
	whatsapp_repository "github.com/aseno/backend/pkg/whatsapp/repository"
	whatsapp_service "github.com/aseno/backend/pkg/whatsapp/service"
	"github.com/aseno/backend/pkg/whatsapp/tools"
	whatsapp_transcriber "github.com/aseno/backend/pkg/whatsapp/transcriber"
)

func main() {
	// Initialize colored logger
	logger.Init()

	// Load configuration
	cfg := config.NewConfig()
	if err := cfg.Validate(); err != nil {
		logger.Fatalf("Invalid configuration: %v", err)
	}

	// Set Gin mode
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize database
	db, err := database.NewDatabase(cfg.GetDatabaseURL(), cfg.IsDevelopment())
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Printf("Error closing database: %v", err)
		}
	}()

	// Run migrations
	if err := runMigrations(db); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	accountRepo := account_repository.NewAccountRepository(db.GetDB())
	clinicRepo := clinic_repository.NewClinicRepository(db.GetDB())
	userRepo := user_repository.NewUserRepository(db.GetDB())
	professionalRepo := professional_repository.NewProfessionalRepository(db.GetDB())
	professionalProcedureRepo := professional_repository.NewProfessionalProcedureRepository(db.GetDB())
	commissionRepo := commission_repository.NewCommissionRepository(db.GetDB())
	clinicMemberRepo := clinicmember_repository.NewClinicMemberRepository(db.GetDB())
	patientRepo := patient_repository.NewPatientRepository(db.GetDB())
	appointmentRepo := appointment_repository.NewAppointmentRepository(db.GetDB())
	procedureRepo := procedure_repository.NewProcedureRepository(db.GetDB())
	messageRepo := whatsapp_repository.NewMessageRepository(db.GetDB())
	messageLogRepo := whatsapp_repository.NewMessageLogRepository(db.GetDB())
	scheduledMsgRepo := whatsapp_repository.NewScheduledMessageRepository(db.GetDB())
	customerRepo := payment_repository.NewCustomerRepository(db.GetDB())
	paymentRepo := payment_repository.NewPaymentRepository(db.GetDB())
	subscriptionRepo := payment_repository.NewSubscriptionRepository(db.GetDB())
	calendarIntegrationRepo := calendar_repository.NewIntegrationRepository(db.GetDB())
	calendarSyncRepo := calendar_repository.NewSyncRepository(db.GetDB())
	sessionPlanRepo := sessionplan_repository.NewSessionPlanRepository(db.GetDB())
	sessionPlanSlotRepo := sessionplan_repository.NewSessionPlanSlotRepository(db.GetDB())
	inviteRepo := invite_repository.NewInviteRepository(db.GetDB())
	patientRecordRepo := patientrecord_repository.NewPatientRecordRepository(db.GetDB())
	planRepo := plan_repository.NewPlanRepository(db.GetDB())
	offerRepo := plan_repository.NewOfferRepository(db.GetDB())
	dashboardRepo := dashboard_repository.NewDashboardRepository(db.GetDB())
	passwordResetRepo := auth_repository.NewPasswordResetRepository(db.GetDB())
	emailVerificationRepo := auth_repository.NewEmailVerificationRepository(db.GetDB())
	waitlistRepo := waitlist_repository.NewWaitlistRepository(db.GetDB())
	analyticsRepo := analytics_repository.NewAnalyticsRepository(db.GetDB())
	notificationRepo := notification_repository.NewNotificationRepository(db.GetDB())
	settingsRepo := settings_repository.NewSettingsRepository(db.GetDB())
	whatsappConnRepo := whatsapp_repository.NewWhatsAppConnectionRepository(db.GetDB())
	auditRepo := audit_repository.NewAuditRepository(db.GetDB())
	systemLogRepo := systemlog_repository.NewSystemLogRepository(db.GetDB())
	featureFlagRepo := featureflag_repository.NewFeatureFlagRepository(db.GetDB())
	usageMetricRepo := usagemetric_repository.NewUsageMetricRepository(db.GetDB())
	aiTokenUsageRepo := aitokenusage_repository.NewRepository(db.GetDB())
	leadRepo := lead_repository.NewLeadRepository(db.GetDB())
	paymentMethodRepo := paymentmethod_repository.NewPaymentMethodRepository(db.GetDB())
	appointmentPaymentRepo := appointment_repository.NewAppointmentPaymentRepository(db.GetDB())
	appointmentReceiptTokenRepo := appointment_repository.NewAppointmentReceiptTokenRepository(db.GetDB())
	specialtyRepo := specialty_repository.NewSpecialtyRepository(db.GetDB())
	partnerNetworkRepo := partnernetwork_repository.NewPartnerNetworkRepository(db.GetDB())
	partnerPlanRepo := partnernetwork_repository.NewPartnerPlanRepository(db.GetDB())
	financeTransactionRepo := finance_repository.NewTransactionRepository(db.GetDB())
	financeCategoryRepo := finance_repository.NewCategoryRepository(db.GetDB())

	// Seed default settings (if not exist)
	if err := settingsRepo.SeedDefaults(context.Background()); err != nil {
		logger.Printf("Warning: Failed to seed default settings: %v", err)
	}

	// Seed default plans (if not exist)
	if err := seedDefaultPlans(db.GetDB()); err != nil {
		logger.Printf("Warning: Failed to seed default plans: %v", err)
	}

	// Initialize Settings service (used by professional service and public config)
	settingsSvc := settings_service.NewSettingsService(settingsRepo)

	// Initialize Sprint 6 services (audit, system logs, feature flags, usage metrics)
	auditSvc := audit_service.NewAuditService(auditRepo)
	systemLogSvc := systemlog_service.NewSystemLogService(systemLogRepo)
	featureFlagSvc := featureflag_service.NewFeatureFlagService(featureFlagRepo)
	usageMetricSvc := usagemetric_service.NewUsageMetricService(usageMetricRepo)
	aiTokenUsageSvc := aitokenusage_service.NewService(aiTokenUsageRepo)

	// Initialize Notification service (needed early for booking and other services)
	notificationSvc := notification_service.NewNotificationService(notificationRepo)

	// Initialize Booking service (needed early for AI Agent and public booking)
	// BookingService initialized below after emailSvc is available

	// Initialize WhatsApp clients for all available providers
	var evolutionClient whatsapp_service.WhatsAppClient
	if cfg.GetEvolutionAPIURL() != "" {
		evolutionClient = whatsapp_evolution.NewEvolutionClient(whatsapp_evolution.Config{
			BaseURL:    cfg.GetEvolutionAPIURL(),
			APIKey:     cfg.GetEvolutionAPIKey(),
			WebhookURL: cfg.GetWhatsAppWebhookURL(),
		})
		logger.Println("WhatsApp (Evolution API) client initialized")
	}

	var wahaClient whatsapp_service.WhatsAppClient
	if cfg.GetWahaAPIURL() != "" {
		wahaClient = whatsapp_waha.NewWahaClient(whatsapp_waha.Config{
			BaseURL: cfg.GetWahaAPIURL(),
			APIKey:  cfg.GetWahaAPIKey(),
		})
		logger.Println("WhatsApp (WAHA) client initialized")
	}

	// Initialize Gupshup/Official Client with partner-level API key
	gupshupClient := whatsapp_gupshup.NewGupshupClient(whatsapp_gupshup.Config{
		BaseURL: cfg.GetGupshupAPIURL(),
		APIKey:  cfg.GetGupshupAPIKey(),
	}, whatsappConnRepo)
	logger.Println("WhatsApp (Gupshup/Official) client initialized")

	// Wrap clients in ProxyClient to route requests dynamically per clinic
	whatsappClient := whatsapp_client.NewProxyClient(
		whatsappConnRepo,
		evolutionClient,
		wahaClient,
		gupshupClient,
	)

	// Initialize Asaas client (Payment Gateway)
	var asaasClient *payment_asaas.Client
	if cfg.GetAsaasAPIKey() != "" {
		asaasClient = payment_asaas.NewClient(payment_asaas.Config{
			BaseURL:   cfg.GetAsaasAPIURL(),
			APIKey:    cfg.GetAsaasAPIKey(),
			IsSandbox: cfg.AsaasSandbox,
		})
		logger.Println("Asaas payment client initialized")
	}

	// Initialize Google Auth client (for OAuth login)
	var googleAuthClient *auth_google.Client
	if cfg.GetGoogleClientID() != "" {
		googleAuthClient = auth_google.NewClient(auth_google.Config{
			ClientID:     cfg.GetGoogleClientID(),
			ClientSecret: cfg.GetGoogleClientSecret(),
			RedirectURL:  cfg.GoogleAuthRedirectURL,
		})
		logger.Println("Google Auth client initialized")
	}

	// Initialize Google Calendar client
	var googleCalendarClient *calendar_google.Client
	if cfg.GetGoogleClientID() != "" && cfg.GetGoogleClientSecret() != "" {
		googleCalendarClient = calendar_google.NewClient(calendar_google.Config{
			ClientID:     cfg.GetGoogleClientID(),
			ClientSecret: cfg.GetGoogleClientSecret(),
			RedirectURL:  cfg.GoogleRedirectURL,
		})
		logger.Println("Google Calendar client initialized")
	}

	// Initialize services
	jwtService := auth_service.NewJWTService(
		cfg.GetJWTSecret(),
		cfg.JWTAccessTokenTTL,
		cfg.JWTRefreshTokenTTL,
	)

	// Initialize Email service
	var emailSvc email_service.EmailService
	if cfg.GetBrevoAPIKey() != "" {
		var err error
		emailSvc, err = email_service.NewBrevoEmailService(cfg.GetBrevoAPIKey(), cfg.GetEmailSenderSupportAddress(), cfg.GetEmailSenderName())
		if err != nil {
			logger.Fatalf("Failed to initialize Brevo email service: %v", err)
		}
		logger.Println("Brevo email service initialized")
	} else if cfg.GetSMTPHost() != "" {
		emailSvc = email_service.NewSMTPEmailService(email_service.SMTPConfig{
			Host:        cfg.GetSMTPHost(),
			Port:        cfg.GetSMTPPort(),
			User:        cfg.GetSMTPUser(),
			Password:    cfg.GetSMTPPassword(),
			From:        cfg.GetSMTPFrom(),
			FrontendURL: cfg.GetFrontendURL(),
		})
		logger.Println("SMTP email service initialized")
	} else {
		emailSvc = email_service.NewNoOpEmailService()
		logger.Println("Using no-op email service (no email provider configured)")
	}

	// Initialize Booking service (needs emailSvc, used by AI Agent and public booking)
	portalAccountRepo := portalaccount_repository.NewPortalAccountRepository(db.GetDB())

	bookingSvc := booking_service.NewBookingService(
		clinicRepo,
		professionalRepo,
		appointmentRepo,
		patientRepo,
		notificationSvc,
		specialtyRepo,
		procedureRepo,
		professionalProcedureRepo,
		portalAccountRepo,
		emailSvc,
		cfg.GetFrontendURL(),
		cfg.GetBookingPaymentTimeoutMinutes(),
	)

	// Initialize auth service with or without Google OAuth
	var authService auth_service.AuthService
	var googleAdapter auth_service.GoogleAuthClient
	if googleAuthClient != nil {
		googleAdapter = &googleAuthAdapter{client: googleAuthClient}
		authService = auth_service.NewAuthServiceWithGoogle(userRepo, clinicRepo, accountRepo, passwordResetRepo, emailVerificationRepo, planRepo, professionalRepo, clinicMemberRepo, jwtService, googleAdapter, emailSvc, cfg.GetFrontendURL())
	} else {
		authService = auth_service.NewAuthService(userRepo, clinicRepo, accountRepo, passwordResetRepo, emailVerificationRepo, planRepo, professionalRepo, clinicMemberRepo, jwtService, emailSvc, cfg.GetFrontendURL())
	}
	professionalService := professional_service.NewProfessionalService(professionalRepo, clinicRepo, appointmentRepo, userRepo, specialtyRepo)
	professionalProcedureService := professional_service.NewProfessionalProcedureService(professionalProcedureRepo, professionalRepo, procedureRepo)
	commissionSvc := commission_service.NewCommissionService(commissionRepo, professionalProcedureRepo, appointmentRepo)
	patientService := patient_service.NewPatientService(patientRepo, clinicRepo)
	appointmentService := appointment_service.NewAppointmentService(appointmentRepo, professionalRepo)
	procedureService := procedure_service.NewProcedureService(procedureRepo)
	paymentMethodService := paymentmethod_service.NewPaymentMethodService(paymentMethodRepo)
	specialtyService := specialty_service.NewSpecialtyService(specialtyRepo)
	partnerNetworkService := partnernetwork_service.NewPartnerNetworkService(partnerNetworkRepo)
	partnerPlanService := partnernetwork_service.NewPartnerPlanService(partnerPlanRepo, partnerNetworkRepo)
	appointmentPaymentService := appointment_service.NewAppointmentPaymentService(appointmentPaymentRepo, appointmentRepo, paymentMethodRepo)
	financeSvc := finance_service.NewFinanceService(financeTransactionRepo, financeCategoryRepo)
	appointmentPaymentService.SetFinanceUpserter(financeSvc)
	appointmentService.SetPaymentLister(appointmentPaymentRepo)
	appointmentService.SetFinanceUpserter(financeSvc)
	commissionSvc.SetFinanceCommissionUpserter(financeSvc)

	// Initialize message log service
	messageLogService := whatsapp_service.NewMessageLogService(messageLogRepo)

	// Initialize worker configuration (needed for Redis and AI Agent)
	workerCfg := config.NewWorkerConfig()

	// Initialize Redis client (for AI Agent conversation management)
	var redis *redisClient.RedisClient
	if workerCfg.RedisURL != "" {
		var err error
		redis, err = redisClient.NewRedisClient(workerCfg.RedisURL)
		if err != nil {
			logger.Printf("Warning: Failed to connect to Redis: %v (AI Agent will not be available)", err)
			redis = nil
		}
	}

	// Initialize WhatsApp service (if client is configured)
	var whatsappService whatsapp_service.WhatsAppService
	// aiAgentForShutdown is captured below when the AI agent is enabled
	// so the graceful shutdown path can drain pending LangFuse traces.
	// Stays nil when the agent isn't initialized; shutdown handles that.
	var aiAgentForShutdown agent.Service
	if whatsappClient != nil {
		whatsappService = whatsapp_service.NewWhatsAppService(whatsappClient, messageRepo, nil)

		// Wire up message log service for tracking message lifecycle
		whatsappService.SetMessageLogService(messageLogService)

		// Wire up instance→clinic resolver so SendTextMessage can log
		// outbound messages without needing clinicID at every call site.
		whatsappService.SetInstanceClinicResolver(whatsappConnRepo)

		// Wire up appointment status updater for WhatsApp confirmations/cancellations
		whatsappService.SetAppointmentUpdater(&appointmentStatusAdapter{appointmentService: appointmentService})
		whatsappService.SetAppointmentFinder(&appointmentFinderAdapter{appointmentRepo: appointmentRepo})
		whatsappService.SetSlotSuggester(appointmentService)
		whatsappService.SetAppointmentCreator(&whatsappAppointmentCreatorAdapter{appointmentService: appointmentService})
		logger.Println("WhatsApp appointment status updater and reschedule connected")

		// Wire up scheduling flow adapters for new appointment booking via WhatsApp
		schedulingIntentRepo := whatsapp_repository.NewSchedulingIntentRepository(db.GetDB())
		whatsappService.SetSchedulingIntentRepo(schedulingIntentRepo)
		whatsappService.SetPatientFinderCreator(&patientFinderCreatorAdapter{patientRepo: patientRepo})
		whatsappService.SetPatientEnsurer(&patientEnsurerAdapter{patientRepo: patientRepo, patientService: patientService})
		whatsappService.SetProfessionalLister(&professionalListerAdapter{professionalRepo: professionalRepo})
		whatsappService.SetSlotFinder(&slotFinderAdapter{appointmentService: appointmentService, professionalRepo: professionalRepo, clinicRepo: clinicRepo})
		whatsappService.SetNewAppointmentCreator(&newAppointmentCreatorAdapter{appointmentService: appointmentService})
		logger.Println("WhatsApp scheduling flow adapters connected")

		// Initialize AI Classifier for message classification (if enabled)
		if cfg.AIClassifierEnabled() {
			var apiKey string
			switch cfg.AIClassifierProvider() {
			case "gemini":
				apiKey = cfg.GetGeminiAPIKey()
			case "claude":
				apiKey = cfg.GetAnthropicAPIKey()
			case "openai":
				apiKey = cfg.GetOpenAIAPIKey()
			case "openrouter":
				apiKey = cfg.GetOpenRouterAPIKey()
			}

			if apiKey != "" {
				aiClassifier, err := whatsapp_classifier.NewClassifier(whatsapp_classifier.Config{
					Provider:                  cfg.AIClassifierProvider(),
					Model:                     cfg.AIClassifierModel(),
					APIKey:                    apiKey,
					Timeout:                   time.Duration(cfg.AIClassifierTimeout()) * time.Second,
					OpenRouterAPIURL:          cfg.GetOpenRouterAPIURL(),
					OpenRouterHTTPReferer:     cfg.GetOpenRouterHTTPReferer(),
					OpenRouterClassifierTitle: cfg.GetOpenRouterClassifierTitle(),
				})
				if err != nil {
					logger.Printf("Warning: Failed to initialize AI classifier: %v", err)
				} else {
					whatsappService.SetClassifier(aiClassifier, cfg.AIClassifierConfidenceThreshold())
					logger.Printf("AI Classifier initialized: provider=%s model=%s threshold=%.2f",
						cfg.AIClassifierProvider(), cfg.AIClassifierModel(), cfg.AIClassifierConfidenceThreshold())
				}
			} else {
				logger.Printf("Warning: AI_CLASSIFIER_ENABLED=true but no API key for provider %s",
					cfg.AIClassifierProvider())
			}
		}

		// Initialize Audio Transcriber
		if cfg.GetOpenRouterAPIKey() != "" {
			transcriberModel := workerCfg.AIAgentModel // Use default multimodal model
			if transcriberModel == "" {
				transcriberModel = "google/gemini-2.0-flash-001"
			}
			audioTranscriber := whatsapp_transcriber.NewOpenRouterTranscriber(
				cfg.GetOpenRouterAPIURL(),
				cfg.GetOpenRouterAPIKey(),
				transcriberModel,
				cfg.GetOpenRouterHTTPReferer(),
				cfg.GetOpenRouterTranscriberTitle(),
			)
			whatsappService.SetAudioTranscriber(audioTranscriber)
			logger.Println("Audio Transcriber initialized with OpenRouter")
		}

		// Initialize AI Agent for dynamic WhatsApp conversations (if enabled)
		if redis != nil && workerCfg.AIAgentEnabled {
			clinicService := clinic_service.NewClinicService(clinicRepo, cfg.GetFrontendURL())
			// Built once and shared between the takeover tracker (which the
			// WhatsApp service reads to silence the AI when a human assumes)
			// and the tools.HumanTakeoverMarker (which complete_handoff_to_human
			// writes to trigger that same silence after gathering context).
			pwcRepo := patient_repository.NewPatientWhatsAppConnectionRepository(db.GetDB())
			takeoverAdapter := &humanTakeoverTrackerAdapter{
				patientRepo:  patientRepo,
				connRepo:     pwcRepo,
				whatsappRepo: whatsappConnRepo,
			}
			// Enrollment service is wired here (instead of further down) because
			// the AI agent's enroll_patient_in_class tool needs it. The full
			// enrollment handler stack is still constructed in the regular
			// repo/service block below; this block only creates the same
			// service/repo so the AI sees them. Both blocks point at the same
			// underlying repository instance — there is no double-wiring of
			// state, just an earlier declaration.
			aiEnrollmentRepo := enrollment_repository.NewEnrollmentRepository(db.GetDB())
			aiEnrollmentSvc := enrollment_service.NewEnrollmentService(aiEnrollmentRepo, sessionPlanRepo)
			aiEnrollmentSvc.SetSlotProvider(sessionPlanSlotRepo)
			aiEnrollmentSvc.SetPartnerPlanProvider(partnerPlanRepo)
			// Group attendance repo is also created here so the
			// confirm_class_attendance tool can write a "confirmed" pre-class
			// row. Same reasoning as the enrollment block above — declared
			// early purely so the AI block sees it; the post-class attendance
			// flow further down still creates its own group_attendance_repo
			// with the same backing table.
			aiGroupAttendanceRepo := attendance_repository.NewGroupAttendanceRepository(db.GetDB())
			aiAgentService, conversationMgr, _, err := initializeAIAgent(
				cfg,
				db,
				redis,
				professionalService,
				appointmentService,
				appointmentRepo,
				patientService,
				clinicService,
				procedureService,
				specialtyRepo,
				patientRepo,
				bookingSvc,
				clinicRepo,
				professionalRepo,
				professionalProcedureRepo,
				systemLogSvc,
				usageMetricSvc,
				aiTokenUsageSvc,
				takeoverAdapter,
				sessionPlanRepo,
				sessionPlanSlotRepo,
				aiEnrollmentRepo,
				aiEnrollmentSvc,
				partnerNetworkRepo,
				aiGroupAttendanceRepo,
			)
			if err != nil {
				logger.Printf("Warning: Failed to initialize AI Agent: %v", err)
			} else if aiAgentService != nil {
				aiAgentForShutdown = aiAgentService
				aiAgentService.SetPromptHistoryTurns(workerCfg.ConversationPromptHistoryTurns)
				aiAgentService.SetTemperatures(workerCfg.AIAgentTemperature, workerCfg.AIAgentHandoffTemperature)
				// Pre-fetch context: next pending appointment + active
				// recurring plans (individual + group). Lets the LLM
				// interpret short patient replies against real state.
				if setter, ok := aiAgentService.(interface {
					SetPatientContextFetcher(agent.PatientContextFetcher)
				}); ok {
					setter.SetPatientContextFetcher(&aiAgentPatientContextFetcherAdapter{
						appointmentRepo:  appointmentRepo,
						professionalRepo: professionalRepo,
						sessionPlanRepo:  sessionPlanRepo,
						slotRepo:         sessionPlanSlotRepo,
						enrollmentRepo:   aiEnrollmentRepo,
					})
				}
				whatsappService.SetAIAgentService(aiAgentService)
				if conversationMgr != nil {
					whatsappService.SetConversationManager(conversationMgr)
				}
				whatsappService.SetClinicRepo(clinicRepo)
				whatsappService.SetPlanGateChecker(whatsapp_service.NewPlanGateChecker(subscriptionRepo, planRepo))
				whatsappService.SetScheduledMessageRepo(scheduledMsgRepo)
				whatsappService.SetPatientAIPreferenceChecker(&patientAIPreferenceAdapter{
					patientRepo: patientRepo,
					connRepo:    pwcRepo,
				})
				whatsappService.SetHumanTakeoverTracker(takeoverAdapter)
				whatsappService.SetSystemLogService(systemLogSvc)
				whatsappService.SetDefaultDelayBetweenPatients(workerCfg.MinDelayBetweenPatientsSeconds)
				whatsappService.SetDebounceDelay(time.Duration(workerCfg.ResponseDebounceSeconds) * time.Second)
				logger.Printf("AI Agent service connected to WhatsApp (response debounce: %ds)", workerCfg.ResponseDebounceSeconds)
			}
		} else if redis == nil {
			logger.Println("AI Agent: Not initialized (requires Redis configuration)")
		} else if !workerCfg.AIAgentEnabled {
			logger.Println("AI Agent: Disabled (set AI_AGENT_ENABLED=true to enable)")
		}
	}

	// Initialize Plan service and handler
	planService := plan_service.NewPlanService(planRepo, offerRepo)
	planHandler := plan_handler.NewPlanHandler(planService)
	offerHandler := plan_handler.NewOfferHandler(planService)

	// Initialize Payment service (if Asaas client is configured)
	var paymentService payment_service.PaymentService
	if asaasClient != nil {
		paymentService = payment_service.NewPaymentService(asaasClient, customerRepo, subscriptionRepo, paymentRepo, clinicRepo, accountRepo, planService)

		// Inject payment service into auth service for subscription creation during registration
		authService.SetPaymentService(paymentService)
	}

	// Initialize Calendar service (if Google client is configured)
	var calendarService calendar_service.CalendarService
	if googleCalendarClient != nil {
		calendarService = calendar_service.NewCalendarService(googleCalendarClient, calendarIntegrationRepo, calendarSyncRepo, clinicRepo)
		// Wire calendar service into appointment service for Google Calendar sync
		appointmentService.SetCalendarService(calendarService)
		logger.Println("Google Calendar sync enabled for appointments")
	}

	// Initialize Session Plan service
	sessionPlanService := sessionplan_service.NewSessionPlanService(sessionPlanRepo, appointmentRepo)

	// Initialize Invite service
	inviteSvc := invite_service.NewInviteService(
		inviteRepo,
		userRepo,
		clinicRepo,
		professionalRepo,
		professionalService,
		clinicMemberRepo,
		emailSvc,
		jwtService,
		googleAdapter, // may be nil if Google OAuth not configured
		cfg.GetFrontendURL(),
	)

	// Initialize User service
	userSvc := user_service.NewUserService(userRepo, professionalRepo)

	// Initialize Patient Record service
	patientRecordSvc := patientrecord_service.NewPatientRecordService(patientRecordRepo, appointmentRepo)
	authService.SetTemplateSeedService(patientRecordSvc)

	// Initialize Dashboard service
	dashboardSvc := dashboard_service.NewDashboardService(dashboardRepo, clinicRepo)

	// Initialize Analytics service
	analyticsSvc := analytics_service.NewAnalyticsService(analyticsRepo, clinicRepo)

	// Notification service already initialized earlier (for AI Agent)
	// notificationSvc already exists

	// Initialize Waitlist service (if WhatsApp and queue are available)
	var waitlistSvc waitlist_service.WaitlistService
	// Note: We'll fully initialize this after the queue is set up

	// Initialize Session Plan tasks worker (runs every hour)
	// No longer generates appointments — occurrences are calculated via RRULE on demand.
	// Tasks can be registered for notifications, billing reminders, etc.
	// Started further below after tasks are registered, so RegisterTask calls
	// are observed before the first tick.
	sessionPlanWorker := worker.NewSessionPlanTasksWorker(1 * time.Hour)

	// Initialize Payment Timeout worker (cancels awaiting_payment appointments)
	paymentTimeoutWorker := worker.NewPaymentTimeoutWorker(db.GetDB(), cfg.GetBookingPaymentTimeoutMinutes())
	go paymentTimeoutWorker.Start()

	// Initialize RabbitMQ queue (if configured)
	var messageQueue queue.Queue
	var reminderWorker *worker.ReminderWorker
	var messageSenderWorker *worker.MessageSenderWorker
	var scheduledMessageWorker *worker.ScheduledMessageWorker

	if workerCfg.RabbitMQURL != "" {
		messageQueue = queue.NewRabbitMQ(queue.RabbitMQConfig{
			URL:           workerCfg.RabbitMQURL,
			QueueName:     workerCfg.RabbitMQQueue,
			DLQName:       workerCfg.RabbitMQDLQ,
			PrefetchCount: 1,
		})

		// Connect to RabbitMQ - REQUIRED for application to start
		if err := messageQueue.Connect(context.Background()); err != nil {
			logger.Fatalf("FATAL: Failed to connect to RabbitMQ: %v\nRabbitMQ is required for reminder and message workers. Please ensure RabbitMQ is running.", err)
		}

		logger.Println("RabbitMQ connected successfully")

		// Initialize Redis rate limiter
		rateLimiter, err := redisClient.NewRateLimiter(workerCfg.RedisURL)
		if err != nil {
			logger.Printf("Warning: Failed to initialize Redis rate limiter: %v (rate limiting will use fallback)", err)
			rateLimiter = nil
		}

		// Wire message queue into WhatsApp service for segmented message delivery
		if whatsappService != nil {
			whatsappService.SetMessageQueue(messageQueue)
			logger.Println("WhatsApp message queue configured for segmented delivery")
			if rateLimiter != nil {
				whatsappService.SetSentMessageCache(rateLimiter)
				logger.Println("WhatsApp sent-message cache connected (used for human takeover detection)")
			}
		}

		// Initialize and start Reminder Worker (only if WhatsApp is configured)
		if whatsappService != nil && workerCfg.ReminderWorkerEnabled {
			reminderWorker = worker.NewReminderWorker(workerCfg, appointmentRepo, messageQueue)
			reminderWorker.SetSubscriptionChecker(&clinicSubscriptionCheckerAdapter{clinicRepo: clinicRepo})
			reminderWorker.SetSystemLogService(systemLogSvc)
			go reminderWorker.Start()

			// Group class reminders run on a parallel worker because group
			// plans never materialize as appointments — the appointment
			// reminder worker (above) can't pick them up. Same ticker, same
			// thresholds, dedicated dedup table. Repos are local to this
			// block (the main enrollment/attendance instances are created
			// further down) — both point at the same DB and share no state,
			// so an extra wrapper here is harmless.
			classReminderEnrollmentRepo := enrollment_repository.NewEnrollmentRepository(db.GetDB())
			classReminderGroupAttendanceRepo := attendance_repository.NewGroupAttendanceRepository(db.GetDB())
			groupClassReminderLogRepo := sessionplan_repository.NewGroupClassReminderLogRepository(db.GetDB())
			groupClassReminderSource := sessionplan_service.NewGroupClassReminderSource(
				clinicRepo,
				sessionPlanRepo,
				sessionPlanSlotRepo,
				classReminderEnrollmentRepo,
				patientRepo,
				whatsappConnRepo,
			)
			groupClassReminderWorker := worker.NewGroupClassReminderWorker(
				workerCfg,
				groupClassReminderSource,
				groupClassReminderLogRepo,
				&groupClassAttendanceCheckerAdapter{repo: classReminderGroupAttendanceRepo},
				messageQueue,
			)
			groupClassReminderWorker.SetSubscriptionChecker(&clinicSubscriptionCheckerAdapter{clinicRepo: clinicRepo})
			groupClassReminderWorker.SetSystemLogService(systemLogSvc)
			go groupClassReminderWorker.Start()

			subscriptionChecker := &clinicSubscriptionCheckerAdapter{clinicRepo: clinicRepo}

			// Initialize and start Message Sender Worker
			messageSenderWorker = worker.NewMessageSenderWorker(workerCfg, messageQueue, whatsappService, rateLimiter, clinicRepo)
			messageSenderWorker.SetSubscriptionChecker(subscriptionChecker)
			go messageSenderWorker.Start()

			// Initialize and start Scheduled Message Worker (processes messages scheduled for outside business hours)
			// Uses WHATSAPP_MIN_DELAY_BETWEEN_PATIENTS_SECONDS as poll interval
			scheduledMessageWorker = worker.NewScheduledMessageWorker(
				workerCfg,
				scheduledMsgRepo,
				clinicRepo,
				whatsappService,
			)
			scheduledMessageWorker.SetSubscriptionChecker(subscriptionChecker)
			go scheduledMessageWorker.Start()

			// Initialize and start Timeout Worker (marks no_response when patient doesn't confirm)
			if workerCfg.TimeoutWorkerEnabled {
				timeoutWorker := worker.NewTimeoutWorker(workerCfg, appointmentRepo)
				timeoutWorker.SetSubscriptionChecker(subscriptionChecker)
				go timeoutWorker.Start()
			}

			logger.Println("Reminder, Message Sender, and Timeout workers started")

			// Register monthly payment reminder task on the session plan tasks
			// worker. The task is idempotent (checks session_plan_payments and
			// session_plan_payment_reminders before dispatch), so running it
			// hourly is safe even if the scheduler double-fires. Requires the
			// WhatsApp message queue for the WhatsApp channel and the
			// EmailService for the email channel; either can be disabled via
			// env kill switch or per-clinic settings.
			paymentReminderRepo := sessionplan_repository.NewPaymentReminderRepository(db.GetDB())
			paymentReminderTask := worker.NewPaymentReminderTask(
				workerCfg,
				worker.NewGormClinicSource(db.GetDB()),
				worker.NewGormCandidateSource(db.GetDB()),
				worker.NewGormTracker(db.GetDB(), paymentReminderRepo),
				messageQueue,
				worker.NewPaymentReminderEmailAdapter(emailSvc),
			)
			paymentReminderTask.SetSubscriptionChecker(&clinicSubscriptionCheckerAdapter{clinicRepo: clinicRepo})
			paymentReminderTask.SetSystemLogService(systemLogSvc)
			sessionPlanWorker.RegisterTask("payment_reminder", paymentReminderTask.Run)
			logger.Println("Payment reminder task registered on session plan worker")
		}

		// Initialize Waitlist service (requires WhatsApp and queue)
		if whatsappService != nil {
			waitlistSvc = waitlist_service.NewWaitlistService(
				waitlistRepo,
				&clinicInfoAdapter{clinicRepo: clinicRepo},
				&professionalInfoAdapter{professionalRepo: professionalRepo},
				&appointmentCreatorAdapter{appointmentService: appointmentService},
				messageQueue,
				clinicRepo,
				whatsappConnRepo,
			)

			// Wire waitlist service into appointment service
			appointmentService.SetWaitlistProcessor(waitlistSvc)

			// Wire waitlist service into whatsapp service for response handling
			whatsappService.SetWaitlistHandler(waitlistSvc)

			logger.Println("Waitlist (encaixe) service initialized")
		}
	} else {
		logger.Fatalf("FATAL: RabbitMQ URL not configured in environment variables. Please set RABBITMQ_URL.")
	}

	// Start the session plan tasks worker after all tasks are registered.
	// When no task was registered (e.g. misconfiguration upstream) the worker
	// blocks on its stop channel and is effectively inert — safe to always run.
	go sessionPlanWorker.Start()

	// Wire patient risk calculator into appointment service (always enabled)
	appointmentService.SetPatientRiskCalculator(&patientRiskAdapter{patientService: patientService})
	logger.Println("Patient risk calculator connected to appointment service")

	// Wire commission calculator into appointment service
	appointmentService.SetCommissionCalculator(commissionSvc)
	logger.Println("Commission calculator connected to appointment service")

	// Wire session plan handler into appointment service for sessions_done tracking
	appointmentService.SetSessionPlanHandler(sessionPlanService)
	logger.Println("Session plan handler connected to appointment service")

	// Initialize middleware
	authMiddleware := auth_middleware.NewAuthMiddleware(jwtService)
	tenantMiddleware := auth_middleware.NewTenantMiddleware(clinicRepo)

	// Initialize handlers
	healthHandler := health_handler.NewHealthHandler(db)
	authHandler := auth_handler.NewAuthHandler(
		authService,
		auth_handler.WithAuditService(auditSvc),
		auth_handler.WithSystemLogService(systemLogSvc),
	)
	professionalHandler := professional_handler.NewProfessionalHandler(professionalService, inviteSvc, auditSvc)
	professionalProcedureHandler := professional_handler.NewProfessionalProcedureHandler(professionalProcedureService, auditSvc)
	commissionHandler := commission_handler.NewCommissionHandler(commissionSvc, auditSvc)
	patientHandler := patient_handler.NewPatientHandler(patientService, auditSvc)
	patientWhatsAppConnRepo := patient_repository.NewPatientWhatsAppConnectionRepository(db.GetDB())
	patientWhatsAppConnService := patient_service.NewPatientWhatsAppConnectionService(patientWhatsAppConnRepo, patientRepo, whatsappConnRepo, clinicRepo)
	patientWhatsAppConnHandler := patient_handler.NewPatientWhatsAppConnectionHandler(patientWhatsAppConnService, auditSvc)
	// Initialize Enrollment service and handler
	enrollmentRepo := enrollment_repository.NewEnrollmentRepository(db.GetDB())
	enrollmentSvc := enrollment_service.NewEnrollmentService(enrollmentRepo, sessionPlanRepo)
	enrollmentSvc.SetSlotProvider(sessionPlanSlotRepo)
	enrollmentSvc.SetPartnerPlanProvider(partnerPlanRepo)
	enrollmentHdlr := enrollment_handler.NewEnrollmentHandler(enrollmentSvc)

	// Initialize Attendance service and handler
	attendanceRepo := attendance_repository.NewAttendanceRepository(db.GetDB())
	attendanceSvc := attendance_service.NewAttendanceService(attendanceRepo, appointmentRepo, sessionPlanRepo, enrollmentRepo)
	attendanceSvc.SetPaymentRepo(appointmentPaymentRepo)
	attendanceHdlr := attendance_handler.NewAttendanceHandler(attendanceSvc)

	// Initialize Group Attendance service and handler
	groupAttendanceRepo := attendance_repository.NewGroupAttendanceRepository(db.GetDB())
	groupAttendanceSvc := attendance_service.NewGroupAttendanceService(groupAttendanceRepo, sessionPlanRepo, enrollmentRepo)
	groupAttendanceHdlr := attendance_handler.NewGroupAttendanceHandler(groupAttendanceSvc)

	// Wire commission calculator and session professional repo into group attendance
	groupSessionProfRepoForAttendance := attendance_repository.NewGroupSessionProfessionalRepository(db.GetDB())
	groupAttendanceSvc.SetCommissionCalculator(commissionSvc)
	groupAttendanceSvc.SetSessionProfessionalRepo(groupSessionProfRepoForAttendance)

	// Initialize Group Session Professional service and handler
	groupSessionProfRepo := attendance_repository.NewGroupSessionProfessionalRepository(db.GetDB())
	groupSessionProfSvc := attendance_service.NewGroupSessionProfessionalService(groupSessionProfRepo, sessionPlanRepo)
	groupSessionProfHdlr := attendance_handler.NewGroupSessionProfessionalHandler(groupSessionProfSvc)

	appointmentHandler := appointment_handler.NewAppointmentHandler(
		appointmentService,
		appointment_handler.WithAttendanceRecorder(attendanceSvc),
		appointment_handler.WithAuditService(auditSvc),
	)
	procedureHandler := procedure_handler.NewProcedureHandler(procedureService)
	paymentMethodHandler := paymentmethod_handler.NewPaymentMethodHandler(paymentMethodService, auditSvc)
	specialtyHandler := specialty_handler.NewSpecialtyHandler(specialtyService, auditSvc)
	partnerNetworkHandler := partnernetwork_handler.NewPartnerNetworkHandler(partnerNetworkService, auditSvc)
	partnerPlanHandler := partnernetwork_handler.NewPartnerPlanHandler(partnerPlanService, auditSvc)
	appointmentPaymentHandler := appointment_handler.NewAppointmentPaymentHandler(appointmentPaymentService, auditSvc)
	financeHandler := finance_handler.NewFinanceHandler(financeSvc, auditSvc)

	// Sales Package (vendas avulsas com saldo flutuante)
	salesPackageRepo := salespackage_repository.NewSalesPackageRepository(db.GetDB())
	salesPackageSvc := salespackage_service.NewSalesPackageService(
		salesPackageRepo,
		salespackage_service.NewPaymentMethodLookup(paymentMethodRepo),
		salespackage_service.NewFinanceHook(financeSvc),
		salespackage_service.NewPatientLookup(patientRepo),
		salespackage_service.NewAuditLogger(auditSvc),
		salespackage_service.NewSystemLogger(systemLogSvc),
	)
	salesPackageHandler := salespackage_handler.NewSalesPackageHandler(salesPackageSvc)

	// Wire FIFO consumption + balance lookup into the appointment service so
	// creating/canceling an appointment automatically charges or releases the
	// patient's package balance.
	appointmentService.SetSalesPackagePicker(salesPackageSvc)

	// Public receipt (share link) — token issuance is authenticated,
	// reading is public. Uses narrow interfaces so tests don't touch the
	// full repositories.
	appointmentReceiptService := appointment_service.NewAppointmentReceiptService(
		appointmentReceiptTokenRepo,
		appointmentRepo,
		appointmentPaymentRepo,
		clinicRepo,
		cfg.GetFrontendURL(),
	)
	appointmentReceiptHandler := appointment_handler.NewAppointmentReceiptHandler(appointmentReceiptService)
	publicReceiptHandler := public_handler.NewPublicReceiptHandler(appointmentReceiptService)

	// Initialize WhatsApp handler (if service is configured)
	var whatsappHandler whatsapp_handler.WhatsAppHandler
	if whatsappService != nil {
		whatsappHandler = whatsapp_handler.NewWhatsAppHandler(whatsappService, clinicRepo, whatsappConnRepo, cfg.GetWhatsAppWebhookSecret(), cfg.GetWhatsAppProvider())
	}

	// Initialize Payment handler (if service is configured)
	var paymentHandler payment_handler.PaymentHandler
	if paymentService != nil {
		paymentHandler = payment_handler.NewPaymentHandler(paymentService, cfg.GetAsaasWebhookSecret())
	}

	// Initialize Marketplace service and handler (if Asaas client and encryption key are configured)
	var marketplaceHdlr marketplace_handler.MarketplaceHandler
	if asaasClient != nil && cfg.GetEncryptionKey() != "" {
		subaccountRepo := marketplace_repository.NewSubaccountRepository(db.GetDB())
		splitConfigRepo := marketplace_repository.NewSplitConfigRepository(db.GetDB())
		clinicPaymentRepo := marketplace_repository.NewClinicPaymentRepository(db.GetDB())
		mktClinicRepo := marketplace_service.NewClinicRepoAdapter(db.GetDB())
		mktAptRepo := marketplace_service.NewAppointmentRepoAdapter(db.GetDB())
		mktAptPaymentRepo := marketplace_service.NewAppointmentPaymentRepoAdapter(db.GetDB())

		mktService := marketplace_service.NewMarketplaceService(
			asaasClient,
			subaccountRepo,
			splitConfigRepo,
			clinicPaymentRepo,
			mktClinicRepo,
			mktAptRepo,
			mktAptPaymentRepo,
			marketplace_service.MarketplaceServiceConfig{
				EncryptionKey:         cfg.GetEncryptionKey(),
				AsenoWalletID:         cfg.GetAsenoWalletID(),
				DefaultFeePercent:     cfg.GetMarketplaceDefaultFeePercent(),
				PaymentTimeoutMinutes: cfg.GetBookingPaymentTimeoutMinutes(),
				WebhookBaseURL:        cfg.GetMarketplaceWebhookURL(),
			},
		)
		marketplaceHdlr = marketplace_handler.NewMarketplaceHandler(mktService, cfg.GetMarketplaceWebhookSecret())
		logger.Println("Marketplace service initialized")
	}

	// Initialize Calendar handler (if service is configured)
	var calendarHandler calendar_handler.CalendarHandler
	if calendarService != nil {
		calendarHandler = calendar_handler.NewCalendarHandler(calendarService, cfg.GetFrontendURL())
	}

	// Initialize Session Plan handler with calendar facade and materialize service
	calendarFacade := sessionplan_service.NewCalendarFacade(sessionPlanRepo, appointmentRepo, enrollmentRepo)
	calendarFacade.SetSlotRepo(sessionPlanSlotRepo)
	materializeSvc := sessionplan_service.NewMaterializeService(sessionPlanRepo, appointmentRepo, professionalRepo)
	sessionPlanHandler := sessionplan_handler.NewSessionPlanHandler(sessionPlanService, calendarFacade, materializeSvc, auditSvc)
	sessionPlanSlotService := sessionplan_service.NewSessionPlanSlotService(sessionPlanRepo, sessionPlanSlotRepo)
	sessionPlanSlotHandler := sessionplan_handler.NewSessionPlanSlotHandler(sessionPlanSlotService, auditSvc)

	// Initialize Plan Payment service and handler
	planPaymentRepo := sessionplan_repository.NewPlanPaymentRepository(db.GetDB())
	planPaymentSvc := sessionplan_service.NewPlanPaymentService(planPaymentRepo, sessionPlanRepo, appointmentPaymentRepo)
	planPaymentSvc.SetFinancePlanUpserter(financeSvc)
	sessionPlanService.SetPaymentLister(planPaymentRepo)
	sessionPlanService.SetFinanceUpserter(financeSvc)
	planPaymentHdlr := sessionplan_handler.NewPlanPaymentHandler(planPaymentSvc)

	// Wire plan payment repo into group attendance for guest payments
	groupAttendanceSvc.SetPlanPaymentRepo(planPaymentRepo)

	// Initialize Invite handler
	inviteHdlr := invite_handler.NewInviteHandler(inviteSvc, auditSvc)

	// Initialize User handler
	userHdlr := user_handler.NewUserHandler(userSvc, auditSvc)

	// Initialize Patient Record handler
	patientRecordHdlr := patientrecord_handler.NewPatientRecordHandler(patientRecordSvc)

	// Initialize Dashboard handler
	dashboardHdlr := dashboard_handler.NewDashboardHandler(dashboardSvc)

	// Initialize Analytics handler
	analyticsHdlr := analytics_handler.NewAnalyticsHandler(analyticsSvc)

	// Initialize Notification handler
	notificationHdlr := notification_handler.NewNotificationHandler(notificationSvc)

	// Connect SSE broadcaster to notification service
	notificationSvc.SetBroadcaster(notificationHdlr.GetHub())

	// Connect JWT service to notification handler for SSE token generation
	notificationHdlr.SetJWTService(jwtService)

	// Initialize Onboarding service and handler
	onboardingSvc := clinic_service.NewOnboardingService(db.GetDB(), clinicRepo)
	onboardingHdlr := clinic_handler.NewOnboardingHandler(onboardingSvc)

	// Initialize Clinic service and handler
	clinicSvc := clinic_service.NewClinicService(clinicRepo, cfg.GetFrontendURL())
	clinicHdlr := clinic_handler.NewClinicHandler(clinicSvc, auditSvc)

	// Initialize Waitlist handler
	var waitlistHdlr waitlist_handler.WaitlistHandler
	if waitlistSvc != nil {
		waitlistHdlr = waitlist_handler.NewWaitlistHandler(waitlistSvc)
	}

	// Initialize Settings handler (admin only - dev clinics)
	settingsHdlr := settings_handler.NewSettingsHandler(settingsSvc, planService)

	// Initialize CRM Lead service and handler
	var crmProvider lead_crm.CRMProvider
	if cfg.GetNotionAPIKey() != "" && cfg.GetNotionCRMDatabaseID() != "" {
		crmProvider = lead_notion.NewNotionAdapter(cfg.GetNotionAPIKey(), cfg.GetNotionCRMDatabaseID())
		logger.Println("Notion CRM provider initialized")
	}
	leadSvc := lead_service.NewLeadService(leadRepo, crmProvider)
	leadHdlr := lead_handler.NewLeadHandler(leadSvc, accountRepo, db.GetDB())
	leadIscaHdlr := lead_handler.NewLeadIscaHandler(leadRepo, emailSvc, "sheets")

	// Initialize Sprint 6 handlers (admin only)
	auditHdlr := audit_handler.NewAuditHandler(auditSvc)
	systemLogHdlr := systemlog_handler.NewSystemLogHandler(systemLogSvc)
	featureFlagHdlr := featureflag_handler.NewFeatureFlagHandler(featureFlagSvc)
	usageMetricHdlr := usagemetric_handler.NewUsageMetricHandler(usageMetricSvc)
	aiTokenUsageHdlr := aitokenusage_handler.NewHandler(aiTokenUsageSvc)

	// Initialize Public handler (no auth required - landing page config)
	publicHdlr := public_handler.NewPublicHandler(settingsSvc, planService, emailSvc)

	// Booking service already initialized earlier (for AI Agent)
	// bookingSvc already exists
	bookingHdlr := booking_handler.NewBookingHandler(bookingSvc)

	// Initialize Portal service, middleware, and handler
	portalSvc := portalaccount_service.NewPortalService(
		portalAccountRepo,
		clinicRepo,
		patientRepo,
		appointmentRepo,
		professionalRepo,
		procedureRepo,
		notificationSvc,
		emailSvc,
		cfg.GetPortalJWTSecret(),
		cfg.GetFrontendURL(),
	)
	portalMw := portalaccount_middleware.NewPortalMiddleware(portalSvc)
	portalHdlr := portalaccount_handler.NewPortalHandler(portalSvc, portalMw)

	// Setup router
	router := setupRouter(
		cfg,
		healthHandler,
		authHandler,
		professionalHandler,
		professionalProcedureHandler,
		patientHandler,
		appointmentHandler,
		procedureHandler,
		whatsappHandler,
		paymentHandler,
		calendarHandler,
		sessionPlanHandler,
		sessionPlanSlotHandler,
		inviteHdlr,
		userHdlr,
		patientRecordHdlr,
		dashboardHdlr,
		analyticsHdlr,
		onboardingHdlr,
		notificationHdlr,
		waitlistHdlr,
		clinicHdlr,
		settingsHdlr,
		publicHdlr,
		bookingHdlr,
		planHandler,
		offerHandler,
		auditHdlr,
		systemLogHdlr,
		featureFlagHdlr,
		usageMetricHdlr,
		aiTokenUsageHdlr,
		leadHdlr,
		leadIscaHdlr,
		paymentMethodHandler,
		appointmentPaymentHandler,
		appointmentReceiptHandler,
		publicReceiptHandler,
		specialtyHandler,
		partnerNetworkHandler,
		partnerPlanHandler,
		commissionHandler,
		financeHandler,
		salesPackageHandler,
		portalHdlr,
		marketplaceHdlr,
		attendanceHdlr,
		groupAttendanceHdlr,
		groupSessionProfHdlr,
		enrollmentHdlr,
		planPaymentHdlr,
		patientWhatsAppConnHandler,
		authMiddleware,
		tenantMiddleware,
		systemLogSvc,
	)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Printf("Starting server on port %s", cfg.Port)
		logger.Printf("Environment: %s", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Println("Shutting down server...")

	// Stop workers
	sessionPlanWorker.Stop()
	if reminderWorker != nil {
		reminderWorker.Stop()
	}
	if messageSenderWorker != nil {
		messageSenderWorker.Stop()
	}
	if scheduledMessageWorker != nil {
		scheduledMessageWorker.Stop()
	}
	if messageQueue != nil {
		messageQueue.Close()
	}

	// Drain LangFuse / observability traces. The concrete agent service
	// exposes Flush() as an unexported capability — type-asserted here
	// so we don't widen the public Service interface (which would break
	// every test stub that implements Service).
	if aiAgentForShutdown != nil {
		if flusher, ok := aiAgentForShutdown.(interface{ Flush() }); ok {
			flusher.Flush()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Println("Server exited")
}

func setupRouter(
	cfg *config.Config,
	healthHandler health_handler.HealthHandler,
	authHandler auth_handler.AuthHandler,
	professionalHandler professional_handler.ProfessionalHandler,
	professionalProcedureHandler professional_handler.ProfessionalProcedureHandler,
	patientHandler patient_handler.PatientHandler,
	appointmentHandler appointment_handler.AppointmentHandler,
	procedureHandler *procedure_handler.ProcedureHandler,
	whatsappHandler whatsapp_handler.WhatsAppHandler,
	paymentHandler payment_handler.PaymentHandler,
	calendarHandler calendar_handler.CalendarHandler,
	sessionPlanHandler sessionplan_handler.SessionPlanHandler,
	sessionPlanSlotHandler *sessionplan_handler.SessionPlanSlotHandler,
	inviteHandler invite_handler.InviteHandler,
	userHandler user_handler.UserHandler,
	patientRecordHandler patientrecord_handler.PatientRecordHandler,
	dashboardHandler *dashboard_handler.DashboardHandler,
	analyticsHandler analytics_handler.AnalyticsHandler,
	onboardingHandler clinic_handler.OnboardingHandler,
	notificationHandler notification_handler.NotificationHandler,
	waitlistHandler waitlist_handler.WaitlistHandler,
	clinicHandler clinic_handler.ClinicHandler,
	settingsHandler settings_handler.SettingsHandler,
	publicHandler public_handler.PublicHandler,
	bookingHandler booking_handler.BookingHandler,
	planHandler *plan_handler.PlanHandler,
	offerHandler *plan_handler.OfferHandler,
	auditHandler audit_handler.AuditHandler,
	systemLogHandler systemlog_handler.SystemLogHandler,
	featureFlagHandler featureflag_handler.FeatureFlagHandler,
	usageMetricHandler usagemetric_handler.UsageMetricHandler,
	aiTokenUsageHandler aitokenusage_handler.Handler,
	leadHandler lead_handler.LeadHandler,
	leadIscaHandler lead_handler.LeadIscaHandler,
	paymentMethodHandler *paymentmethod_handler.PaymentMethodHandler,
	appointmentPaymentHandler appointment_handler.AppointmentPaymentHandler,
	appointmentReceiptHandler appointment_handler.AppointmentReceiptHandler,
	publicReceiptHandler public_handler.PublicReceiptHandler,
	specialtyHandler *specialty_handler.SpecialtyHandler,
	partnerNetworkHandler *partnernetwork_handler.PartnerNetworkHandler,
	partnerPlanHandler *partnernetwork_handler.PartnerPlanHandler,
	commissionHandler commission_handler.CommissionHandler,
	financeHandler finance_handler.FinanceHandler,
	salesPackageHandler salespackage_handler.SalesPackageHandler,
	portalHandler *portalaccount_handler.PortalHandler,
	marketplaceHandler marketplace_handler.MarketplaceHandler,
	attendanceHandler attendance_handler.AttendanceHandler,
	groupAttendanceHandler attendance_handler.GroupAttendanceHandler,
	groupSessionProfHandler attendance_handler.GroupSessionProfessionalHandler,
	enrollmentHandler enrollment_handler.EnrollmentHandler,
	planPaymentHandler sessionplan_handler.PlanPaymentHandler,
	patientWhatsAppConnHandler patient_handler.PatientWhatsAppConnectionHandler,
	authMiddleware auth_middleware.AuthMiddleware,
	tenantMiddleware auth_middleware.TenantMiddleware,
	systemLogSvc systemlog_service.SystemLogService,
) *gin.Engine {
	router := gin.New()

	// Middlewares
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// CORS configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check routes (no auth required)
	healthHandler.RegisterRoutes(router)

	// API v1 routes
	v1 := router.Group("/api/v1")
	v1.Use(systemlog_middleware.ErrorCapture(systemLogSvc))
	{
		// Public auth routes (no auth required)
		authHandler.RegisterRoutes(v1)

		// Public invite routes (no auth required)
		inviteHandler.RegisterPublicRoutes(v1)

		// Public config routes (no auth required - landing page)
		publicHandler.RegisterRoutes(v1)

		// Public appointment receipt (share link) — no auth required
		publicReceiptHandler.RegisterRoutes(v1)

		// Public lead isca routes (no auth required - lead capture)
		leadIscaHandler.RegisterRoutes(v1)

		// Public booking routes (no auth required - online booking)
		bookingHandler.RegisterRoutes(v1)

		// Portal routes (public + portal-auth protected)
		portalHandler.RegisterRoutes(v1)

		// Public plan routes (no auth required)
		planHandler.RegisterPublicRoutes(v1)

		// Public SSE stream route (uses token in query param for auth)
		notificationHandler.RegisterPublicRoutes(v1)

		// Auth-only routes (auth required, NO subscription check)
		// These routes MUST work even when subscription is expired
		// so users can view their account, pay, and manage billing
		authOnly := v1.Group("")
		authOnly.Use(authMiddleware.Authenticate())
		{
			// Auth routes that must work with expired subscription
			auth := authOnly.Group("/auth")
			{
				auth.POST("/logout", func(c *gin.Context) {
					authHandler.Logout(c)
				})
				auth.GET("/me", func(c *gin.Context) {
					authHandler.GetCurrentUser(c)
				})
				auth.POST("/switch-clinic", func(c *gin.Context) {
					authHandler.SwitchClinic(c)
				})
				auth.POST("/change-password", func(c *gin.Context) {
					authHandler.ChangePassword(c)
				})
			}

			// /me routes (current-user scoped) — kept outside the subscription
			// gate so in-app tutorial can load even on expired subscriptions.
			userHandler.RegisterMeRoutes(authOnly)

			// Billing routes must work with expired subscription so users can pay
			if paymentHandler != nil {
				billingAuthOnly := authOnly.Group("")
				billingAuthOnly.Use(authMiddleware.RequireFinancialAccess())
				paymentHandler.RegisterRoutes(billingAuthOnly)
			}

			// Marketplace routes (financial access + professional plan required)
			if marketplaceHandler != nil {
				mktAuthOnly := authOnly.Group("")
				mktAuthOnly.Use(tenantMiddleware.ValidateTenant())
				mktAuthOnly.Use(authMiddleware.RequireFinancialAccess())
				mktAuthOnly.Use(auth_middleware.RequirePlan("professional"))
				marketplaceHandler.RegisterRoutes(mktAuthOnly)
			}
		}

		// Protected routes (auth required + active subscription required)
		protected := v1.Group("")
		protected.Use(authMiddleware.Authenticate())
		protected.Use(tenantMiddleware.ValidateSubscription())
		{
			// Professional routes
			professionalHandler.RegisterRoutes(protected)
			professionalProcedureHandler.RegisterRoutes(protected)

			// Patient routes
			patientHandler.RegisterRoutes(protected)
			patientWhatsAppConnHandler.RegisterRoutes(protected)

			// Appointment routes (with professional access control)
			appointmentsProtected := protected.Group("")
			appointmentsProtected.Use(authMiddleware.RequireProfessionalAccess())
			{
				appointmentHandler.RegisterRoutes(appointmentsProtected)
				// Payment sub-routes registered on the same /appointments group to avoid wildcard conflict
				appointmentPaymentHandler.RegisterRoutes(appointmentsProtected.Group("/appointments"))
				// Receipt-token endpoint (shares the same /appointments group)
				appointmentReceiptHandler.RegisterRoutes(appointmentsProtected.Group("/appointments"))
				// Attendance sub-routes
				attendanceHandler.RegisterRoutes(appointmentsProtected)
			}

			// Procedure routes
			procedureHandler.RegisterRoutes(protected)

			// Payment method routes (settings)
			paymentMethodHandler.RegisterRoutes(protected)

			// Specialty routes (settings)
			specialtyHandler.RegisterRoutes(protected)

			// Partner network / plan routes (convênios — settings)
			partnerNetworkHandler.RegisterRoutes(protected)
			partnerPlanHandler.RegisterRoutes(protected)

			// Commission routes (financial reports)
			commissionHandler.RegisterRoutes(protected)

			// Finance routes (manual income/expense + auto-generated from appointments)
			financeHandler.RegisterRoutes(protected)

			// Sales packages (vendas avulsas com saldo flutuante para procedimentos)
			salesPackageHandler.RegisterRoutes(protected)

			// Plan routes (protected endpoints inside PlanHandler/OfferHandler for admins)
			planHandler.RegisterRoutes(protected, authMiddleware)
			offerHandler.RegisterRoutes(protected, authMiddleware)

			// WhatsApp routes (protected endpoints)
			if whatsappHandler != nil {
				whatsappHandler.RegisterRoutes(protected)
			}

			// Calendar routes (protected endpoints)
			if calendarHandler != nil {
				calendarHandler.RegisterRoutes(protected)
			}

			// Session Plan routes
			sessionPlanHandler.RegisterRoutes(protected)
			sessionPlanSlotHandler.RegisterRoutes(protected)

			// Enrollment routes (group session plan enrollments)
			enrollmentHandler.RegisterRoutes(protected)

			// Group attendance routes (group session plan attendance by date)
			groupAttendanceHandler.RegisterRoutes(protected)

			// Group session professional routes (professional assignment per session date)
			groupSessionProfHandler.RegisterRoutes(protected)

			// Plan payment routes (upfront/monthly payments)
			planPaymentHandler.RegisterRoutes(protected)

			// Patient Record routes (clinical records and session notes)
			patientRecordHandler.RegisterRoutes(protected)

			// Onboarding routes
			onboardingHandler.RegisterRoutes(protected)

			// Notification routes
			notificationHandler.RegisterRoutes(protected)

			// Waitlist routes (lista de encaixe)
			if waitlistHandler != nil {
				waitlistHandler.RegisterRoutes(protected)
			}

			// Financial routes (admin/owner only)
			// Dashboard and Analytics require financial access
			financialProtected := protected.Group("")
			financialProtected.Use(authMiddleware.RequireFinancialAccess())
			{
				// Dashboard routes
				dashboard := financialProtected.Group("/dashboard")
				{
					dashboard.GET("/metrics", dashboardHandler.GetMetrics)
				}

				// Analytics routes (slot suggestions and reports)
				analyticsHandler.RegisterRoutes(financialProtected)
			}

			// Admin-only routes
			adminProtected := protected.Group("")
			adminProtected.Use(authMiddleware.RequireAdmin())
			{
				// Invite routes
				inviteHandler.RegisterProtectedRoutes(adminProtected)
				// User management routes
				userHandler.RegisterRoutes(adminProtected)
				// Clinic management routes (admin/owner can update clinic info)
				clinicHandler.RegisterRoutes(adminProtected)
			}

			// Settings routes (dev clinics only - has its own middleware)
			settingsHandler.RegisterRoutes(protected, authMiddleware.Authenticate())

			// Sprint 6: Admin-only routes for observability and control (dev clinics only)
			adminPanel := protected.Group("/admin")
			adminPanel.Use(authMiddleware.Authenticate())
			adminPanel.Use(devOnlyMiddleware())
			{
				auditHandler.RegisterRoutes(adminPanel)
				systemLogHandler.RegisterRoutes(adminPanel)
				featureFlagHandler.RegisterRoutes(adminPanel)
				usageMetricHandler.RegisterRoutes(adminPanel)
				aiTokenUsageHandler.RegisterRoutes(adminPanel)
				clinicHandler.RegisterAdminLookupRoutes(adminPanel)
				leadHandler.RegisterRoutes(adminPanel)
			}
		}

		// WhatsApp webhook (public, validated by secret)
		if whatsappHandler != nil {
			v1.POST("/whatsapp/webhook", whatsappHandler.Webhook)
			v1.POST("/whatsapp/webhook/evolution", whatsappHandler.WebhookEvolution)
			v1.POST("/whatsapp/webhook/waha", whatsappHandler.WebhookWaha)
			v1.POST("/whatsapp/webhook/gupshup", whatsappHandler.WebhookGupshup)
			v1.POST("/whatsapp/webhook/:secret", whatsappHandler.WebhookWithSecret)
		}

		// Payment webhook (public, validated by secret)
		if paymentHandler != nil {
			v1.POST("/billing/webhook", paymentHandler.Webhook)
		}

		// Marketplace public routes and webhook
		if marketplaceHandler != nil {
			marketplaceHandler.RegisterPublicRoutes(v1)
			marketplaceHandler.RegisterWebhookRoutes(v1)
		}
	}

	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": map[string]interface{}{
				"type":    "NOT_FOUND",
				"message": "Route not found",
			},
		})
	})

	return router
}

// devOnlyMiddleware restricts access to dev-type clinics only (admin panel)
func devOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clinicRaw, exists := c.Get("clinic")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Não autorizado"})
			c.Abort()
			return
		}

		clinic, ok := clinicRaw.(*clinic_model.Clinic)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro interno"})
			c.Abort()
			return
		}

		if clinic.Type != clinic_model.ClinicTypeDev {
			c.JSON(http.StatusForbidden, gin.H{"error": "Acesso restrito a clínicas de desenvolvimento"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// googleAuthAdapter adapts the Google client to the auth service interface
type googleAuthAdapter struct {
	client *auth_google.Client
}

func (a *googleAuthAdapter) VerifyIDToken(ctx context.Context, idToken string) (*auth_service.GoogleUserInfo, error) {
	info, err := a.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}
	return &auth_service.GoogleUserInfo{
		ID:            info.ID,
		Email:         info.Email,
		VerifiedEmail: info.VerifiedEmail,
		Name:          info.Name,
		GivenName:     info.GivenName,
		FamilyName:    info.FamilyName,
		Picture:       info.Picture,
	}, nil
}

func runMigrations(db *database.Database) error {
	logger.Println("Running database migrations...")

	// Add all models that need migration here
	// Order matters due to foreign key constraints
	models := []interface{}{
		&plan_model.Plan{},
		&plan_model.PlanCycle{},
		&plan_model.Offer{},
		&plan_model.OfferCycle{},
		&account_model.Account{},
		&clinic_model.Clinic{},
		&user_model.User{},
		&professional_model.Professional{},
		&patient_model.Patient{},
		&appointment_model.Appointment{},
		&whatsapp_model.Message{},
		&whatsapp_model.MessageLog{},
		&whatsapp_model.SchedulingIntent{},
		&whatsapp_model.ScheduledMessage{},
		&payment_model.Customer{},
		&payment_model.Subscription{},
		&payment_model.Payment{},
		&calendar_model.CalendarIntegration{},
		&calendar_model.CalendarSync{},
		&sessionplan_model.SessionPlan{},
		&sessionplan_model.SessionPlanSlot{},
		&attendance_model.AppointmentAttendance{},
		&attendance_model.GroupSessionAttendance{},
		&attendance_model.GroupSessionProfessional{},
		&enrollment_model.SessionPlanEnrollment{},
		&enrollment_model.EnrollmentSlot{},
		&sessionplan_model.SessionPlanPayment{},
		&sessionplan_model.SessionPlanPaymentReminder{},
		&invite_model.Invite{},
		&patientrecord_model.PatientRecord{},
		&patientrecord_model.SessionNote{},
		&patientrecord_model.AnamneseTemplate{},
		&patientrecord_model.AnamneseTemplateField{},
		&patientrecord_model.AnamneseTemplateSpecialty{},
		&auth_model.PasswordReset{},
		&auth_model.EmailVerification{},
		&notification_model.Notification{},
		&waitlist_model.WaitlistEntry{},
		&settings_model.SystemSetting{},
		&whatsapp_model.WhatsAppConnection{},
		// Procedure models
		&procedure_model.Procedure{},
		&appointment_model.AppointmentProcedure{},
		&appointment_model.AppointmentSpecialty{},
		// AI Agent conversation tables
		&conversation.ConversationContext{},
		&conversation.ToolCallLog{},
		// Sprint 6: Observability and control tables
		&audit_model.AuditLog{},
		&systemlog_model.SystemLog{},
		&featureflag_model.FeatureFlag{},
		&usagemetric_model.UsageMetric{},
		&aitokenusage_model.AITokenUsage{},
		// Multi-clinic membership
		&clinicmember_model.ClinicMember{},
		// CRM Leads
		&lead_model.Lead{},
		// Patient WhatsApp connection preferences
		&patient_model.PatientWhatsAppConnection{},
		// Payment methods and appointment payments
		&paymentmethod_model.PaymentMethod{},
		&appointment_model.AppointmentPayment{},
		// Public receipt share tokens
		&appointment_model.AppointmentReceiptToken{},
		// Specialties and portal accounts
		&specialty_model.Specialty{},
		&partnernetwork_model.PartnerNetwork{},
		&partnernetwork_model.PartnerPlan{},
		&professional_model.ProfessionalSpecialty{},
		&professional_model.ProfessionalProcedure{},
		&commission_model.Commission{},
		&portalaccount_model.PortalAccount{},
		// Marketplace (subcontas)
		&marketplace_model.Subaccount{},
		&marketplace_model.SplitConfig{},
		&marketplace_model.ClinicPayment{},
		// Finance (contas a pagar/receber + fluxo de caixa)
		&finance_model.FinancialCategory{},
		&finance_model.FinancialTransaction{},
		// Sales packages (vendas avulsas com saldo flutuante)
		&salespackage_model.SalesPackage{},
		&salespackage_model.SalesPackageItem{},
		&salespackage_model.SalesPackagePayment{},
		// Group class reminder dedup log — written by the class reminder
		// worker so the same (clinic, plan, patient, date, threshold) tuple
		// is never enqueued twice.
		&sessionplan_model.GroupClassReminderLog{},
	}

	if err := db.AutoMigrate(models...); err != nil {
		return err
	}

	// Backfill SessionPlanSlot rows for legacy group plans created before the
	// slot model existed. Idempotent — only runs for plans with no slots yet.
	if err := sessionplan_migration.PortGroupPlansToSlots(context.Background(), db.GetDB()); err != nil {
		logger.Printf("Warning: PortGroupPlansToSlots failed: %v", err)
	}

	// Finance hooks now key by the specific payment event — one finance row per
	// appointment_payment (signal, installment, final) and per session_plan_payment
	// (upfront/monthly). Legacy aggregate indexes are dropped so replaying the
	// hook on an edit/delete hits the right row.
	db.GetDB().Exec(`DROP INDEX IF EXISTS idx_financial_tx_appointment_unique`)
	db.GetDB().Exec(`DROP INDEX IF EXISTS idx_financial_tx_commission_unique`)

	db.GetDB().Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_financial_tx_appointment_payment_unique
		ON financial_transactions (appointment_payment_id)
		WHERE appointment_payment_id IS NOT NULL AND deleted_at IS NULL
	`)
	db.GetDB().Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_financial_tx_session_plan_payment_unique
		ON financial_transactions (session_plan_payment_id)
		WHERE session_plan_payment_id IS NOT NULL AND deleted_at IS NULL
	`)
	db.GetDB().Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_financial_tx_sales_package_payment_unique
		ON financial_transactions (sales_package_payment_id)
		WHERE sales_package_payment_id IS NOT NULL AND deleted_at IS NULL
	`)
	// payment_method_id became nullable when open receivables were introduced
	// (a sale that hasn't been paid yet has no method until the patient settles).
	// AutoMigrate doesn't relax NOT NULL on existing columns, so do it by hand.
	db.GetDB().Exec(`ALTER TABLE sales_package_payments ALTER COLUMN payment_method_id DROP NOT NULL`)

	// Backfill payment_methods settlement parameters for clinics that already
	// have methods registered before this column was introduced. Credit cards
	// default to 30-day settlement with installments enabled (12x); all other
	// types default to immediate, single-payment.
	db.GetDB().Exec(`
		UPDATE payment_methods
		SET settlement_days = 30, installments_allowed = TRUE, max_installments = 12
		WHERE type = 'credit'
		  AND settlement_days = 0
		  AND installments_allowed = FALSE
	`)

	// Drop legacy unique index on commissions.appointment_id (now nullable for group sessions)
	db.GetDB().Exec(`DROP INDEX IF EXISTS idx_commissions_appointment_id`)

	// Make appointment_id nullable for group session commissions (GORM AutoMigrate does not drop NOT NULL)
	db.GetDB().Exec(`ALTER TABLE commissions ALTER COLUMN appointment_id DROP NOT NULL`)

	// Partial index for payment timeout worker (only indexes awaiting_payment rows)
	db.GetDB().Exec(`
		CREATE INDEX IF NOT EXISTS idx_appointments_awaiting_payment
		ON appointments (status, payment_expires_at)
		WHERE status = 'awaiting_payment'
	`)

	// Indexes for patient records (append-only and JSONB search)
	db.GetDB().Exec(`
		CREATE INDEX IF NOT EXISTS idx_patient_records_latest 
		ON patient_records(patient_id, clinic_id, recorded_at DESC)
	`)
	db.GetDB().Exec(`
		CREATE INDEX IF NOT EXISTS idx_patient_records_custom_fields
		ON patient_records USING GIN (custom_fields)
	`)

	// Unique constraint prevents the payment reminder worker from inserting
	// two reminders for the same plan/enrollment/reference_month/channel.
	// COALESCE trick lets NULL enrollment_ids (individual plans) share the
	// same uniqueness space without colliding with group enrollments.
	db.GetDB().Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_session_plan_payment_reminders_unique
		ON session_plan_payment_reminders (
			clinic_id,
			session_plan_id,
			COALESCE(enrollment_id, '00000000-0000-0000-0000-000000000000'::uuid),
			reference_month,
			channel
		)
	`)

	// Data Migration: Migrando plan, status, e asaas id da tabela clinics para a nova accounts
	if err := db.GetDB().Exec(`
		DO $$
		DECLARE
			c RECORD;
			new_account_id UUID;
			owner_id UUID;
			column_exists BOOLEAN;
		BEGIN
			-- Check se a coluna asaas_customer_id ainda existe na tabela clinics
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_name = 'clinics' AND column_name = 'asaas_customer_id'
			) INTO column_exists;

			IF column_exists THEN
				-- Query dynamic para evitar erro de compilacao se a coluna sumir
				FOR c IN EXECUTE 'SELECT id, asaas_customer_id, plan, subscription_status, trial_ends_at, subscription_ends_at FROM clinics WHERE account_id IS NULL OR account_id = ''00000000-0000-0000-0000-000000000000'''
				LOOP
					-- Encontre o dono clinica
					SELECT id INTO owner_id FROM users WHERE clinic_id = c.id AND role = 'owner' LIMIT 1;
					
					IF owner_id IS NOT NULL THEN
						new_account_id := gen_random_uuid();
						
						INSERT INTO accounts (id, owner_id, asaas_customer_id, plan, subscription_status, trial_ends_at, subscription_ends_at, created_at, updated_at)
						VALUES (new_account_id, owner_id, c.asaas_customer_id, c.plan, c.subscription_status, c.trial_ends_at, c.subscription_ends_at, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
						
						UPDATE clinics SET account_id = new_account_id WHERE id = c.id;
					END IF;
				END LOOP;
			END IF;
		END $$;
	`).Error; err != nil {
		logger.Printf("Data migration error: %v (Pode ser ignorado se já migrado)", err)
	}

	// Definir account_id como NOT NULL se possível
	db.GetDB().Exec("ALTER TABLE clinics ALTER COLUMN account_id SET NOT NULL")

	// Data Migration: Migrate professional.specialty string to specialties table + N:N junction
	if err := db.GetDB().Exec(`
		DO $$
		DECLARE
			rec RECORD;
			new_id UUID;
			spec_id UUID;
		BEGIN
			-- Create specialty records from distinct professional.specialty strings per clinic
			FOR rec IN
				SELECT DISTINCT clinic_id, specialty
				FROM professionals
				WHERE specialty IS NOT NULL AND specialty != '' AND deleted_at IS NULL
				  AND NOT EXISTS (
					SELECT 1 FROM specialties s
					WHERE s.clinic_id = professionals.clinic_id AND s.name = professionals.specialty AND s.deleted_at IS NULL
				  )
			LOOP
				new_id := gen_random_uuid();
				INSERT INTO specialties (id, clinic_id, name, active, created_at, updated_at)
				VALUES (new_id, rec.clinic_id, rec.specialty, true, NOW(), NOW());
			END LOOP;

			-- Backfill professional_specialties junction table
			FOR rec IN
				SELECT p.id AS professional_id, s.id AS specialty_id
				FROM professionals p
				JOIN specialties s ON p.clinic_id = s.clinic_id AND p.specialty = s.name AND s.deleted_at IS NULL
				WHERE p.specialty IS NOT NULL AND p.specialty != '' AND p.deleted_at IS NULL
				  AND NOT EXISTS (
					SELECT 1 FROM professional_specialties ps
					WHERE ps.professional_id = p.id AND ps.specialty_id = s.id
				  )
			LOOP
				INSERT INTO professional_specialties (professional_id, specialty_id)
				VALUES (rec.professional_id, rec.specialty_id);
			END LOOP;
		END $$;
	`).Error; err != nil {
		logger.Printf("Specialty migration error: %v (Pode ser ignorado se já migrado)", err)
	}

	// Backfill clinic_members from users table (idempotent)
	if err := db.GetDB().Exec(`
		INSERT INTO clinic_members (user_id, clinic_id, role, is_active, created_at, updated_at)
		SELECT u.id, u.clinic_id, u.role, u.is_active, u.created_at, u.updated_at
		FROM users u
		WHERE u.deleted_at IS NULL
		  AND u.clinic_id IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1 FROM clinic_members cm
			WHERE cm.user_id = u.id AND cm.clinic_id = u.clinic_id AND cm.deleted_at IS NULL
		  )
	`).Error; err != nil {
		logger.Printf("clinic_members backfill error: %v", err)
	}

	logger.Println("Database migrations completed")
	return nil
}

// seedDefaultPlans creates the default plans with pricing cycles if they don't exist
func seedDefaultPlans(db *gorm.DB) error {
	plans := []plan_model.Plan{
		{
			Name:                   "Básico",
			Slug:                   "basic",
			IsActive:               true,
			MaxClinics:             1,
			MaxProfessionals:       5,
			MaxWhatsappConnections: 1,
			MaxGoogleCalendars:     0,
			HasApiAccess:           false,
			Features: plan_model.StringArray{
				"Até 5 profissionais",
				"Agenda ilimitada",
				"Landing page pública da clínica",
				"Agendamento online",
				"Portal do Paciente",
				"Confirmações WhatsApp",
				"Sessões recorrentes",
				"Gestão de pacientes",
				"Histórico de atendimentos",
				"Acesso via celular/computador",
				"Suporte por email",
			},
			Cycles: []plan_model.PlanCycle{
				{CycleType: plan_model.CycleTypeMonthly, PriceBRL: 99},
				{CycleType: plan_model.CycleTypeYearly, PriceBRL: 990},
			},
		},
		{
			Name:                   "Profissional",
			Slug:                   "professional",
			IsActive:               true,
			MaxClinics:             1,
			MaxProfessionals:       15,
			MaxWhatsappConnections: 1,
			MaxGoogleCalendars:     1,
			HasApiAccess:           false,
			Features: plan_model.StringArray{
				"Até 15 profissionais",
				"Tudo do Básico",
				"Integração Google Calendar",
				"Teleconsulta (Google Meet)",
				"Lista de Espera Automática",
				"Detector de Risco de Faltas",
				"Relatórios de Prejuízo",
				"Relatórios de Faltas",
				"Personalização de mensagens",
				"Suporte prioritário WhatsApp",
			},
			Cycles: []plan_model.PlanCycle{
				{CycleType: plan_model.CycleTypeMonthly, PriceBRL: 199},
				{CycleType: plan_model.CycleTypeYearly, PriceBRL: 1990},
			},
		},
		{
			Name:                   "Avançado",
			Slug:                   "advanced",
			IsActive:               true,
			MaxClinics:             -1,
			MaxProfessionals:       -1,
			MaxWhatsappConnections: -1,
			MaxGoogleCalendars:     -1,
			HasApiAccess:           true,
			Features: plan_model.StringArray{
				"Profissionais ilimitados",
				"Tudo do Profissional",
				"API de integração",
				"Treinamento da equipe",
				"Importação de dados",
				"SLA de 99.9%",
				"Backup diário automático",
			},
			Cycles: []plan_model.PlanCycle{
				{CycleType: plan_model.CycleTypeMonthly, PriceBRL: 349},
				{CycleType: plan_model.CycleTypeYearly, PriceBRL: 3490},
			},
		},
	}

	for _, p := range plans {
		var existing plan_model.Plan
		result := db.Where("slug = ?", p.Slug).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			// Plan doesn't exist, create with cycles
			if err := db.Create(&p).Error; err != nil {
				return err
			}
			logger.Printf("Seeded plan: %s", p.Name)
		} else if result.Error == nil {
			// Plan exists, ensure it has cycles
			var cycleCount int64
			db.Model(&plan_model.PlanCycle{}).Where("plan_id = ?", existing.ID).Count(&cycleCount)
			if cycleCount == 0 {
				for i := range p.Cycles {
					p.Cycles[i].PlanID = existing.ID
				}
				if err := db.Create(&p.Cycles).Error; err != nil {
					return err
				}
				logger.Printf("Seeded cycles for existing plan: %s", existing.Name)
			}
			// Seed features and is_active for existing plans that don't have them
			updates := map[string]interface{}{"is_active": true}
			if len(existing.Features) == 0 && len(p.Features) > 0 {
				updates["features"] = p.Features
			}
			db.Model(&existing).Updates(updates)
		}
	}

	return nil
}

// Adapters for waitlist service dependencies

// clinicInfoAdapter adapts clinic_repository to waitlist_service.ClinicInfoProvider
type clinicInfoAdapter struct {
	clinicRepo clinic_repository.ClinicRepository
}

func (a *clinicInfoAdapter) GetClinicInfo(ctx context.Context, clinicID uuid.UUID) (*waitlist_service.ClinicInfo, error) {
	clinic, err := a.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	return &waitlist_service.ClinicInfo{
		ID:   clinic.ID,
		Name: clinic.Name,
	}, nil
}

// professionalInfoAdapter adapts professional_repository to waitlist_service.ProfessionalInfoProvider
type professionalInfoAdapter struct {
	professionalRepo professional_repository.ProfessionalRepository
}

func (a *professionalInfoAdapter) GetProfessionalInfo(ctx context.Context, professionalID, clinicID uuid.UUID) (*waitlist_service.ProfessionalInfo, error) {
	professional, err := a.professionalRepo.GetByID(ctx, professionalID, clinicID)
	if err != nil {
		return nil, err
	}
	return &waitlist_service.ProfessionalInfo{
		ID:   professional.ID,
		Name: professional.Name,
	}, nil
}

// appointmentCreatorAdapter adapts appointment_service to waitlist_service.AppointmentCreator
type appointmentCreatorAdapter struct {
	appointmentService appointment_service.AppointmentService
}

func (a *appointmentCreatorAdapter) CreateFromWaitlist(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) (uuid.UUID, error) {
	return a.appointmentService.CreateFromWaitlist(ctx, clinicID, patientID, professionalID, startTime, durationMinutes)
}

func (a *appointmentCreatorAdapter) Reschedule(ctx context.Context, appointmentID, clinicID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) error {
	return a.appointmentService.Reschedule(ctx, appointmentID, clinicID, professionalID, startTime, durationMinutes)
}

// patientRiskAdapter adapts patient_service to appointment_service.PatientRiskCalculator
type patientRiskAdapter struct {
	patientService patient_service.PatientService
}

func (a *patientRiskAdapter) CalculateRiskScore(ctx context.Context, patientID, clinicID uuid.UUID) error {
	return a.patientService.CalculateRiskScore(ctx, patientID, clinicID)
}

// clinicSubscriptionCheckerAdapter adapts the clinic repository to
// worker.ClinicSubscriptionChecker. It reads the clinic (with preloaded
// Account) and delegates to Account.IsSubscriptionActive so the reminder
// worker stops enqueuing messages for clinics whose trial expired or whose
// subscription was canceled.
type clinicSubscriptionCheckerAdapter struct {
	clinicRepo clinic_repository.ClinicRepository
}

func (a *clinicSubscriptionCheckerAdapter) IsSubscriptionActive(ctx context.Context, clinicID uuid.UUID) (bool, error) {
	clinic, err := a.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return false, err
	}
	if clinic == nil || clinic.Account == nil {
		return false, nil
	}
	return clinic.Account.IsSubscriptionActive(), nil
}

// appointmentStatusAdapter adapts appointment_service to whatsapp_service.AppointmentStatusUpdater
type appointmentStatusAdapter struct {
	appointmentService appointment_service.AppointmentService
}

func (a *appointmentStatusAdapter) ConfirmByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, patientNotes string) error {
	_, err := a.appointmentService.Confirm(ctx, appointmentID, clinicID, &appointment_model.ConfirmAppointmentRequest{
		PatientNotes: patientNotes,
	})
	return err
}

func (a *appointmentStatusAdapter) CancelByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, reason string) error {
	_, err := a.appointmentService.Cancel(ctx, appointmentID, clinicID, &appointment_model.CancelAppointmentRequest{
		Reason: reason,
	})
	return err
}

// appointmentFinderAdapter adapts appointment_repository to whatsapp_service.AppointmentFinder
type appointmentFinderAdapter struct {
	appointmentRepo appointment_repository.AppointmentRepository
}

func (a *appointmentFinderAdapter) GetPendingByPhone(ctx context.Context, clinicID uuid.UUID, phone string) (*whatsapp_service.AppointmentInfo, error) {
	apt, err := a.appointmentRepo.GetPendingByPhone(ctx, clinicID, phone)
	if err != nil {
		return nil, err
	}
	if apt == nil {
		return nil, nil
	}

	profName := ""
	if apt.Professional != nil {
		profName = apt.Professional.Name
	}

	patientName := ""
	patientPhone := ""
	if apt.Patient != nil {
		patientName = apt.Patient.Name
		if apt.Patient.WhatsApp != "" {
			patientPhone = apt.Patient.WhatsApp
		} else {
			patientPhone = apt.Patient.Phone
		}
	}

	return &whatsapp_service.AppointmentInfo{
		ID:                    apt.ID,
		PatientID:             apt.PatientID,
		ProfessionalID:        apt.ProfessionalID,
		PatientName:           patientName,
		PatientPhone:          patientPhone,
		Professional:          profName,
		StartTime:             apt.StartTime,
		EndTime:               apt.EndTime,
		Status:                string(apt.Status),
		ConfirmationExpiresAt: apt.ConfirmationExpiresAt,
	}, nil
}

// whatsappAppointmentCreatorAdapter adapts appointment_service to whatsapp_service.AppointmentCreator
type whatsappAppointmentCreatorAdapter struct {
	appointmentService appointment_service.AppointmentService
}

func (a *whatsappAppointmentCreatorAdapter) CreateFromReschedule(ctx context.Context, clinicID uuid.UUID, patientPhone string, slot *appointment_model.SmartSlotSuggestion, originalApt *whatsapp_service.AppointmentInfo) (*whatsapp_service.AppointmentInfo, error) {
	// Calculate duration from slot
	durationMinutes := int(slot.EndTime.Sub(slot.StartTime).Minutes())
	if durationMinutes <= 0 {
		durationMinutes = 50 // Default duration
	}

	// Create new appointment
	req := &appointment_model.CreateAppointmentRequest{
		PatientID:       originalApt.PatientID,
		ProfessionalID:  originalApt.ProfessionalID,
		StartTime:       slot.StartTime,
		DurationMinutes: durationMinutes,
		Title:           "Reagendamento",
		Notes:           "Reagendado via WhatsApp",
	}

	aptResp, err := a.appointmentService.Create(ctx, clinicID, req)
	if err != nil {
		return nil, err
	}

	// Confirm the appointment immediately since patient chose this slot
	_, err = a.appointmentService.Confirm(ctx, aptResp.ID, clinicID, &appointment_model.ConfirmAppointmentRequest{
		PatientNotes: "Confirmado automaticamente no reagendamento",
	})
	if err != nil {
		// Log but don't fail - appointment was created
		logger.Printf("[Reschedule] Failed to auto-confirm appointment: %v", err)
	}

	// Get names from nested structs
	patientName := ""
	if aptResp.Patient != nil {
		patientName = aptResp.Patient.Name
	}
	professionalName := ""
	if aptResp.Professional != nil {
		professionalName = aptResp.Professional.Name
	}

	return &whatsapp_service.AppointmentInfo{
		ID:             aptResp.ID,
		PatientID:      aptResp.PatientID,
		ProfessionalID: aptResp.ProfessionalID,
		PatientName:    patientName,
		PatientPhone:   patientPhone,
		Professional:   professionalName,
		StartTime:      aptResp.StartTime,
		EndTime:        aptResp.EndTime,
	}, nil
}

// Adapters for WhatsApp scheduling flow

// patientFinderCreatorAdapter adapts patient_repository to whatsapp_service.PatientFinderCreator
type patientFinderCreatorAdapter struct {
	patientRepo patient_repository.PatientRepository
}

func (a *patientFinderCreatorAdapter) FindOrCreateByPhone(ctx context.Context, clinicID uuid.UUID, phone, name string) (*whatsapp_service.PatientInfo, error) {
	// Try to find existing patient by phone
	patient, err := a.patientRepo.GetByPhone(ctx, phone, clinicID)
	if err == nil && patient != nil {
		return &whatsapp_service.PatientInfo{
			ID:    patient.ID,
			Name:  patient.Name,
			Phone: patient.Phone,
		}, nil
	}

	// Create new patient
	newPatient := &patient_model.Patient{
		ClinicID: clinicID,
		Name:     name,
		Phone:    phone,
		WhatsApp: phone,
		IsActive: true,
	}
	if err := a.patientRepo.Create(ctx, newPatient); err != nil {
		return nil, err
	}

	return &whatsapp_service.PatientInfo{
		ID:    newPatient.ID,
		Name:  newPatient.Name,
		Phone: newPatient.Phone,
	}, nil
}

// patientAIPreferenceAdapter checks if AI scheduling is enabled for a patient by phone
type patientAIPreferenceAdapter struct {
	patientRepo patient_repository.PatientRepository
	connRepo    patient_repository.PatientWhatsAppConnectionRepository
}

// humanTakeoverTrackerAdapter is the whatsapp_service.HumanTakeoverTracker
// implementation. Persists the timestamp on the patient_whatsapp_connections
// row keyed by phone → patient → connection. When no patient exists yet for
// the phone, skips silently (no conversation yet = nothing to pause).
type humanTakeoverTrackerAdapter struct {
	patientRepo  patient_repository.PatientRepository
	connRepo     patient_repository.PatientWhatsAppConnectionRepository
	whatsappRepo whatsapp_repository.WhatsAppConnectionRepository
}

func (a *humanTakeoverTrackerAdapter) Mark(ctx context.Context, clinicID uuid.UUID, phone string, at time.Time) error {
	patient, err := a.patientRepo.GetByPhone(ctx, phone, clinicID)
	if err != nil || patient == nil {
		// No patient record → nothing to pause. A later incoming message
		// will create the patient and the takeover state is lost, but
		// that's fine: without a patient, there's no AI conversation to
		// silence. Fail-open.
		return nil
	}
	conn, err := a.whatsappRepo.GetActiveByClinicID(ctx, clinicID)
	if err != nil || conn == nil {
		return nil
	}
	return a.connRepo.SetHumanTakeoverAt(ctx, patient.ID, clinicID, conn.ID, &at)
}

func (a *humanTakeoverTrackerAdapter) GetTakeoverAt(ctx context.Context, clinicID uuid.UUID, phone string) (*time.Time, error) {
	patient, err := a.patientRepo.GetByPhone(ctx, phone, clinicID)
	if err != nil || patient == nil {
		return nil, nil
	}
	return a.connRepo.GetTakeoverAtByPatientID(ctx, patient.ID, clinicID)
}

func (a *patientAIPreferenceAdapter) IsAISchedulingEnabledByPhone(ctx context.Context, clinicID uuid.UUID, phone string) (bool, error) {
	patient, err := a.patientRepo.GetByPhone(ctx, phone, clinicID)
	if err != nil || patient == nil {
		// Patient not found — default to AI enabled (new patient)
		return true, nil
	}

	conn, err := a.connRepo.GetByPatientID(ctx, patient.ID, clinicID)
	if err != nil {
		// No preference record — default to AI enabled
		return true, nil
	}

	return conn.AISchedulingEnabled, nil
}

// patientEnsurerAdapter ensures a patient record exists when a WhatsApp message arrives.
// If no patient exists for the given phone, creates one with Source = WHATSAPP_INBOUND.
type patientEnsurerAdapter struct {
	patientRepo    patient_repository.PatientRepository
	patientService patient_service.PatientService
}

func (a *patientEnsurerAdapter) EnsureByPhone(ctx context.Context, clinicID uuid.UUID, phone, senderName string) error {
	// If patient already exists, nothing to do
	patient, err := a.patientRepo.GetByPhone(ctx, phone, clinicID)
	if err == nil && patient != nil {
		return nil
	}

	name := senderName
	if name == "" {
		name = phone
	}

	_, err = a.patientService.Create(ctx, clinicID, &patient_model.CreatePatientRequest{
		Name:   name,
		Phone:  phone,
		Source: patient_model.PatientSourceWhatsApp,
	})
	return err
}

// professionalListerAdapter adapts professional_repository to whatsapp_service.ProfessionalLister
type professionalListerAdapter struct {
	professionalRepo professional_repository.ProfessionalRepository
}

func (a *professionalListerAdapter) ListAvailable(ctx context.Context, clinicID uuid.UUID, specialty string) ([]*whatsapp_service.ProfessionalInfo, error) {
	professionals, _, err := a.professionalRepo.List(ctx, clinicID, pagination.Filter{Status: "active", Limit: 100})
	if err != nil {
		return nil, err
	}

	var result []*whatsapp_service.ProfessionalInfo
	for _, p := range professionals {
		result = append(result, &whatsapp_service.ProfessionalInfo{
			ID:          p.ID,
			Name:        p.Name,
			Specialty:   "", // Could be extracted from professional if available
			IsAvailable: true,
		})
	}
	return result, nil
}

// slotFinderAdapter adapts appointment_service to whatsapp_service.SlotFinder
type slotFinderAdapter struct {
	appointmentService appointment_service.AppointmentService
	professionalRepo   professional_repository.ProfessionalRepository
	clinicRepo         clinic_repository.ClinicRepository
}

func (a *slotFinderAdapter) clinicDateInLocalTZ(ctx context.Context, clinicID uuid.UUID, date time.Time) time.Time {
	loc := time.Local
	if a.clinicRepo != nil {
		if clinic, err := a.clinicRepo.GetByID(ctx, clinicID); err == nil && clinic != nil && clinic.Settings.Timezone != "" {
			if parsed, lerr := time.LoadLocation(clinic.Settings.Timezone); lerr == nil {
				loc = parsed
			}
		}
	}
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
}

func (a *slotFinderAdapter) GetAvailableSlots(ctx context.Context, clinicID uuid.UUID, professionalID uuid.UUID, date time.Time, durationMinutes int) ([]*whatsapp_service.AvailableSlot, error) {
	date = a.clinicDateInLocalTZ(ctx, clinicID, date)
	slots, err := a.appointmentService.SuggestSmartSlots(ctx, clinicID, professionalID, date, 14, durationMinutes)
	if err != nil {
		return nil, err
	}

	// Get professional name
	profName := ""
	prof, err := a.professionalRepo.GetByID(ctx, professionalID, clinicID)
	if err == nil && prof != nil {
		profName = prof.Name
	}

	var result []*whatsapp_service.AvailableSlot
	for _, s := range slots {
		result = append(result, &whatsapp_service.AvailableSlot{
			ProfessionalID:   professionalID,
			ProfessionalName: profName,
			StartTime:        s.StartTime,
			EndTime:          s.EndTime,
		})
	}
	return result, nil
}

func (a *slotFinderAdapter) GetAvailableSlotsMultipleProfessionals(ctx context.Context, clinicID uuid.UUID, professionalIDs []uuid.UUID, date time.Time, days int, durationMinutes int, limit int) ([]*whatsapp_service.AvailableSlot, error) {
	date = a.clinicDateInLocalTZ(ctx, clinicID, date)
	var result []*whatsapp_service.AvailableSlot

	// Iterate through professionals to find slots
	for _, profID := range professionalIDs {
		if len(result) >= limit {
			break
		}

		slots, err := a.appointmentService.SuggestSmartSlots(ctx, clinicID, profID, date, days, durationMinutes)
		if err != nil {
			continue
		}

		// Get professional name
		profName := ""
		prof, err := a.professionalRepo.GetByID(ctx, profID, clinicID)
		if err == nil && prof != nil {
			profName = prof.Name
		}

		for _, s := range slots {
			if len(result) >= limit {
				break
			}
			result = append(result, &whatsapp_service.AvailableSlot{
				ProfessionalID:   profID,
				ProfessionalName: profName,
				StartTime:        s.StartTime,
				EndTime:          s.EndTime,
			})
		}
	}

	return result, nil
}

// newAppointmentCreatorAdapter adapts appointment_service to whatsapp_service.NewAppointmentCreator
type newAppointmentCreatorAdapter struct {
	appointmentService appointment_service.AppointmentService
}

func (a *newAppointmentCreatorAdapter) CreateFromSchedulingIntent(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) (*whatsapp_service.AppointmentInfo, error) {
	req := &appointment_model.CreateAppointmentRequest{
		PatientID:       patientID,
		ProfessionalID:  professionalID,
		StartTime:       startTime,
		DurationMinutes: durationMinutes,
		Title:           "Agendamento via WhatsApp",
		Notes:           "Agendado automaticamente via WhatsApp",
	}

	aptResp, err := a.appointmentService.Create(ctx, clinicID, req)
	if err != nil {
		return nil, err
	}

	// Auto-confirm since patient explicitly chose this slot
	_, err = a.appointmentService.Confirm(ctx, aptResp.ID, clinicID, &appointment_model.ConfirmAppointmentRequest{
		PatientNotes: "Confirmado automaticamente no agendamento via WhatsApp",
	})
	if err != nil {
		logger.Printf("[Schedule] Failed to auto-confirm appointment: %v", err)
	}

	patientName := ""
	if aptResp.Patient != nil {
		patientName = aptResp.Patient.Name
	}
	professionalName := ""
	if aptResp.Professional != nil {
		professionalName = aptResp.Professional.Name
	}

	return &whatsapp_service.AppointmentInfo{
		ID:             aptResp.ID,
		PatientID:      aptResp.PatientID,
		ProfessionalID: aptResp.ProfessionalID,
		PatientName:    patientName,
		Professional:   professionalName,
		StartTime:      aptResp.StartTime,
		EndTime:        aptResp.EndTime,
	}, nil
}

// aiTokenUsageAdapter bridges the agent package's AITokenUsageRecorder
// interface to the aitokenusage service, translating the event payload and
// computing a best-effort BRL cost from the model name.
type aiTokenUsageAdapter struct {
	svc aitokenusage_service.Service
}

func (a *aiTokenUsageAdapter) Record(ctx context.Context, evt agent.AITokenUsageEvent) error {
	return a.svc.Record(ctx, aitokenusage_service.RecordInput{
		ClinicID:       evt.ClinicID,
		ConversationID: evt.ConversationID,
		Provider:       evt.Provider,
		Model:          evt.Model,
		InputTokens:    evt.InputTokens,
		OutputTokens:   evt.OutputTokens,
		TotalTokens:    evt.TotalTokens,
		CostBRL:        agent.EstimateTokenCostSplit(evt.InputTokens, evt.OutputTokens, evt.Model),
	})
}

// initializeAIAgent initializes the AI agent stack for WhatsApp conversations
func initializeAIAgent(
	cfg *config.Config,
	db *database.Database,
	redis *redisClient.RedisClient,
	professionalService professional_service.ProfessionalService,
	appointmentService appointment_service.AppointmentService,
	appointmentRepo appointment_repository.AppointmentRepository,
	patientService patient_service.PatientService,
	clinicService clinic_service.ClinicService,
	procedureService procedure_service.ProcedureService,
	specialtyRepo specialty_repository.SpecialtyRepository,
	patientRepo patient_repository.PatientRepository,
	bookingService booking_service.BookingService,
	clinicRepo clinic_repository.ClinicRepository,
	professionalRepo professional_repository.ProfessionalRepository,
	professionalProcedureRepo professional_repository.ProfessionalProcedureRepository,
	systemLogSvc systemlog_service.SystemLogService,
	usageMetricSvc usagemetric_service.UsageMetricService,
	aiTokenUsageSvc aitokenusage_service.Service,
	humanTakeoverMarker tools.HumanTakeoverMarker,
	sessionPlanRepo sessionplan_repository.SessionPlanRepository,
	sessionPlanSlotRepo sessionplan_repository.SessionPlanSlotRepository,
	enrollmentRepo enrollment_repository.EnrollmentRepository,
	enrollmentService enrollment_service.EnrollmentService,
	partnerNetworkRepo partnernetwork_repository.PartnerNetworkRepository,
	groupAttendanceRepo attendance_repository.GroupAttendanceRepository,
) (agent.Service, conversation.Manager, *tools.ExecutionContext, error) {
	// Check if AI agent is enabled
	workerCfg := config.NewWorkerConfig()
	if !workerCfg.AIAgentEnabled {
		logger.Println("AI Agent is disabled (AI_AGENT_ENABLED=false)")
		return nil, nil, nil, nil
	}

	logger.Println("Initializing AI Agent for WhatsApp...")

	// 1. Create conversation repository
	conversationRepo := conversation.NewRepository(
		db.GetDB(),
		redis.GetClient(),
		workerCfg.ConversationRedisTTL,
		workerCfg.ConversationPostgresTTL,
	)

	// 2. Create conversation manager
	conversationMgr := conversation.NewManager(
		conversationRepo,
		workerCfg.ConversationRedisTTL,
		workerCfg.ConversationPostgresTTL,
		workerCfg.ConversationMaxTurns,
	)

	// 3. Create tool registry (defines all 10 tools)
	toolRegistry := tools.NewRegistry()

	// 4. Create execution context (will be updated per request)
	execContext := &tools.ExecutionContext{
		// Service dependencies - wrapped with adapters
		ProfessionalLister:       &aiAgentProfessionalListerAdapter{professionalService: professionalService, professionalProcedures: professionalProcedureRepo},
		ProcedureLister:          &aiAgentProcedureListerAdapter{procedureService: procedureService, specialtyRepo: specialtyRepo},
		SpecialtyLister:          &aiAgentSpecialtyListerAdapter{specialtyRepo: specialtyRepo},
		SlotFinder:               &aiAgentSlotFinderAdapter{appointmentService: appointmentService, clinicRepo: clinicRepo, professionalRepo: professionalRepo},
		AppointmentCreator:       &aiAgentAppointmentCreatorAdapter{appointmentService: appointmentService},
		AppointmentFinder:        &aiAgentAppointmentFinderAdapter{appointmentService: appointmentService, patientRepo: patientRepo, appointmentRepo: appointmentRepo, professionalRepo: professionalRepo},
		AppointmentConfirmer:     &aiAgentAppointmentConfirmerAdapter{appointmentService: appointmentService},
		AppointmentCanceller:     &aiAgentAppointmentCancellerAdapter{appointmentService: appointmentService},
		AppointmentRescheduler:   &aiAgentAppointmentReschedulerAdapter{appointmentService: appointmentService, appointmentRepo: appointmentRepo},
		ClinicInfoProvider:       &aiAgentClinicInfoAdapter{clinicService: clinicService},
		ProfessionalInfoProvider: &aiAgentProfessionalInfoAdapter{professionalService: professionalService},
		PatientInfoProvider:      &aiAgentPatientInfoAdapter{patientService: patientService},
		PatientFinderCreator:     &aiAgentPatientFinderCreatorAdapter{patientService: patientService, patientRepo: patientRepo},
		PatientNameUpdater:       &aiAgentPatientNameUpdaterAdapter{patientService: patientService},
		HumanTakeoverMarker:      humanTakeoverMarker,
		GroupSlotLister: &aiAgentGroupSlotListerAdapter{
			sessionPlanRepo:  sessionPlanRepo,
			slotRepo:         sessionPlanSlotRepo,
			enrollmentRepo:   enrollmentRepo,
			professionalRepo: professionalRepo,
		},
		PartnerPlanLister: &aiAgentPartnerPlanListerAdapter{networkRepo: partnerNetworkRepo},
		ClassEnroller:     &aiAgentClassEnrollerAdapter{enrollmentService: enrollmentService, sessionPlanRepo: sessionPlanRepo, partnerNetworkRepo: partnerNetworkRepo},
		ClassConfirmer: &aiAgentClassConfirmerAdapter{
			sessionPlanRepo:     sessionPlanRepo,
			slotRepo:            sessionPlanSlotRepo,
			enrollmentRepo:      enrollmentRepo,
			groupAttendanceRepo: groupAttendanceRepo,
		},
		EnrollmentSummaryPresenter: &aiAgentEnrollmentSummaryPresenterAdapter{
			sessionPlanRepo:    sessionPlanRepo,
			slotRepo:           sessionPlanSlotRepo,
			partnerNetworkRepo: partnerNetworkRepo,
		},
	}

	// 5. Create tool executor
	toolExecutor := tools.NewExecutor(toolRegistry)

	// 6. Create AI agent with dynamic provider selection
	var aiAgent agent.Agent
	var err error

	agentConfig := agent.Config{
		Provider:       workerCfg.AIAgentProvider,
		Model:          workerCfg.AIAgentModel,
		FallbackModels: workerCfg.AIAgentFallbackModels,
		MaxTokens:      workerCfg.AIAgentMaxTokens,
		Temperature:    workerCfg.AIAgentTemperature,
		Timeout:        workerCfg.AIAgentTimeout,
	}

	switch strings.ToLower(workerCfg.AIAgentProvider) {
	case "gemini":
		apiKey := strings.TrimSpace(cfg.GetGeminiAPIKey())
		if apiKey == "" {
			return nil, nil, nil, fmt.Errorf("GEMINI_API_KEY is required for Gemini provider")
		}
		agentConfig.APIKey = apiKey
		aiAgent, err = agent.NewGeminiAgent(agentConfig, toolRegistry, toolExecutor, execContext)

	case "openrouter":
		apiKey := strings.TrimSpace(cfg.GetOpenRouterAPIKey())
		if apiKey == "" {
			return nil, nil, nil, fmt.Errorf("OPENROUTER_API_KEY is required for OpenRouter provider")
		}
		agentConfig.APIKey = apiKey
		aiAgent, err = agent.NewOpenRouterAgent(agentConfig, toolRegistry, toolExecutor, execContext)

	default:
		return nil, nil, nil, fmt.Errorf("unsupported AI provider: %s (supported: gemini, openrouter)", workerCfg.AIAgentProvider)
	}

	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize AI agent (%s): %w", workerCfg.AIAgentProvider, err)
	}

	// 8. Create feature flags
	featureFlags := &agent.FeatureFlags{
		SchedulingEnabled: workerCfg.AIAgentSchedulingEnabled,
		ConfirmEnabled:    workerCfg.AIAgentConfirmEnabled,
		CancelEnabled:     workerCfg.AIAgentCancelEnabled,
		RescheduleEnabled: workerCfg.AIAgentRescheduleEnabled,
		RemindersEnabled:  workerCfg.AIAgentRemindersEnabled,
	}

	// 9. Create AI agent service
	aiTokenRecorder := &aiTokenUsageAdapter{svc: aiTokenUsageSvc}
	agentService := agent.NewService(
		aiAgent,
		conversationMgr,
		toolRegistry,
		toolExecutor,
		execContext,
		featureFlags,
		clinicRepo,
		workerCfg.MinDelayBetweenMessagesSeconds,
		systemLogSvc,
		usageMetricSvc,
		aiTokenRecorder,
	)

	logger.Printf("AI Agent initialized: provider=%s model=%s", workerCfg.AIAgentProvider, workerCfg.AIAgentModel)
	logger.Printf("AI Agent features: scheduling=%v confirm=%v cancel=%v reschedule=%v reminders=%v",
		workerCfg.AIAgentSchedulingEnabled,
		workerCfg.AIAgentConfirmEnabled,
		workerCfg.AIAgentCancelEnabled,
		workerCfg.AIAgentRescheduleEnabled,
		workerCfg.AIAgentRemindersEnabled)

	return agentService, conversationMgr, execContext, nil
}

// AI Agent adapters - wrap existing services to match tool interfaces

type aiAgentProfessionalListerAdapter struct {
	professionalService    professional_service.ProfessionalService
	professionalProcedures professional_repository.ProfessionalProcedureRepository
}

func (a *aiAgentProfessionalListerAdapter) ListAvailable(ctx context.Context, clinicID uuid.UUID, specialty string, procedureID *uuid.UUID) ([]*tools.ProfessionalInfo, error) {
	listResp, err := a.professionalService.List(ctx, clinicID, pagination.Filter{Status: "active", Page: 1, Limit: 100})
	if err != nil {
		return nil, err
	}

	// procedure_id filter: build the set of professional IDs that have an
	// active association with the requested procedure. If empty, the result
	// must be empty too — better to return [] than to mislead the LLM into
	// listing professionals who don't perform that procedure.
	var qualifiedSet map[uuid.UUID]struct{}
	if procedureID != nil && a.professionalProcedures != nil {
		links, lerr := a.professionalProcedures.ListByProcedure(ctx, *procedureID, clinicID)
		if lerr != nil {
			return nil, fmt.Errorf("failed to load professional_procedures for procedure %s: %w", *procedureID, lerr)
		}
		qualifiedSet = make(map[uuid.UUID]struct{}, len(links))
		for _, link := range links {
			if link.IsActive {
				qualifiedSet[link.ProfessionalID] = struct{}{}
			}
		}
	}

	specialtyLower := strings.ToLower(strings.TrimSpace(specialty))

	var result []*tools.ProfessionalInfo
	for _, p := range listResp.Data {
		if qualifiedSet != nil {
			if _, ok := qualifiedSet[p.ID]; !ok {
				continue
			}
		}
		// Exact case-insensitive match against any of the professional's
		// specialties. The AI is expected to resolve approximate terms via
		// list_specialties before calling this tool, so we do not fuzzy-match.
		if specialtyLower != "" {
			matched := false
			for _, sp := range p.Specialties {
				if strings.ToLower(sp.Name) == specialtyLower {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		refs := make([]tools.SpecialtyRef, 0, len(p.Specialties))
		legacy := ""
		for _, sp := range p.Specialties {
			refs = append(refs, tools.SpecialtyRef{ID: sp.ID, Name: sp.Name})
			if legacy == "" {
				legacy = sp.Name
			}
		}
		if legacy == "" {
			legacy = "Geral"
		}

		result = append(result, &tools.ProfessionalInfo{
			ID:           p.ID,
			Name:         p.Name,
			Specialty:    legacy,
			Specialties:  refs,
			WorkingHours: convertWorkingHours(p.WorkingHours),
			IsAvailable:  p.IsActive,
		})
	}

	return result, nil
}

// convertWorkingHours copies a professional model's weekly schedule into the
// tools package's DTO so the AI agent receives the same shape regardless of
// model-layer changes.
func convertWorkingHours(src professional_model.WorkingHours) *tools.WorkingHours {
	toSlots := func(in []professional_model.TimeSlot) []tools.TimeSlot {
		if len(in) == 0 {
			return nil
		}
		out := make([]tools.TimeSlot, len(in))
		for i, s := range in {
			out[i] = tools.TimeSlot{Start: s.Start, End: s.End}
		}
		return out
	}
	wh := &tools.WorkingHours{
		Monday:    toSlots(src.Monday),
		Tuesday:   toSlots(src.Tuesday),
		Wednesday: toSlots(src.Wednesday),
		Thursday:  toSlots(src.Thursday),
		Friday:    toSlots(src.Friday),
		Saturday:  toSlots(src.Saturday),
		Sunday:    toSlots(src.Sunday),
	}
	if wh.Monday == nil && wh.Tuesday == nil && wh.Wednesday == nil &&
		wh.Thursday == nil && wh.Friday == nil && wh.Saturday == nil && wh.Sunday == nil {
		return nil
	}
	return wh
}

type aiAgentSpecialtyListerAdapter struct {
	specialtyRepo specialty_repository.SpecialtyRepository
}

func (a *aiAgentSpecialtyListerAdapter) ListSpecialties(_ context.Context, clinicID uuid.UUID) ([]*tools.SpecialtyRef, error) {
	specs, err := a.specialtyRepo.ListActiveByClinic(clinicID)
	if err != nil {
		return nil, err
	}
	out := make([]*tools.SpecialtyRef, 0, len(specs))
	for _, s := range specs {
		out = append(out, &tools.SpecialtyRef{ID: s.ID, Name: s.Name})
	}
	return out, nil
}

type aiAgentProcedureListerAdapter struct {
	procedureService procedure_service.ProcedureService
	specialtyRepo    specialty_repository.SpecialtyRepository
}

func (a *aiAgentProcedureListerAdapter) ListProcedures(ctx context.Context, clinicID uuid.UUID, specialty string) ([]*tools.ProcedureInfo, error) {
	// Limit aligned with the service's hard cap (100). Previously passed 200,
	// which the service silently shrunk to 10 — items past the 10th in
	// name-ASC order disappeared from the AI's view.
	//
	// Name filtering happens in the executor (Go-side) with Unicode
	// normalization for accent-insensitivity. SQL ILIKE was rejected because
	// it requires the unaccent extension (per-database, would silently break
	// on environments without it).
	listResp, err := a.procedureService.List(ctx, clinicID, pagination.Filter{
		Page:  1,
		Limit: 100,
	})
	if err != nil {
		return nil, err
	}

	// Build specialtyID → name lookup once so we can enrich each procedure
	// without N+1 round trips (the response only carries specialty_id).
	names := map[uuid.UUID]string{}
	if a.specialtyRepo != nil {
		specs, _ := a.specialtyRepo.ListByClinic(clinicID)
		for _, s := range specs {
			names[s.ID] = s.Name
		}
	}

	specialtyLower := strings.ToLower(strings.TrimSpace(specialty))

	var result []*tools.ProcedureInfo
	for _, p := range listResp.Data {
		spName := ""
		if p.SpecialtyID != nil {
			spName = names[*p.SpecialtyID]
		}

		if specialtyLower != "" {
			if strings.ToLower(spName) != specialtyLower {
				continue
			}
		}

		result = append(result, &tools.ProcedureInfo{
			ID:              p.ID,
			Name:            p.Name,
			Price:           p.Price,
			DurationMinutes: p.DurationMinutes,
			SpecialtyName:   spName,
		})
	}

	return result, nil
}

type aiAgentSlotFinderAdapter struct {
	appointmentService appointment_service.AppointmentService
	clinicRepo         clinic_repository.ClinicRepository
	professionalRepo   professional_repository.ProfessionalRepository
}

// checkSubscription verifica se a assinatura da clínica está ativa.
// Se inativa, o agente não deve operar para evitar custos desnecessários.
func (a *aiAgentSlotFinderAdapter) checkSubscription(ctx context.Context, clinicID uuid.UUID) error {
	clinic, err := a.clinicRepo.GetByID(ctx, clinicID)
	if err != nil {
		return fmt.Errorf("failed to get clinic: %w", err)
	}
	if !clinic.IsSubscriptionActive() {
		return fmt.Errorf("clinica com assinatura inativa")
	}
	return nil
}

// clinicDateInLocalTZ normalizes a date to the clinic's configured timezone.
// Working hours are stored as "HH:MM" strings assuming clinic-local time, and
// SuggestSmartSlots builds time.Date using date.Location() — so a UTC date
// here produces slots shifted by the timezone offset (e.g. a BRT 09-18
// schedule shows up as 06-15 BRT). This helper ensures the date's Location
// matches the clinic so slots land on the correct real-world hour.
func (a *aiAgentSlotFinderAdapter) clinicDateInLocalTZ(ctx context.Context, clinicID uuid.UUID, date time.Time) time.Time {
	loc := time.Local
	if clinic, err := a.clinicRepo.GetByID(ctx, clinicID); err == nil && clinic != nil && clinic.Settings.Timezone != "" {
		if parsed, lerr := time.LoadLocation(clinic.Settings.Timezone); lerr == nil {
			loc = parsed
		}
	}
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
}

func (a *aiAgentSlotFinderAdapter) GetAvailableSlots(ctx context.Context, clinicID uuid.UUID, professionalID uuid.UUID, date time.Time, durationMinutes int) ([]*tools.AvailableSlot, error) {
	// INV-001: clinicID sempre presente; verificar assinatura ativa
	if err := a.checkSubscription(ctx, clinicID); err != nil {
		return nil, err
	}

	// Get professional info (INV-001: filtra por clinicID)
	professional, err := a.professionalRepo.GetByID(ctx, professionalID, clinicID)
	if err != nil {
		return nil, fmt.Errorf("failed to get professional: %w", err)
	}

	// Normaliza date pro timezone da clínica: sem isso, slots baseados em
	// working_hours ("09:00"–"18:00" locais) saem 3h deslocados em BRT.
	date = a.clinicDateInLocalTZ(ctx, clinicID, date)

	// Usar SuggestSmartSlots diretamente — sem verificação de AllowOnlineBooking
	slots, err := a.appointmentService.SuggestSmartSlots(ctx, clinicID, professionalID, date, 1, durationMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get available slots: %w", err)
	}

	result := make([]*tools.AvailableSlot, len(slots))
	for i, slot := range slots {
		result[i] = &tools.AvailableSlot{
			ProfessionalID:   professionalID,
			ProfessionalName: professional.Name,
			StartTime:        slot.StartTime,
			EndTime:          slot.EndTime,
		}
	}

	return result, nil
}

func (a *aiAgentSlotFinderAdapter) GetAvailableSlotsMultipleProfessionals(ctx context.Context, clinicID uuid.UUID, professionalIDs []uuid.UUID, date time.Time, days int, durationMinutes int, limit int) ([]*tools.AvailableSlot, error) {
	// INV-001: clinicID sempre presente; verificar assinatura ativa
	if err := a.checkSubscription(ctx, clinicID); err != nil {
		return nil, err
	}

	// Se nenhum profissional especificado, buscar todos ativos da clínica (INV-001)
	if len(professionalIDs) == 0 {
		professionals, _, err := a.professionalRepo.List(ctx, clinicID, pagination.Filter{Status: "active", Limit: 100})
		if err != nil {
			return nil, fmt.Errorf("failed to list professionals: %w", err)
		}
		for _, prof := range professionals {
			professionalIDs = append(professionalIDs, prof.ID)
		}
	}

	// Normaliza date pro timezone da clínica — senão slots saem deslocados.
	date = a.clinicDateInLocalTZ(ctx, clinicID, date)

	// Coletar slots de todos os profissionais nos dias solicitados
	var allSlots []*tools.AvailableSlot
	for day := 0; day < days; day++ {
		currentDate := date.AddDate(0, 0, day)

		for _, profID := range professionalIDs {
			// INV-001: GetByID filtra por clinicID
			professional, err := a.professionalRepo.GetByID(ctx, profID, clinicID)
			if err != nil {
				continue
			}

			// Usar SuggestSmartSlots diretamente — sem AllowOnlineBooking
			slots, err := a.appointmentService.SuggestSmartSlots(ctx, clinicID, profID, currentDate, 1, durationMinutes)
			if err != nil {
				continue
			}

			for _, slot := range slots {
				allSlots = append(allSlots, &tools.AvailableSlot{
					ProfessionalID:   profID,
					ProfessionalName: professional.Name,
					StartTime:        slot.StartTime,
					EndTime:          slot.EndTime,
				})

				if len(allSlots) >= limit {
					return allSlots[:limit], nil
				}
			}
		}
	}

	return allSlots, nil
}

type aiAgentAppointmentCreatorAdapter struct {
	appointmentService appointment_service.AppointmentService
}

func (a *aiAgentAppointmentCreatorAdapter) CreateFromSchedulingIntent(ctx context.Context, clinicID, patientID, professionalID uuid.UUID, startTime time.Time, durationMinutes int) (*tools.AppointmentInfo, error) {
	req := &appointment_model.CreateAppointmentRequest{
		ProfessionalID:  professionalID,
		PatientID:       patientID,
		StartTime:       startTime,
		DurationMinutes: durationMinutes,
		Notes:           "Agendado via WhatsApp AI",
	}

	apt, err := a.appointmentService.Create(ctx, clinicID, req)
	if err != nil {
		return nil, err
	}

	// Auto-confirm appointments for next day or later
	// so the patient doesn't receive an unnecessary confirmation message
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	now := time.Now().In(loc)
	aptDate := startTime.In(loc)
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
	if !aptDate.Before(tomorrow) {
		_, confirmErr := a.appointmentService.Confirm(ctx, apt.ID, clinicID, &appointment_model.ConfirmAppointmentRequest{
			PatientNotes: "Confirmado automaticamente - agendamento via IA",
		})
		if confirmErr != nil {
			logger.Printf("[AI Agent] Failed to auto-confirm appointment %s: %v", apt.ID, confirmErr)
		} else {
			apt.Status = "confirmed"
		}
	}

	return &tools.AppointmentInfo{
		ID:             apt.ID,
		PatientID:      apt.PatientID,
		ProfessionalID: apt.ProfessionalID,
		StartTime:      apt.StartTime,
		EndTime:        apt.EndTime,
		Status:         string(apt.Status),
	}, nil
}

type aiAgentAppointmentFinderAdapter struct {
	appointmentService appointment_service.AppointmentService
	patientRepo        patient_repository.PatientRepository
	appointmentRepo    appointment_repository.AppointmentRepository
	professionalRepo   professional_repository.ProfessionalRepository
}

// GetByPatientPhone resolves the patient by phone, then returns the recent
// appointments for that patient filtered by status. Replaces the previous
// stub that always returned empty — which made the confirm/cancel-by-phone
// flow report "no pending appointments" for every patient.
func (a *aiAgentAppointmentFinderAdapter) GetByPatientPhone(ctx context.Context, clinicID uuid.UUID, phone string, status string, limit int) ([]*tools.AppointmentInfo, error) {
	if a.patientRepo == nil || a.appointmentRepo == nil {
		return []*tools.AppointmentInfo{}, nil
	}
	patient, err := a.patientRepo.GetByPhone(ctx, phone, clinicID)
	if err != nil || patient == nil {
		return []*tools.AppointmentInfo{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	apts, err := a.appointmentRepo.GetByPatient(ctx, patient.ID, clinicID, limit)
	if err != nil {
		return nil, err
	}
	// Cache professional names to avoid N queries when listing many apts.
	profNames := map[uuid.UUID]string{}
	out := make([]*tools.AppointmentInfo, 0, len(apts))
	for _, apt := range apts {
		if status != "" && string(apt.Status) != status {
			continue
		}
		profName := profNames[apt.ProfessionalID]
		if profName == "" && a.professionalRepo != nil {
			if p, perr := a.professionalRepo.GetByID(ctx, apt.ProfessionalID, clinicID); perr == nil && p != nil {
				profName = p.Name
				profNames[apt.ProfessionalID] = profName
			}
		}
		out = append(out, &tools.AppointmentInfo{
			ID:             apt.ID,
			PatientID:      apt.PatientID,
			ProfessionalID: apt.ProfessionalID,
			Professional:   profName,
			StartTime:      apt.StartTime,
			EndTime:        apt.EndTime,
			Status:         string(apt.Status),
		})
	}
	return out, nil
}

func (a *aiAgentAppointmentFinderAdapter) GetByID(ctx context.Context, appointmentID, clinicID uuid.UUID) (*tools.AppointmentInfo, error) {
	apt, err := a.appointmentService.GetByID(ctx, appointmentID, clinicID)
	if err != nil {
		return nil, err
	}

	return &tools.AppointmentInfo{
		ID:             apt.ID,
		PatientID:      apt.PatientID,
		ProfessionalID: apt.ProfessionalID,
		StartTime:      apt.StartTime,
		EndTime:        apt.EndTime,
		Status:         string(apt.Status),
	}, nil
}

type aiAgentAppointmentConfirmerAdapter struct {
	appointmentService appointment_service.AppointmentService
}

func (a *aiAgentAppointmentConfirmerAdapter) ConfirmByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, patientNotes string) error {
	_, err := a.appointmentService.Confirm(ctx, appointmentID, clinicID, &appointment_model.ConfirmAppointmentRequest{
		PatientNotes: patientNotes,
	})
	return err
}

type aiAgentAppointmentCancellerAdapter struct {
	appointmentService appointment_service.AppointmentService
}

func (a *aiAgentAppointmentCancellerAdapter) CancelByPatient(ctx context.Context, appointmentID, clinicID uuid.UUID, reason string) error {
	_, err := a.appointmentService.Cancel(ctx, appointmentID, clinicID, &appointment_model.CancelAppointmentRequest{
		Reason: reason,
	})
	return err
}

type aiAgentAppointmentReschedulerAdapter struct {
	appointmentService appointment_service.AppointmentService
	appointmentRepo    appointment_repository.AppointmentRepository
}

// Reschedule wires the tool layer to the real appointment_service.Reschedule.
// When newProfessionalID is nil we keep the current one (patient wants only a
// new time). duration is derived from the stored End-Start diff, falling back
// to 60 min if unavailable. Replaces the previous stub that returned
// "reschedule not yet implemented" — which made the AI surface that literal
// error to patients.
func (a *aiAgentAppointmentReschedulerAdapter) Reschedule(ctx context.Context, appointmentID, clinicID uuid.UUID, newStartTime time.Time, newProfessionalID *uuid.UUID) error {
	if a.appointmentService == nil {
		return fmt.Errorf("appointment service not configured")
	}

	professionalID := uuid.Nil
	durationMinutes := 60

	if a.appointmentRepo != nil {
		if apt, err := a.appointmentRepo.GetByID(ctx, appointmentID, clinicID); err == nil && apt != nil {
			professionalID = apt.ProfessionalID
			if !apt.EndTime.IsZero() && !apt.StartTime.IsZero() {
				if d := int(apt.EndTime.Sub(apt.StartTime).Minutes()); d > 0 {
					durationMinutes = d
				}
			}
		}
	}
	if newProfessionalID != nil {
		professionalID = *newProfessionalID
	}
	if professionalID == uuid.Nil {
		return fmt.Errorf("could not resolve professional for reschedule")
	}

	return a.appointmentService.Reschedule(ctx, appointmentID, clinicID, professionalID, newStartTime, durationMinutes)
}

type aiAgentClinicInfoAdapter struct {
	clinicService clinic_service.ClinicService
}

func (a *aiAgentClinicInfoAdapter) GetClinicInfo(ctx context.Context, clinicID uuid.UUID) (*tools.ClinicInfo, error) {
	clinic, err := a.clinicService.GetByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	return &tools.ClinicInfo{
		Name:          clinic.Name,
		Address:       clinic.Address,
		Phone:         clinic.Phone,
		BusinessHours: "Segunda a Sexta, 8h às 18h", // TODO: Get from clinic settings
	}, nil
}

type aiAgentProfessionalInfoAdapter struct {
	professionalService professional_service.ProfessionalService
}

func (a *aiAgentProfessionalInfoAdapter) GetProfessionalInfo(ctx context.Context, professionalID uuid.UUID) (*tools.ProfessionalDetailedInfo, error) {
	// This would need GetByID in professional service
	// For now, return basic info
	return &tools.ProfessionalDetailedInfo{
		ID:          professionalID,
		Name:        "Professional", // TODO: Get from service
		Specialty:   "Unknown",
		Description: "",
		IsAvailable: true,
	}, nil
}

type aiAgentPatientInfoAdapter struct {
	patientService patient_service.PatientService
}

func (a *aiAgentPatientInfoAdapter) GetPatientInfo(ctx context.Context, patientID uuid.UUID) (*tools.PatientInfo, error) {
	// Note: We need clinicID but interface doesn't provide it
	// This is a limitation - for now we'll need to handle this differently
	// The executor should pass clinicID through ExecutionContext
	return nil, fmt.Errorf("GetPatientInfo needs clinicID from ExecutionContext - not yet implemented")
}

type aiAgentPatientFinderCreatorAdapter struct {
	patientService patient_service.PatientService
	patientRepo    patient_repository.PatientRepository
}

func (a *aiAgentPatientFinderCreatorAdapter) FindOrCreateByPhone(ctx context.Context, clinicID uuid.UUID, phone, name string) (*tools.PatientInfo, error) {
	// Try to find existing patient by phone
	patient, err := a.patientRepo.GetByPhone(ctx, phone, clinicID)
	if err == nil {
		// Patient found
		return &tools.PatientInfo{
			ID:    patient.ID,
			Name:  patient.Name,
			Phone: patient.Phone,
			Email: patient.Email,
		}, nil
	}

	// Patient not found, create new one
	req := &patient_model.CreatePatientRequest{
		Name:  name,
		Phone: phone,
	}

	newPatient, err := a.patientService.Create(ctx, clinicID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create patient: %w", err)
	}

	return &tools.PatientInfo{
		ID:    newPatient.ID,
		Name:  newPatient.Name,
		Phone: newPatient.Phone,
		Email: newPatient.Email,
	}, nil
}

type aiAgentPatientNameUpdaterAdapter struct {
	patientService patient_service.PatientService
}

func (a *aiAgentPatientNameUpdaterAdapter) UpdateName(ctx context.Context, clinicID, patientID uuid.UUID, newName string) error {
	_, err := a.patientService.Update(ctx, patientID, clinicID, &patient_model.UpdatePatientRequest{
		Name: newName,
	})
	return err
}

// ---------------------------------------------------------------------------
// Group session plan adapters — Pilates / aulas em grupo
// ---------------------------------------------------------------------------

// weekdayPT maps Sunday..Saturday (0..6) to the Portuguese label the AI
// already uses elsewhere when describing slots to patients.
var weekdayPTLabels = [7]string{"Domingo", "Segunda", "Terça", "Quarta", "Quinta", "Sexta", "Sábado"}

type aiAgentGroupSlotListerAdapter struct {
	sessionPlanRepo  sessionplan_repository.SessionPlanRepository
	slotRepo         sessionplan_repository.SessionPlanSlotRepository
	enrollmentRepo   enrollment_repository.EnrollmentRepository
	professionalRepo professional_repository.ProfessionalRepository
}

func (a *aiAgentGroupSlotListerAdapter) ListGroupSlots(
	ctx context.Context,
	clinicID uuid.UUID,
	filter tools.GroupSlotFilter,
) ([]*tools.GroupPlanInfo, error) {
	plans, err := a.sessionPlanRepo.GetAllActive(ctx, clinicID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active group plans: %w", err)
	}

	// Cache professional names so the same instructor cited on N slots costs
	// at most one DB lookup. Maps nil/empty to nil so the AI sees null when
	// the slot has no fixed instructor (rotating schedule).
	profNames := map[uuid.UUID]string{}

	out := make([]*tools.GroupPlanInfo, 0, len(plans))
	for _, plan := range plans {
		if plan.Type != sessionplan_model.PlanTypeGroup {
			continue
		}
		if filter.ProcedureID != nil {
			if plan.ProcedureID == nil || *plan.ProcedureID != *filter.ProcedureID {
				continue
			}
		}

		slots, err := a.slotRepo.ListActiveByPlan(ctx, plan.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list active slots for plan %s: %w", plan.ID, err)
		}
		if len(slots) == 0 {
			continue
		}

		// Active enrollments are loaded once per plan; capacity = slot.capacity
		// minus enrollments that include the slot's weekday. Mirrors the
		// calendar facade's logic so the AI shows the same "available" count
		// the dashboard does.
		enrollments, err := a.enrollmentRepo.GetActiveByPlan(ctx, clinicID, plan.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to count enrollments for plan %s: %w", plan.ID, err)
		}

		slotInfos := make([]tools.GroupSlotInfo, 0, len(slots))
		for _, s := range slots {
			if filter.Weekday != nil && s.Weekday != *filter.Weekday {
				continue
			}
			if filter.TimeFilter != "" && s.StartTime != filter.TimeFilter {
				continue
			}

			enrolled := 0
			for _, e := range enrollments {
				if e.HasWeekDay(s.Weekday) {
					enrolled++
				}
			}
			available := s.Capacity - enrolled
			if available < 0 {
				available = 0
			}

			var profName *string
			if s.ProfessionalID != nil {
				name, ok := profNames[*s.ProfessionalID]
				if !ok {
					prof, err := a.professionalRepo.GetByID(ctx, *s.ProfessionalID, clinicID)
					if err == nil && prof != nil {
						name = prof.Name
					}
					profNames[*s.ProfessionalID] = name
				}
				if name != "" {
					profName = &name
				}
			}

			weekday := s.Weekday
			weekdayLabel := ""
			if weekday >= 0 && weekday < 7 {
				weekdayLabel = weekdayPTLabels[weekday]
			}

			slotInfos = append(slotInfos, tools.GroupSlotInfo{
				SlotID:           s.ID,
				Weekday:          weekday,
				WeekdayPT:        weekdayLabel,
				StartTime:        s.StartTime,
				Capacity:         s.Capacity,
				Enrolled:         enrolled,
				Available:        available,
				ProfessionalName: profName,
			})
		}

		if len(slotInfos) == 0 {
			continue
		}

		out = append(out, &tools.GroupPlanInfo{
			PlanID:    plan.ID,
			PlanTitle: plan.Title,
			Slots:     slotInfos,
		})
	}

	return out, nil
}

type aiAgentPartnerPlanListerAdapter struct {
	networkRepo partnernetwork_repository.PartnerNetworkRepository
}

func (a *aiAgentPartnerPlanListerAdapter) ListPartnerPlans(_ context.Context, clinicID uuid.UUID) ([]*tools.PartnerPlanInfo, error) {
	networks, err := a.networkRepo.ListActiveByClinic(clinicID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active partner networks: %w", err)
	}
	out := make([]*tools.PartnerPlanInfo, 0)
	for _, n := range networks {
		for _, p := range n.Plans {
			if !p.IsActive {
				continue
			}
			info := &tools.PartnerPlanInfo{
				ID:          p.ID,
				NetworkName: n.Name,
				Name:        p.Name,
			}
			if p.MaxSessionsPerWeek != nil {
				v := *p.MaxSessionsPerWeek
				info.MaxSessionsPerWeek = &v
			}
			out = append(out, info)
		}
	}
	return out, nil
}

type aiAgentClassEnrollerAdapter struct {
	enrollmentService  enrollment_service.EnrollmentService
	sessionPlanRepo    sessionplan_repository.SessionPlanRepository
	partnerNetworkRepo partnernetwork_repository.PartnerNetworkRepository
}

func (a *aiAgentClassEnrollerAdapter) Enroll(
	ctx context.Context,
	clinicID, patientID, planID uuid.UUID,
	slotIDs []uuid.UUID,
	partnerPlanID *uuid.UUID,
	monthlyAmount *float64,
	billingDay *int,
) (*tools.ClassEnrollmentInfo, error) {
	// PaymentMode follows the rule the existing form enforces: if there is a
	// partner plan, it is "per_session" (the partner is billed per
	// attendance); if monthly_amount is set, it is "monthly"; otherwise
	// per_session. billing_day is only required for monthly.
	paymentMode := enrollment_model.PaymentModePerSession
	if partnerPlanID == nil && monthlyAmount != nil {
		paymentMode = enrollment_model.PaymentModeMonthly
	}

	req := &enrollment_model.EnrollRequest{
		PatientID:     patientID,
		SlotIDs:       slotIDs,
		PartnerPlanID: partnerPlanID,
		PaymentMode:   paymentMode,
		MonthlyAmount: monthlyAmount,
		BillingDay:    billingDay,
	}
	enrollment, err := a.enrollmentService.Enroll(ctx, clinicID, planID, req)
	if err != nil {
		return nil, err
	}

	// Resolve plan title for the LLM's confirmation message. Best-effort —
	// missing title still returns the rest of the enrollment data.
	planTitle := ""
	if plan, err := a.sessionPlanRepo.GetByID(ctx, clinicID, planID); err == nil && plan != nil {
		planTitle = plan.Title
	}

	// Resolve partner plan label "Network — Plan" so the AI can confirm it
	// verbatim without looking it up again.
	partnerLabel := ""
	if partnerPlanID != nil && a.partnerNetworkRepo != nil {
		networks, err := a.partnerNetworkRepo.ListActiveByClinic(clinicID)
		if err == nil {
			for _, n := range networks {
				for _, p := range n.Plans {
					if p.ID == *partnerPlanID {
						partnerLabel = fmt.Sprintf("%s — %s", n.Name, p.Name)
						break
					}
				}
				if partnerLabel != "" {
					break
				}
			}
		}
	}

	weekdays := make([]int, len(enrollment.WeekDays))
	for i, d := range enrollment.WeekDays {
		weekdays[i] = int(d)
	}

	return &tools.ClassEnrollmentInfo{
		EnrollmentID:  enrollment.ID,
		PlanTitle:     planTitle,
		Weekdays:      weekdays,
		MonthlyAmount: enrollment.MonthlyAmount,
		PartnerPlan:   partnerLabel,
		Status:        string(enrollment.Status),
	}, nil
}

// aiAgentClassConfirmerAdapter resolves the patient's next upcoming group
// class (across active enrollments and their slots) and idempotently writes
// a GroupSessionAttendance row with status="confirmed". It does NOT
// increment sessions_done — that only happens when the operator (or a
// future check-in flow) flips the status to "present" on class day.
type aiAgentClassConfirmerAdapter struct {
	sessionPlanRepo     sessionplan_repository.SessionPlanRepository
	slotRepo            sessionplan_repository.SessionPlanSlotRepository
	enrollmentRepo      enrollment_repository.EnrollmentRepository
	groupAttendanceRepo attendance_repository.GroupAttendanceRepository
}

func (a *aiAgentClassConfirmerAdapter) Confirm(
	ctx context.Context,
	clinicID, patientID uuid.UUID,
) (*tools.ClassConfirmationInfo, error) {
	plans, err := a.sessionPlanRepo.GetAllActive(ctx, clinicID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active group plans: %w", err)
	}

	type candidate struct {
		plan         *sessionplan_model.SessionPlan
		enrollmentID uuid.UUID
		startTime    string
		dt           time.Time
	}
	now := time.Now()
	var best *candidate

	for _, plan := range plans {
		if plan.Type != sessionplan_model.PlanTypeGroup {
			continue
		}
		enrollment, err := a.enrollmentRepo.GetByPatientAndPlan(ctx, clinicID, patientID, plan.ID)
		if err != nil || enrollment == nil {
			continue
		}
		if !enrollment.IsActive() {
			continue
		}

		// Slot-based path (modern). Each EnrollmentSlot points at a
		// SessionPlanSlot — load it for weekday + start_time.
		if len(enrollment.Slots) > 0 {
			for _, junction := range enrollment.Slots {
				slot, err := a.slotRepo.GetByID(ctx, junction.SessionPlanSlotID)
				if err != nil || slot == nil || !slot.IsActive {
					continue
				}
				dt := nextOccurrence(now, slot.Weekday, slot.StartTime)
				if dt.IsZero() {
					continue
				}
				if best == nil || dt.Before(best.dt) {
					best = &candidate{plan: plan, enrollmentID: enrollment.ID, startTime: slot.StartTime, dt: dt}
				}
			}
			continue
		}

		// Legacy path: enrollment.WeekDays + plan.PreferredTime (or first
		// DayTimes entry). New enrollments populate Slots, but old data
		// may still flow through here.
		if len(enrollment.WeekDays) == 0 {
			continue
		}
		fallbackTime := plan.PreferredTime
		if fallbackTime == "" {
			for _, t := range plan.DayTimes {
				fallbackTime = t
				break
			}
		}
		if fallbackTime == "" {
			fallbackTime = "08:00"
		}
		for _, wd := range enrollment.WeekDays {
			dt := nextOccurrence(now, int(wd), fallbackTime)
			if dt.IsZero() {
				continue
			}
			if best == nil || dt.Before(best.dt) {
				best = &candidate{plan: plan, enrollmentID: enrollment.ID, startTime: fallbackTime, dt: dt}
			}
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no upcoming class found — patient has no active group enrollment")
	}

	dateStr := best.dt.Format("2006-01-02")

	// Idempotent upsert: if a row already exists for (plan, patient, date),
	// reuse it. Don't downgrade Present/Absent back to Confirmed (the class
	// already happened from the operator's POV); just return the current
	// state so the AI can tell the patient.
	existing, err := a.groupAttendanceRepo.GetByPlanDateAndPatient(ctx, clinicID, best.plan.ID, patientID, dateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to look up existing attendance: %w", err)
	}
	if existing != nil {
		// First run already created Confirmed; or operator marked something.
		// Return as-is — never override operator state.
		return &tools.ClassConfirmationInfo{
			AttendanceID: existing.ID,
			PlanTitle:    best.plan.Title,
			SessionDate:  dateStr,
			StartTime:    best.startTime,
			Status:       string(existing.Status),
		}, nil
	}

	enrollmentID := best.enrollmentID
	record := &attendance_model.GroupSessionAttendance{
		ClinicID:      clinicID,
		SessionPlanID: best.plan.ID,
		SessionDate:   dateStr,
		PatientID:     patientID,
		EnrollmentID:  &enrollmentID,
		Status:        attendance_model.AttendanceStatusConfirmed,
	}
	if err := a.groupAttendanceRepo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to create confirmation record: %w", err)
	}
	return &tools.ClassConfirmationInfo{
		AttendanceID: record.ID,
		PlanTitle:    best.plan.Title,
		SessionDate:  dateStr,
		StartTime:    best.startTime,
		Status:       string(record.Status),
	}, nil
}

// groupClassAttendanceCheckerAdapter wraps the existing GroupAttendanceRepository
// so the class reminder worker can check whether the patient already engaged
// with the upcoming class (any status — confirmed, present, absent,
// substituted). When a row exists we skip the reminder to avoid re-prompting.
type groupClassAttendanceCheckerAdapter struct {
	repo attendance_repository.GroupAttendanceRepository
}

func (a *groupClassAttendanceCheckerAdapter) HasAttendanceRow(
	ctx context.Context,
	clinicID, planID, patientID uuid.UUID,
	sessionDate string,
) (bool, error) {
	row, err := a.repo.GetByPlanDateAndPatient(ctx, clinicID, planID, patientID, sessionDate)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

// nextOccurrence returns the next datetime (>= now) for the given weekday
// (0=Sun..6=Sat) and HH:MM start time. Today counts only if start_time is
// still in the future. Returns zero time if start_time is unparseable.
func nextOccurrence(now time.Time, weekday int, startTime string) time.Time {
	if len(startTime) != 5 || startTime[2] != ':' {
		return time.Time{}
	}
	var h, m int
	if _, err := fmt.Sscanf(startTime, "%d:%d", &h, &m); err != nil {
		return time.Time{}
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}
	}
	loc := now.Location()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, loc)
	dayDiff := (weekday - int(now.Weekday()) + 7) % 7
	if dayDiff == 0 && !candidate.After(now) {
		dayDiff = 7
	}
	return candidate.AddDate(0, 0, dayDiff)
}

// aiAgentEnrollmentSummaryPresenterAdapter validates the slot picks against
// the database and builds the patient-facing matrícula confirmation message
// server-side. Closes the hallucination vector where the LLM would compose
// the resumé as free-text and invent slot picks — every slot_id passed
// here is checked for existence + plan match + active state, partner_plan
// is resolved to its labels (Network — Plan), and the markdown is built
// deterministically. The LLM's job is to echo the returned summary_text
// verbatim (no rewording).
type aiAgentEnrollmentSummaryPresenterAdapter struct {
	sessionPlanRepo    sessionplan_repository.SessionPlanRepository
	slotRepo           sessionplan_repository.SessionPlanSlotRepository
	partnerNetworkRepo partnernetwork_repository.PartnerNetworkRepository
}

func (a *aiAgentEnrollmentSummaryPresenterAdapter) PresentSummary(
	ctx context.Context,
	clinicID, planID uuid.UUID,
	slotIDs []uuid.UUID,
	partnerPlanID *uuid.UUID,
	monthlyAmount *float64,
	billingDay *int,
) (*tools.EnrollmentSummaryInfo, error) {
	plan, err := a.sessionPlanRepo.GetByID(ctx, clinicID, planID)
	if err != nil || plan == nil {
		return nil, fmt.Errorf("plan not found in this clinic — did you call get_group_class_slots first?")
	}

	type validatedSlot struct {
		id        uuid.UUID
		weekday   int
		startTime string
	}

	dedup := make(map[uuid.UUID]bool, len(slotIDs))
	validated := make([]validatedSlot, 0, len(slotIDs))
	for _, sid := range slotIDs {
		if dedup[sid] {
			continue
		}
		dedup[sid] = true
		slot, err := a.slotRepo.GetByIDScoped(ctx, sid, clinicID)
		if err != nil || slot == nil {
			return nil, fmt.Errorf("slot %s not found in this clinic — you may have invented this slot_id; go back and ask the patient which days/times again", sid)
		}
		if slot.SessionPlanID != planID {
			return nil, fmt.Errorf("slot %s does not belong to plan %s", sid, planID)
		}
		if !slot.IsActive {
			return nil, fmt.Errorf("slot %s is inactive — pick another", sid)
		}
		validated = append(validated, validatedSlot{
			id:        slot.ID,
			weekday:   slot.Weekday,
			startTime: slot.StartTime,
		})
	}

	// Sort by weekday then start_time so the summary is readable regardless
	// of the order the LLM passed slot_ids in.
	sort.Slice(validated, func(i, j int) bool {
		if validated[i].weekday != validated[j].weekday {
			return validated[i].weekday < validated[j].weekday
		}
		return validated[i].startTime < validated[j].startTime
	})

	// Format slots line: "Segunda 09:00, Quarta 09:00, Sexta 09:00"
	parts := make([]string, len(validated))
	for i, v := range validated {
		label := ""
		if v.weekday >= 0 && v.weekday < 7 {
			label = weekdayPTLabels[v.weekday]
		}
		parts[i] = fmt.Sprintf("%s %s", label, v.startTime)
	}
	horariosLine := strings.Join(parts, ", ")

	// Convênio label: "Gympass — Silver" or "Particular".
	convenioLabel := "Particular"
	if partnerPlanID != nil {
		networks, err := a.partnerNetworkRepo.ListActiveByClinic(clinicID)
		if err == nil {
		outer:
			for _, n := range networks {
				for _, p := range n.Plans {
					if p.ID == *partnerPlanID {
						convenioLabel = fmt.Sprintf("%s — %s", n.Name, p.Name)
						break outer
					}
				}
			}
		}
		if convenioLabel == "Particular" {
			// partner_plan_id was given but didn't match — refuse to build
			// a summary with stale/wrong convênio; force the LLM to recheck.
			return nil, fmt.Errorf("partner_plan_id %s not found in this clinic — confirm the convênio name with the patient", *partnerPlanID)
		}
	}

	// Mensalidade label: convênio cobre, ou valor formatado em R$.
	mensalidadeLabel := ""
	if partnerPlanID != nil {
		mensalidadeLabel = fmt.Sprintf("Coberta pelo convênio (%s)", convenioLabel)
	} else if monthlyAmount != nil && *monthlyAmount > 0 {
		mensalidadeLabel = fmt.Sprintf("R$ %s", formatBRL(*monthlyAmount))
	} else {
		mensalidadeLabel = "A confirmar com a clínica"
	}

	frequencia := fmt.Sprintf("%dx/semana", len(validated))

	summaryText := fmt.Sprintf(
		"Antes de finalizar, confirma a matrícula?\n\n"+
			"*Modalidade:* %s\n"+
			"*Frequência:* %s\n"+
			"*Horários:* %s\n"+
			"*Convênio:* %s\n"+
			"*Mensalidade:* %s\n\n"+
			"Posso confirmar?",
		plan.Title, frequencia, horariosLine, convenioLabel, mensalidadeLabel,
	)

	out := &tools.EnrollmentSummaryInfo{
		SummaryText: summaryText,
		PlanID:      planID,
		PlanTitle:   plan.Title,
		SlotIDs:     make([]uuid.UUID, 0, len(validated)),
	}
	for _, v := range validated {
		out.SlotIDs = append(out.SlotIDs, v.id)
	}
	// billingDay is intentionally omitted from the summary text — operators
	// store it on the enrollment record, but the patient doesn't need to
	// see it on the WhatsApp message. Keeping it noise-free.
	_ = billingDay
	return out, nil
}

// aiAgentPatientContextFetcherAdapter pre-fetches what the LLM needs to
// interpret short replies. Three sources:
//
//  1. Next pending individual appointment from the appointments table.
//     Limit 1 — weekly individual plans otherwise show appointments 3
//     weeks out as "next", which is noise.
//
//  2. Active SessionPlans owned by the patient with type=individual.
//     Each weekly occurrence is virtual until materialized; the next
//     class is computed in code from WeekDays + PreferredTime/DayTimes.
//
//  3. Active SessionPlanEnrollments (group classes — Pilates etc).
//     Each enrollment carries its slot grid; next class computed from
//     the slot's weekday + start_time.
//
// All three are best-effort: per-source errors are swallowed and the
// other sources still surface in the prompt. A patient with absolutely
// nothing returns an empty (but non-nil) PatientContext — that's the
// signal for "treat short replies as ambiguous".
type aiAgentPatientContextFetcherAdapter struct {
	appointmentRepo  appointment_repository.AppointmentRepository
	professionalRepo professional_repository.ProfessionalRepository
	sessionPlanRepo  sessionplan_repository.SessionPlanRepository
	slotRepo         sessionplan_repository.SessionPlanSlotRepository
	enrollmentRepo   enrollment_repository.EnrollmentRepository
}

func (a *aiAgentPatientContextFetcherAdapter) Fetch(
	ctx context.Context,
	clinicID, patientID uuid.UUID,
) (*agent.PatientContext, error) {
	out := &agent.PatientContext{}
	now := time.Now()

	// 1) Next pending individual appointment (materialized in DB).
	// GetByPatient is sorted by start_time ASC, so the first row in the
	// future is the next one. Filter to pending/confirmed and future.
	appts, _ := a.appointmentRepo.GetByPatient(ctx, patientID, clinicID, 10)
	for _, appt := range appts {
		if appt.StartTime.Before(now) {
			continue
		}
		// Only "actionable" statuses — past patient may have appointments
		// completed/canceled in the list. We want pending or confirmed.
		st := string(appt.Status)
		if st != "pending" && st != "confirmed" && st != "no_response" {
			continue
		}
		profName := ""
		if prof, err := a.professionalRepo.GetByID(ctx, appt.ProfessionalID, clinicID); err == nil && prof != nil {
			profName = prof.Name
		}
		out.NextAppointment = &agent.PatientNextAppointment{
			StartTime:        appt.StartTime,
			ProfessionalName: profName,
		}
		break // first future actionable appointment wins
	}

	// 2) Active recurring plans owned by this patient (individual).
	individualPlans, _ := a.sessionPlanRepo.GetByPatient(ctx, clinicID, patientID)
	for _, plan := range individualPlans {
		if plan.Status != sessionplan_model.PlanStatusActive {
			continue
		}
		if plan.Type != sessionplan_model.PlanTypeIndividual {
			continue
		}
		ap := agent.PatientActivePlan{
			Type:      "individual",
			PlanTitle: planDisplayTitle(plan),
		}
		// Weekday list: prefer DayTimes (legacy per-day-time map),
		// otherwise fall back to plain WeekDays + PreferredTime.
		ap.Slots = individualPlanSlots(plan)
		ap.NextClass = nextOccurrenceFromSlots(now, ap.Slots)
		out.ActivePlans = append(out.ActivePlans, ap)
	}

	// 3) Active group enrollments.
	enrollments, _ := a.enrollmentRepo.GetActiveByPatient(ctx, clinicID, patientID)
	for _, enr := range enrollments {
		plan, err := a.sessionPlanRepo.GetByID(ctx, clinicID, enr.SessionPlanID)
		if err != nil || plan == nil {
			continue
		}
		ap := agent.PatientActivePlan{
			Type:      "group",
			PlanTitle: planDisplayTitle(plan),
		}
		// Resolve the enrollment's slots into the preview shape.
		for _, junction := range enr.Slots {
			slot, err := a.slotRepo.GetByID(ctx, junction.SessionPlanSlotID)
			if err != nil || slot == nil || !slot.IsActive {
				continue
			}
			ap.Slots = append(ap.Slots, agent.PlanSlotPreview{
				Weekday:   slot.Weekday,
				StartTime: slot.StartTime,
			})
		}
		ap.NextClass = nextOccurrenceFromSlots(now, ap.Slots)
		out.ActivePlans = append(out.ActivePlans, ap)
	}

	return out, nil
}

// planDisplayTitle picks the most useful title for a session plan. Falls
// back to procedure name when title is empty (legacy data).
func planDisplayTitle(plan *sessionplan_model.SessionPlan) string {
	if plan.Title != "" {
		return plan.Title
	}
	if plan.ProcedureName != "" {
		return plan.ProcedureName
	}
	return "Plano"
}

// individualPlanSlots derives the weekly slot grid from an individual
// SessionPlan. Tries DayTimes (most accurate — different times per day),
// then WeekDays + PreferredTime.
func individualPlanSlots(plan *sessionplan_model.SessionPlan) []agent.PlanSlotPreview {
	out := []agent.PlanSlotPreview{}

	// DayTimes is map "MO" -> "08:00".
	if len(plan.DayTimes) > 0 {
		for code, t := range plan.DayTimes {
			wd := byDayCodeToWeekday(code)
			if wd < 0 {
				continue
			}
			out = append(out, agent.PlanSlotPreview{Weekday: wd, StartTime: t})
		}
		return out
	}

	// Legacy: WeekDays array + PreferredTime.
	t := plan.PreferredTime
	if t == "" {
		t = "08:00"
	}
	for _, wd := range plan.WeekDays {
		out = append(out, agent.PlanSlotPreview{Weekday: int(wd), StartTime: t})
	}
	return out
}

// byDayCodeToWeekday converts a RFC 5545 BYDAY code (MO, TU, etc) into
// time.Weekday integer (0=Sun..6=Sat). Returns -1 on unknown.
func byDayCodeToWeekday(code string) int {
	switch strings.ToUpper(code) {
	case "SU":
		return 0
	case "MO":
		return 1
	case "TU":
		return 2
	case "WE":
		return 3
	case "TH":
		return 4
	case "FR":
		return 5
	case "SA":
		return 6
	}
	return -1
}

// nextOccurrenceFromSlots returns the soonest next occurrence across
// all slots. Returns nil when slots is empty or all are unparseable.
func nextOccurrenceFromSlots(now time.Time, slots []agent.PlanSlotPreview) *time.Time {
	var best *time.Time
	for _, s := range slots {
		dt := nextOccurrence(now, s.Weekday, s.StartTime)
		if dt.IsZero() {
			continue
		}
		if best == nil || dt.Before(*best) {
			d := dt
			best = &d
		}
	}
	return best
}

// formatBRL formats a float as Brazilian-style "1.234,56" (thousand
// separator dot, decimal comma). Hand-rolled because the formatter we'd
// reach for (golang.org/x/text/message) is overkill for a single locale.
func formatBRL(v float64) string {
	cents := int64(v*100 + 0.5)
	whole := cents / 100
	frac := cents % 100
	wholeStr := fmt.Sprintf("%d", whole)
	// Insert dots as thousand separators.
	if len(wholeStr) > 3 {
		var b strings.Builder
		for i, c := range wholeStr {
			if i > 0 && (len(wholeStr)-i)%3 == 0 {
				b.WriteByte('.')
			}
			b.WriteRune(c)
		}
		wholeStr = b.String()
	}
	return fmt.Sprintf("%s,%02d", wholeStr, frac)
}
