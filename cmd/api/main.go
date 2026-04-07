package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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
	analytics_handler "github.com/aseno/backend/pkg/analytics/handler"
	analytics_repository "github.com/aseno/backend/pkg/analytics/repository"
	analytics_service "github.com/aseno/backend/pkg/analytics/service"
	appointment_handler "github.com/aseno/backend/pkg/appointment/handler"
	appointment_model "github.com/aseno/backend/pkg/appointment/model"
	appointment_repository "github.com/aseno/backend/pkg/appointment/repository"
	appointment_service "github.com/aseno/backend/pkg/appointment/service"
	auth_google "github.com/aseno/backend/pkg/auth/client/google"
	auth_handler "github.com/aseno/backend/pkg/auth/handler"
	auth_middleware "github.com/aseno/backend/pkg/auth/middleware"
	auth_model "github.com/aseno/backend/pkg/auth/model"
	auth_repository "github.com/aseno/backend/pkg/auth/repository"
	auth_service "github.com/aseno/backend/pkg/auth/service"
	booking_handler "github.com/aseno/backend/pkg/booking/handler"
	booking_service "github.com/aseno/backend/pkg/booking/service"
	commission_handler "github.com/aseno/backend/pkg/commission/handler"
	commission_model "github.com/aseno/backend/pkg/commission/model"
	commission_repository "github.com/aseno/backend/pkg/commission/repository"
	commission_service "github.com/aseno/backend/pkg/commission/service"
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
	dashboard_handler "github.com/aseno/backend/pkg/dashboard/handler"
	dashboard_repository "github.com/aseno/backend/pkg/dashboard/repository"
	dashboard_service "github.com/aseno/backend/pkg/dashboard/service"
	email_service "github.com/aseno/backend/pkg/email/service"
	health_handler "github.com/aseno/backend/pkg/health/handler"
	invite_handler "github.com/aseno/backend/pkg/invite/handler"
	invite_model "github.com/aseno/backend/pkg/invite/model"
	invite_repository "github.com/aseno/backend/pkg/invite/repository"
	invite_service "github.com/aseno/backend/pkg/invite/service"
	notification_handler "github.com/aseno/backend/pkg/notification/handler"
	notification_model "github.com/aseno/backend/pkg/notification/model"
	notification_repository "github.com/aseno/backend/pkg/notification/repository"
	notification_service "github.com/aseno/backend/pkg/notification/service"
	"github.com/aseno/backend/pkg/pagination"
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
	plan_handler "github.com/aseno/backend/pkg/plan/handler"
	plan_model "github.com/aseno/backend/pkg/plan/model"
	plan_repository "github.com/aseno/backend/pkg/plan/repository"
	plan_service "github.com/aseno/backend/pkg/plan/service"
	paymentmethod_handler "github.com/aseno/backend/pkg/paymentmethod/handler"
	paymentmethod_model "github.com/aseno/backend/pkg/paymentmethod/model"
	paymentmethod_repository "github.com/aseno/backend/pkg/paymentmethod/repository"
	paymentmethod_service "github.com/aseno/backend/pkg/paymentmethod/service"
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
	specialty_handler "github.com/aseno/backend/pkg/specialty/handler"
	specialty_model "github.com/aseno/backend/pkg/specialty/model"
	specialty_repository "github.com/aseno/backend/pkg/specialty/repository"
	specialty_service "github.com/aseno/backend/pkg/specialty/service"
	public_handler "github.com/aseno/backend/pkg/public/handler"
	sessionplan_handler "github.com/aseno/backend/pkg/sessionplan/handler"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
	sessionplan_repository "github.com/aseno/backend/pkg/sessionplan/repository"
	sessionplan_service "github.com/aseno/backend/pkg/sessionplan/service"
	settings_handler "github.com/aseno/backend/pkg/settings/handler"
	settings_model "github.com/aseno/backend/pkg/settings/model"
	settings_repository "github.com/aseno/backend/pkg/settings/repository"
	settings_service "github.com/aseno/backend/pkg/settings/service"
	user_handler "github.com/aseno/backend/pkg/user/handler"
	user_model "github.com/aseno/backend/pkg/user/model"
	user_repository "github.com/aseno/backend/pkg/user/repository"
	user_service "github.com/aseno/backend/pkg/user/service"
	waitlist_handler "github.com/aseno/backend/pkg/waitlist/handler"
	waitlist_model "github.com/aseno/backend/pkg/waitlist/model"
	waitlist_repository "github.com/aseno/backend/pkg/waitlist/repository"
	waitlist_service "github.com/aseno/backend/pkg/waitlist/service"
	marketplace_handler "github.com/aseno/backend/pkg/marketplace/handler"
	marketplace_model "github.com/aseno/backend/pkg/marketplace/model"
	marketplace_repository "github.com/aseno/backend/pkg/marketplace/repository"
	marketplace_service "github.com/aseno/backend/pkg/marketplace/service"
	lead_crm "github.com/aseno/backend/pkg/lead/crm"
	lead_notion "github.com/aseno/backend/pkg/lead/crm/notion"
	lead_handler "github.com/aseno/backend/pkg/lead/handler"
	lead_model "github.com/aseno/backend/pkg/lead/model"
	lead_repository "github.com/aseno/backend/pkg/lead/repository"
	lead_service "github.com/aseno/backend/pkg/lead/service"
	audit_handler "github.com/aseno/backend/pkg/audit/handler"
	audit_model "github.com/aseno/backend/pkg/audit/model"
	audit_repository "github.com/aseno/backend/pkg/audit/repository"
	audit_service "github.com/aseno/backend/pkg/audit/service"
	featureflag_handler "github.com/aseno/backend/pkg/featureflag/handler"
	featureflag_model "github.com/aseno/backend/pkg/featureflag/model"
	featureflag_repository "github.com/aseno/backend/pkg/featureflag/repository"
	featureflag_service "github.com/aseno/backend/pkg/featureflag/service"
	systemlog_handler "github.com/aseno/backend/pkg/systemlog/handler"
	systemlog_model "github.com/aseno/backend/pkg/systemlog/model"
	systemlog_repository "github.com/aseno/backend/pkg/systemlog/repository"
	systemlog_service "github.com/aseno/backend/pkg/systemlog/service"
	usagemetric_handler "github.com/aseno/backend/pkg/usagemetric/handler"
	usagemetric_model "github.com/aseno/backend/pkg/usagemetric/model"
	usagemetric_repository "github.com/aseno/backend/pkg/usagemetric/repository"
	usagemetric_service "github.com/aseno/backend/pkg/usagemetric/service"
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
	leadRepo := lead_repository.NewLeadRepository(db.GetDB())
	paymentMethodRepo := paymentmethod_repository.NewPaymentMethodRepository(db.GetDB())
	appointmentPaymentRepo := appointment_repository.NewAppointmentPaymentRepository(db.GetDB())
	specialtyRepo := specialty_repository.NewSpecialtyRepository(db.GetDB())

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
	professionalService := professional_service.NewProfessionalService(professionalRepo, clinicRepo, appointmentRepo)
	professionalProcedureService := professional_service.NewProfessionalProcedureService(professionalProcedureRepo, professionalRepo, procedureRepo)
	commissionSvc := commission_service.NewCommissionService(commissionRepo, professionalProcedureRepo, appointmentRepo)
	patientService := patient_service.NewPatientService(patientRepo, clinicRepo)
	appointmentService := appointment_service.NewAppointmentService(appointmentRepo, professionalRepo)
	procedureService := procedure_service.NewProcedureService(procedureRepo)
	paymentMethodService := paymentmethod_service.NewPaymentMethodService(paymentMethodRepo)
	specialtyService := specialty_service.NewSpecialtyService(specialtyRepo)
	appointmentPaymentService := appointment_service.NewAppointmentPaymentService(appointmentPaymentRepo, appointmentRepo, paymentMethodRepo)

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
	if whatsappClient != nil {
		whatsappService = whatsapp_service.NewWhatsAppService(whatsappClient, messageRepo, nil)

		// Wire up message log service for tracking message lifecycle
		whatsappService.SetMessageLogService(messageLogService)

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
		whatsappService.SetProfessionalLister(&professionalListerAdapter{professionalRepo: professionalRepo})
		whatsappService.SetSlotFinder(&slotFinderAdapter{appointmentService: appointmentService, professionalRepo: professionalRepo})
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
			aiAgentService, _, _, err := initializeAIAgent(
				cfg,
				db,
				redis,
				professionalService,
				appointmentService,
				patientService,
				clinicService,
				patientRepo,
				bookingSvc,
				clinicRepo,
				professionalRepo,
			)
			if err != nil {
				logger.Printf("Warning: Failed to initialize AI Agent: %v", err)
			} else if aiAgentService != nil {
				whatsappService.SetAIAgentService(aiAgentService)
				whatsappService.SetClinicRepo(clinicRepo)
				whatsappService.SetScheduledMessageRepo(scheduledMsgRepo)
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

	// Initialize Session Plan worker (runs every hour)
	sessionPlanWorker := worker.NewSessionPlanWorker(sessionPlanService, 1*time.Hour)
	go sessionPlanWorker.Start()

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
		}

		// Initialize and start Reminder Worker (only if WhatsApp is configured)
		if whatsappService != nil && workerCfg.ReminderWorkerEnabled {
			reminderWorker = worker.NewReminderWorker(workerCfg, appointmentRepo, messageQueue)
			go reminderWorker.Start()

			// Initialize and start Message Sender Worker
			messageSenderWorker = worker.NewMessageSenderWorker(workerCfg, messageQueue, whatsappService, rateLimiter, clinicRepo)
			go messageSenderWorker.Start()

			// Initialize and start Scheduled Message Worker (processes messages scheduled for outside business hours)
			// Uses WHATSAPP_MIN_DELAY_BETWEEN_PATIENTS_SECONDS as poll interval
			scheduledMessageWorker = worker.NewScheduledMessageWorker(
				workerCfg,
				scheduledMsgRepo,
				clinicRepo,
				whatsappService,
			)
			go scheduledMessageWorker.Start()

			// Initialize and start Timeout Worker (marks no_response when patient doesn't confirm)
			if workerCfg.TimeoutWorkerEnabled {
				timeoutWorker := worker.NewTimeoutWorker(workerCfg, appointmentRepo)
				go timeoutWorker.Start()
			}

			logger.Println("Reminder, Message Sender, and Timeout workers started")
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

	// Wire patient risk calculator into appointment service (always enabled)
	appointmentService.SetPatientRiskCalculator(&patientRiskAdapter{patientService: patientService})
	logger.Println("Patient risk calculator connected to appointment service")

	// Wire commission calculator into appointment service
	appointmentService.SetCommissionCalculator(commissionSvc)
	logger.Println("Commission calculator connected to appointment service")

	// Initialize middleware
	authMiddleware := auth_middleware.NewAuthMiddleware(jwtService)
	tenantMiddleware := auth_middleware.NewTenantMiddleware(clinicRepo)

	// Initialize handlers
	healthHandler := health_handler.NewHealthHandler(db)
	authHandler := auth_handler.NewAuthHandler(authService)
	professionalHandler := professional_handler.NewProfessionalHandler(professionalService, auditSvc)
	professionalProcedureHandler := professional_handler.NewProfessionalProcedureHandler(professionalProcedureService, auditSvc)
	commissionHandler := commission_handler.NewCommissionHandler(commissionSvc)
	patientHandler := patient_handler.NewPatientHandler(patientService)
	appointmentHandler := appointment_handler.NewAppointmentHandler(appointmentService)
	procedureHandler := procedure_handler.NewProcedureHandler(procedureService)
	paymentMethodHandler := paymentmethod_handler.NewPaymentMethodHandler(paymentMethodService, auditSvc)
	specialtyHandler := specialty_handler.NewSpecialtyHandler(specialtyService, auditSvc)
	appointmentPaymentHandler := appointment_handler.NewAppointmentPaymentHandler(appointmentPaymentService, auditSvc)

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

	// Initialize Session Plan handler
	sessionPlanHandler := sessionplan_handler.NewSessionPlanHandler(sessionPlanService)

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
	clinicHdlr := clinic_handler.NewClinicHandler(clinicSvc)

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
		leadHdlr,
		leadIscaHdlr,
		paymentMethodHandler,
		appointmentPaymentHandler,
		specialtyHandler,
		commissionHandler,
		portalHdlr,
		marketplaceHdlr,
		authMiddleware,
		tenantMiddleware,
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
	leadHandler lead_handler.LeadHandler,
	leadIscaHandler lead_handler.LeadIscaHandler,
	paymentMethodHandler *paymentmethod_handler.PaymentMethodHandler,
	appointmentPaymentHandler appointment_handler.AppointmentPaymentHandler,
	specialtyHandler *specialty_handler.SpecialtyHandler,
	commissionHandler commission_handler.CommissionHandler,
	portalHandler *portalaccount_handler.PortalHandler,
	marketplaceHandler marketplace_handler.MarketplaceHandler,
	authMiddleware auth_middleware.AuthMiddleware,
	tenantMiddleware auth_middleware.TenantMiddleware,
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
	{
		// Public auth routes (no auth required)
		authHandler.RegisterRoutes(v1)

		// Public invite routes (no auth required)
		inviteHandler.RegisterPublicRoutes(v1)

		// Public config routes (no auth required - landing page)
		publicHandler.RegisterRoutes(v1)

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

			// Appointment routes (with professional access control)
			appointmentsProtected := protected.Group("")
			appointmentsProtected.Use(authMiddleware.RequireProfessionalAccess())
			{
				appointmentHandler.RegisterRoutes(appointmentsProtected)
				// Payment sub-routes registered on the same /appointments group to avoid wildcard conflict
				appointmentPaymentHandler.RegisterRoutes(appointmentsProtected.Group("/appointments"))
			}

			// Procedure routes
			procedureHandler.RegisterRoutes(protected)

			// Payment method routes (settings)
			paymentMethodHandler.RegisterRoutes(protected)

			// Specialty routes (settings)
			specialtyHandler.RegisterRoutes(protected)

			// Commission routes (financial reports)
			commissionHandler.RegisterRoutes(protected)


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
		// Multi-clinic membership
		&clinicmember_model.ClinicMember{},
		// CRM Leads
		&lead_model.Lead{},
		// Payment methods and appointment payments
		&paymentmethod_model.PaymentMethod{},
		&appointment_model.AppointmentPayment{},
		// Specialties and portal accounts
		&specialty_model.Specialty{},
		&professional_model.ProfessionalSpecialty{},
		&professional_model.ProfessionalProcedure{},
		&commission_model.Commission{},
		&portalaccount_model.PortalAccount{},
		// Marketplace (subcontas)
		&marketplace_model.Subaccount{},
		&marketplace_model.SplitConfig{},
		&marketplace_model.ClinicPayment{},
	}

	if err := db.AutoMigrate(models...); err != nil {
		return err
	}

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
		ID:             apt.ID,
		PatientID:      apt.PatientID,
		ProfessionalID: apt.ProfessionalID,
		PatientName:    patientName,
		PatientPhone:   patientPhone,
		Professional:   profName,
		StartTime:      apt.StartTime,
		EndTime:        apt.EndTime,
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
}

func (a *slotFinderAdapter) GetAvailableSlots(ctx context.Context, clinicID uuid.UUID, professionalID uuid.UUID, date time.Time, durationMinutes int) ([]*whatsapp_service.AvailableSlot, error) {
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

// initializeAIAgent initializes the AI agent stack for WhatsApp conversations
func initializeAIAgent(
	cfg *config.Config,
	db *database.Database,
	redis *redisClient.RedisClient,
	professionalService professional_service.ProfessionalService,
	appointmentService appointment_service.AppointmentService,
	patientService patient_service.PatientService,
	clinicService clinic_service.ClinicService,
	patientRepo patient_repository.PatientRepository,
	bookingService booking_service.BookingService,
	clinicRepo clinic_repository.ClinicRepository,
	professionalRepo professional_repository.ProfessionalRepository,
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
		ProfessionalLister:       &aiAgentProfessionalListerAdapter{professionalService: professionalService},
		SlotFinder:               &aiAgentSlotFinderAdapter{appointmentService: appointmentService, clinicRepo: clinicRepo, professionalRepo: professionalRepo},
		AppointmentCreator:       &aiAgentAppointmentCreatorAdapter{appointmentService: appointmentService},
		AppointmentFinder:        &aiAgentAppointmentFinderAdapter{appointmentService: appointmentService},
		AppointmentConfirmer:     &aiAgentAppointmentConfirmerAdapter{appointmentService: appointmentService},
		AppointmentCanceller:     &aiAgentAppointmentCancellerAdapter{appointmentService: appointmentService},
		AppointmentRescheduler:   &aiAgentAppointmentReschedulerAdapter{appointmentService: appointmentService},
		ClinicInfoProvider:       &aiAgentClinicInfoAdapter{clinicService: clinicService},
		ProfessionalInfoProvider: &aiAgentProfessionalInfoAdapter{professionalService: professionalService},
		PatientInfoProvider:      &aiAgentPatientInfoAdapter{patientService: patientService},
		PatientFinderCreator:     &aiAgentPatientFinderCreatorAdapter{patientService: patientService, patientRepo: patientRepo},
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
		RemindersEnabled:  workerCfg.AIAgentRemindersEnabled,
	}

	// 9. Create AI agent service
	agentService := agent.NewService(
		aiAgent,
		conversationMgr,
		toolRegistry,
		toolExecutor,
		execContext,
		featureFlags,
		clinicRepo,
		workerCfg.MinDelayBetweenMessagesSeconds,
	)

	logger.Printf("AI Agent initialized: provider=%s model=%s", workerCfg.AIAgentProvider, workerCfg.AIAgentModel)
	logger.Printf("AI Agent features: scheduling=%v confirm=%v cancel=%v reminders=%v",
		workerCfg.AIAgentSchedulingEnabled,
		workerCfg.AIAgentConfirmEnabled,
		workerCfg.AIAgentCancelEnabled,
		workerCfg.AIAgentRemindersEnabled)

	return agentService, conversationMgr, execContext, nil
}

// AI Agent adapters - wrap existing services to match tool interfaces

type aiAgentProfessionalListerAdapter struct {
	professionalService professional_service.ProfessionalService
}

func (a *aiAgentProfessionalListerAdapter) ListAvailable(ctx context.Context, clinicID uuid.UUID, specialty string) ([]*tools.ProfessionalInfo, error) {
	// Use List method with activeOnly=true to get available professionals
	listResp, err := a.professionalService.List(ctx, clinicID, pagination.Filter{Status: "active", Page: 1, Limit: 100})
	if err != nil {
		return nil, err
	}

	var result []*tools.ProfessionalInfo
	for _, p := range listResp.Data {
		// Build specialty string from N:N specialties
		actualSpecialty := "Geral"
		if len(p.Specialties) > 0 {
			actualSpecialty = p.Specialties[0].Name
		}

		// Filter by specialty if provided
		if specialty != "" {
			matched := false
			for _, sp := range p.Specialties {
				if sp.Name == specialty {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		result = append(result, &tools.ProfessionalInfo{
			ID:          p.ID,
			Name:        p.Name,
			Specialty:   actualSpecialty,
			IsAvailable: p.IsActive,
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
}

func (a *aiAgentAppointmentFinderAdapter) GetByPatientPhone(ctx context.Context, clinicID uuid.UUID, phone string, status string, limit int) ([]*tools.AppointmentInfo, error) {
	// This would need to be implemented in your appointment service
	// For now, return empty
	return []*tools.AppointmentInfo{}, nil
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
}

func (a *aiAgentAppointmentReschedulerAdapter) Reschedule(ctx context.Context, appointmentID, clinicID uuid.UUID, newStartTime time.Time, newProfessionalID *uuid.UUID) error {
	// This would need to be implemented in your appointment service
	// For now, return not implemented error
	return fmt.Errorf("reschedule not yet implemented")
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
