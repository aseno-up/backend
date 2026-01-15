package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/internal/config"
	"github.com/aseno/backend/internal/database"
	"github.com/aseno/backend/internal/queue"
	"github.com/aseno/backend/internal/worker"
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
	calendar_google "github.com/aseno/backend/pkg/calendar/client/google"
	calendar_handler "github.com/aseno/backend/pkg/calendar/handler"
	calendar_model "github.com/aseno/backend/pkg/calendar/model"
	calendar_repository "github.com/aseno/backend/pkg/calendar/repository"
	calendar_service "github.com/aseno/backend/pkg/calendar/service"
	clinic_handler "github.com/aseno/backend/pkg/clinic/handler"
	clinic_model "github.com/aseno/backend/pkg/clinic/model"
	clinic_repository "github.com/aseno/backend/pkg/clinic/repository"
	clinic_service "github.com/aseno/backend/pkg/clinic/service"
	dashboard_handler "github.com/aseno/backend/pkg/dashboard/handler"
	dashboard_repository "github.com/aseno/backend/pkg/dashboard/repository"
	dashboard_service "github.com/aseno/backend/pkg/dashboard/service"
	email_service "github.com/aseno/backend/pkg/email/service"
	health_handler "github.com/aseno/backend/pkg/health/handler"
	invite_handler "github.com/aseno/backend/pkg/invite/handler"
	invite_model "github.com/aseno/backend/pkg/invite/model"
	invite_repository "github.com/aseno/backend/pkg/invite/repository"
	invite_service "github.com/aseno/backend/pkg/invite/service"
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
	professional_handler "github.com/aseno/backend/pkg/professional/handler"
	professional_model "github.com/aseno/backend/pkg/professional/model"
	professional_repository "github.com/aseno/backend/pkg/professional/repository"
	professional_service "github.com/aseno/backend/pkg/professional/service"
	sessionplan_handler "github.com/aseno/backend/pkg/sessionplan/handler"
	sessionplan_model "github.com/aseno/backend/pkg/sessionplan/model"
	sessionplan_repository "github.com/aseno/backend/pkg/sessionplan/repository"
	sessionplan_service "github.com/aseno/backend/pkg/sessionplan/service"
	user_handler "github.com/aseno/backend/pkg/user/handler"
	user_model "github.com/aseno/backend/pkg/user/model"
	user_repository "github.com/aseno/backend/pkg/user/repository"
	user_service "github.com/aseno/backend/pkg/user/service"
	waitlist_handler "github.com/aseno/backend/pkg/waitlist/handler"
	waitlist_model "github.com/aseno/backend/pkg/waitlist/model"
	waitlist_repository "github.com/aseno/backend/pkg/waitlist/repository"
	waitlist_service "github.com/aseno/backend/pkg/waitlist/service"
	whatsapp_classifier "github.com/aseno/backend/pkg/whatsapp/classifier"
	whatsapp_evolution "github.com/aseno/backend/pkg/whatsapp/client/evolution"
	whatsapp_handler "github.com/aseno/backend/pkg/whatsapp/handler"
	whatsapp_model "github.com/aseno/backend/pkg/whatsapp/model"
	whatsapp_repository "github.com/aseno/backend/pkg/whatsapp/repository"
	whatsapp_service "github.com/aseno/backend/pkg/whatsapp/service"
	analytics_handler "github.com/aseno/backend/pkg/analytics/handler"
	analytics_repository "github.com/aseno/backend/pkg/analytics/repository"
	analytics_service "github.com/aseno/backend/pkg/analytics/service"
	notification_handler "github.com/aseno/backend/pkg/notification/handler"
	notification_model "github.com/aseno/backend/pkg/notification/model"
	notification_repository "github.com/aseno/backend/pkg/notification/repository"
	notification_service "github.com/aseno/backend/pkg/notification/service"
	public_handler "github.com/aseno/backend/pkg/public/handler"
	settings_handler "github.com/aseno/backend/pkg/settings/handler"
	booking_handler "github.com/aseno/backend/pkg/booking/handler"
	booking_service "github.com/aseno/backend/pkg/booking/service"
	settings_model "github.com/aseno/backend/pkg/settings/model"
	settings_repository "github.com/aseno/backend/pkg/settings/repository"
	settings_service "github.com/aseno/backend/pkg/settings/service"
)

func main() {
	// Load configuration
	cfg := config.NewConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Set Gin mode
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize database
	db, err := database.NewDatabase(cfg.GetDatabaseURL(), cfg.IsDevelopment())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	clinicRepo := clinic_repository.NewClinicRepository(db.GetDB())
	userRepo := user_repository.NewUserRepository(db.GetDB())
	professionalRepo := professional_repository.NewProfessionalRepository(db.GetDB())
	patientRepo := patient_repository.NewPatientRepository(db.GetDB())
	appointmentRepo := appointment_repository.NewAppointmentRepository(db.GetDB())
	messageRepo := whatsapp_repository.NewMessageRepository(db.GetDB())
	messageLogRepo := whatsapp_repository.NewMessageLogRepository(db.GetDB())
	customerRepo := payment_repository.NewCustomerRepository(db.GetDB())
	paymentRepo := payment_repository.NewPaymentRepository(db.GetDB())
	subscriptionRepo := payment_repository.NewSubscriptionRepository(db.GetDB())
	calendarIntegrationRepo := calendar_repository.NewIntegrationRepository(db.GetDB())
	calendarSyncRepo := calendar_repository.NewSyncRepository(db.GetDB())
	sessionPlanRepo := sessionplan_repository.NewSessionPlanRepository(db.GetDB())
	inviteRepo := invite_repository.NewInviteRepository(db.GetDB())
	patientRecordRepo := patientrecord_repository.NewPatientRecordRepository(db.GetDB())
	dashboardRepo := dashboard_repository.NewDashboardRepository(db.GetDB())
	passwordResetRepo := auth_repository.NewPasswordResetRepository(db.GetDB())
	emailVerificationRepo := auth_repository.NewEmailVerificationRepository(db.GetDB())
	waitlistRepo := waitlist_repository.NewWaitlistRepository(db.GetDB())
	analyticsRepo := analytics_repository.NewAnalyticsRepository(db.GetDB())
	notificationRepo := notification_repository.NewNotificationRepository(db.GetDB())
	settingsRepo := settings_repository.NewSettingsRepository(db.GetDB())

	// Seed default settings (if not exist)
	if err := settingsRepo.SeedDefaults(context.Background()); err != nil {
		log.Printf("Warning: Failed to seed default settings: %v", err)
	}

	// Initialize Settings service (used by professional service and public config)
	settingsSvc := settings_service.NewSettingsService(settingsRepo)

	// Initialize WhatsApp client (Evolution API)
	var whatsappClient whatsapp_service.WhatsAppClient
	if cfg.GetEvolutionAPIURL() != "" {
		whatsappClient = whatsapp_evolution.NewEvolutionClient(whatsapp_evolution.Config{
			BaseURL: cfg.GetEvolutionAPIURL(),
			APIKey:  cfg.GetEvolutionAPIKey(),
		})
		log.Println("WhatsApp (Evolution API) client initialized")
	}

	// Initialize Asaas client (Payment Gateway)
	var asaasClient *payment_asaas.Client
	if cfg.GetAsaasAPIKey() != "" {
		asaasClient = payment_asaas.NewClient(payment_asaas.Config{
			BaseURL:   cfg.GetAsaasAPIURL(),
			APIKey:    cfg.GetAsaasAPIKey(),
			IsSandbox: cfg.AsaasSandbox,
		})
		log.Println("Asaas payment client initialized")
	}

	// Initialize Google Auth client (for OAuth login)
	var googleAuthClient *auth_google.Client
	if cfg.GetGoogleClientID() != "" {
		googleAuthClient = auth_google.NewClient(auth_google.Config{
			ClientID:     cfg.GetGoogleClientID(),
			ClientSecret: cfg.GetGoogleClientSecret(),
			RedirectURL:  cfg.GoogleAuthRedirectURL,
		})
		log.Println("Google Auth client initialized")
	}

	// Initialize Google Calendar client
	var googleCalendarClient *calendar_google.Client
	if cfg.GetGoogleClientID() != "" && cfg.GetGoogleClientSecret() != "" {
		googleCalendarClient = calendar_google.NewClient(calendar_google.Config{
			ClientID:     cfg.GetGoogleClientID(),
			ClientSecret: cfg.GetGoogleClientSecret(),
			RedirectURL:  cfg.GoogleRedirectURL,
		})
		log.Println("Google Calendar client initialized")
	}

	// Initialize services
	jwtService := auth_service.NewJWTService(
		cfg.GetJWTSecret(),
		cfg.JWTAccessTokenTTL,
		cfg.JWTRefreshTokenTTL,
	)

	// Initialize Email service
	var emailSvc email_service.EmailService
	if cfg.GetSMTPHost() != "" {
		emailSvc = email_service.NewSMTPEmailService(email_service.SMTPConfig{
			Host:        cfg.GetSMTPHost(),
			Port:        cfg.GetSMTPPort(),
			User:        cfg.GetSMTPUser(),
			Password:    cfg.GetSMTPPassword(),
			From:        cfg.GetSMTPFrom(),
			FrontendURL: cfg.GetFrontendURL(),
		})
		log.Println("SMTP email service initialized")
	} else {
		emailSvc = email_service.NewNoOpEmailService()
		log.Println("Using no-op email service (SMTP not configured)")
	}

	// Initialize auth service with or without Google OAuth
	var authService auth_service.AuthService
	if googleAuthClient != nil {
		authService = auth_service.NewAuthServiceWithGoogle(userRepo, clinicRepo, passwordResetRepo, emailVerificationRepo, jwtService, &googleAuthAdapter{client: googleAuthClient}, emailSvc)
	} else {
		authService = auth_service.NewAuthService(userRepo, clinicRepo, passwordResetRepo, emailVerificationRepo, jwtService, emailSvc)
	}
	professionalService := professional_service.NewProfessionalService(professionalRepo, clinicRepo, settingsSvc)
	patientService := patient_service.NewPatientService(patientRepo, clinicRepo)
	appointmentService := appointment_service.NewAppointmentService(appointmentRepo, professionalRepo)

	// Initialize message log service
	messageLogService := whatsapp_service.NewMessageLogService(messageLogRepo)

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
		log.Println("WhatsApp appointment status updater and reschedule connected")

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
			}

			if apiKey != "" {
				aiClassifier, err := whatsapp_classifier.NewClassifier(whatsapp_classifier.Config{
					Provider: cfg.AIClassifierProvider(),
					Model:    cfg.AIClassifierModel(),
					APIKey:   apiKey,
					Timeout:  time.Duration(cfg.AIClassifierTimeout()) * time.Second,
				})
				if err != nil {
					log.Printf("Warning: Failed to initialize AI classifier: %v", err)
				} else {
					whatsappService.SetClassifier(aiClassifier, cfg.AIClassifierConfidenceThreshold())
					log.Printf("AI Classifier initialized: provider=%s model=%s threshold=%.2f",
						cfg.AIClassifierProvider(), cfg.AIClassifierModel(), cfg.AIClassifierConfidenceThreshold())
				}
			} else {
				log.Printf("Warning: AI_CLASSIFIER_ENABLED=true but no API key for provider %s",
					cfg.AIClassifierProvider())
			}
		}
	}

	// Initialize Payment service (if Asaas client is configured)
	var paymentService payment_service.PaymentService
	if asaasClient != nil {
		paymentService = payment_service.NewPaymentService(asaasClient, customerRepo, paymentRepo, subscriptionRepo)
	}

	// Initialize Calendar service (if Google client is configured)
	var calendarService calendar_service.CalendarService
	if googleCalendarClient != nil {
		calendarService = calendar_service.NewCalendarService(googleCalendarClient, calendarIntegrationRepo, calendarSyncRepo, clinicRepo)
		// Wire calendar service into appointment service for Google Calendar sync
		appointmentService.SetCalendarService(calendarService)
		log.Println("Google Calendar sync enabled for appointments")
	}

	// Initialize Session Plan service
	sessionPlanService := sessionplan_service.NewSessionPlanService(sessionPlanRepo, appointmentRepo)

	// Initialize Invite service
	inviteSvc := invite_service.NewInviteService(
		inviteRepo,
		userRepo,
		clinicRepo,
		emailSvc,
		jwtService,
		cfg.GetFrontendURL(),
	)

	// Initialize User service
	userSvc := user_service.NewUserService(userRepo)

	// Initialize Patient Record service
	patientRecordSvc := patientrecord_service.NewPatientRecordService(patientRecordRepo)

	// Initialize Dashboard service
	dashboardSvc := dashboard_service.NewDashboardService(dashboardRepo, clinicRepo)

	// Initialize Analytics service
	analyticsSvc := analytics_service.NewAnalyticsService(analyticsRepo, clinicRepo)

	// Initialize Notification service
	notificationSvc := notification_service.NewNotificationService(notificationRepo)

	// Initialize Waitlist service (if WhatsApp and queue are available)
	var waitlistSvc waitlist_service.WaitlistService
	// Note: We'll fully initialize this after the queue is set up

	// Initialize Session Plan worker (runs every hour)
	sessionPlanWorker := worker.NewSessionPlanWorker(sessionPlanService, 1*time.Hour)
	go sessionPlanWorker.Start()

	// Initialize worker configuration
	workerCfg := config.NewWorkerConfig()

	// Initialize RabbitMQ queue (if configured)
	var messageQueue queue.Queue
	var reminderWorker *worker.ReminderWorker
	var messageSenderWorker *worker.MessageSenderWorker

	if workerCfg.RabbitMQURL != "" {
		messageQueue = queue.NewRabbitMQ(queue.RabbitMQConfig{
			URL:           workerCfg.RabbitMQURL,
			QueueName:     workerCfg.RabbitMQQueue,
			DLQName:       workerCfg.RabbitMQDLQ,
			PrefetchCount: 1,
		})

		// Connect to RabbitMQ
		if err := messageQueue.Connect(context.Background()); err != nil {
			log.Printf("Warning: Failed to connect to RabbitMQ: %v", err)
			log.Println("Reminder workers will be disabled")
		} else {
			log.Println("RabbitMQ connected successfully")

			// Initialize and start Reminder Worker (only if WhatsApp is configured)
			if whatsappService != nil && workerCfg.ReminderWorkerEnabled {
				reminderWorker = worker.NewReminderWorker(workerCfg, appointmentRepo, messageQueue)
				go reminderWorker.Start()

				// Initialize and start Message Sender Worker
				messageSenderWorker = worker.NewMessageSenderWorker(workerCfg, messageQueue, whatsappService)
				go messageSenderWorker.Start()

				log.Println("Reminder and Message Sender workers started")
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
				)

				// Wire waitlist service into appointment service
				appointmentService.SetWaitlistProcessor(waitlistSvc)

				// Wire waitlist service into whatsapp service for response handling
				whatsappService.SetWaitlistHandler(waitlistSvc)

				log.Println("Waitlist (encaixe) service initialized")
			}
		}
	} else {
		log.Println("RabbitMQ not configured - reminder workers and waitlist disabled")
	}

	// Wire patient risk calculator into appointment service (always enabled)
	appointmentService.SetPatientRiskCalculator(&patientRiskAdapter{patientService: patientService})
	log.Println("Patient risk calculator connected to appointment service")

	// Initialize middleware
	authMiddleware := auth_middleware.NewAuthMiddleware(jwtService)
	tenantMiddleware := auth_middleware.NewTenantMiddleware(clinicRepo)

	// Initialize handlers
	healthHandler := health_handler.NewHealthHandler(db)
	authHandler := auth_handler.NewAuthHandler(authService)
	professionalHandler := professional_handler.NewProfessionalHandler(professionalService)
	patientHandler := patient_handler.NewPatientHandler(patientService)
	appointmentHandler := appointment_handler.NewAppointmentHandler(appointmentService)

	// Initialize WhatsApp handler (if service is configured)
	var whatsappHandler whatsapp_handler.WhatsAppHandler
	if whatsappService != nil {
		whatsappHandler = whatsapp_handler.NewWhatsAppHandler(whatsappService, clinicRepo, cfg.GetWhatsAppWebhookSecret())
	}

	// Initialize Payment handler (if service is configured)
	var paymentHandler payment_handler.PaymentHandler
	if paymentService != nil {
		paymentHandler = payment_handler.NewPaymentHandler(paymentService, cfg.GetAsaasWebhookSecret())
	}

	// Initialize Calendar handler (if service is configured)
	var calendarHandler calendar_handler.CalendarHandler
	if calendarService != nil {
		calendarHandler = calendar_handler.NewCalendarHandler(calendarService)
	}

	// Initialize Session Plan handler
	sessionPlanHandler := sessionplan_handler.NewSessionPlanHandler(sessionPlanService)

	// Initialize Invite handler
	inviteHdlr := invite_handler.NewInviteHandler(inviteSvc)

	// Initialize User handler
	userHdlr := user_handler.NewUserHandler(userSvc)

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
	settingsHdlr := settings_handler.NewSettingsHandler(settingsSvc)

	// Initialize Public handler (no auth required - landing page config)
	publicHdlr := public_handler.NewPublicHandler(settingsSvc)

	// Initialize Booking service and handler (public online booking)
	bookingSvc := booking_service.NewBookingService(
		clinicRepo,
		professionalRepo,
		appointmentRepo,
		patientRepo,
		notificationSvc,
	)
	bookingHdlr := booking_handler.NewBookingHandler(bookingSvc)

	// Setup router
	router := setupRouter(
		cfg,
		healthHandler,
		authHandler,
		professionalHandler,
		patientHandler,
		appointmentHandler,
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
		log.Printf("Starting server on port %s", cfg.Port)
		log.Printf("Environment: %s", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop workers
	sessionPlanWorker.Stop()
	if reminderWorker != nil {
		reminderWorker.Stop()
	}
	if messageSenderWorker != nil {
		messageSenderWorker.Stop()
	}
	if messageQueue != nil {
		messageQueue.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func setupRouter(
	cfg *config.Config,
	healthHandler health_handler.HealthHandler,
	authHandler auth_handler.AuthHandler,
	professionalHandler professional_handler.ProfessionalHandler,
	patientHandler patient_handler.PatientHandler,
	appointmentHandler appointment_handler.AppointmentHandler,
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

		// Public booking routes (no auth required - online booking)
		bookingHandler.RegisterRoutes(v1)

		// Public SSE stream route (uses token in query param for auth)
		notificationHandler.RegisterPublicRoutes(v1)

		// Protected routes (auth required)
		protected := v1.Group("")
		protected.Use(authMiddleware.Authenticate())
		protected.Use(tenantMiddleware.ValidateSubscription())
		{
			// Protected auth routes
			auth := protected.Group("/auth")
			{
				auth.POST("/logout", func(c *gin.Context) {
					authHandler.Logout(c)
				})
				auth.GET("/me", func(c *gin.Context) {
					authHandler.GetCurrentUser(c)
				})
				auth.POST("/change-password", func(c *gin.Context) {
					authHandler.ChangePassword(c)
				})
			}

			// Professional routes
			professionalHandler.RegisterRoutes(protected)

			// Patient routes
			patientHandler.RegisterRoutes(protected)

			// Appointment routes (with professional access control)
			// Professional role can only see their own appointments
			appointmentsProtected := protected.Group("")
			appointmentsProtected.Use(authMiddleware.RequireProfessionalAccess())
			{
				appointmentHandler.RegisterRoutes(appointmentsProtected)
			}

			// WhatsApp routes (protected endpoints)
			if whatsappHandler != nil {
				whatsappHandler.RegisterRoutes(protected)
			}

			// Payment routes (protected endpoints)
			if paymentHandler != nil {
				paymentHandler.RegisterRoutes(protected)
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
		}

		// WhatsApp webhook (public, validated by secret)
		if whatsappHandler != nil {
			v1.POST("/whatsapp/webhook", whatsappHandler.Webhook)
			v1.POST("/whatsapp/webhook/:secret", func(c *gin.Context) {
				// Handled by the handler itself
				whatsappHandler.Webhook(c)
			})
		}

		// Payment webhook (public, validated by secret)
		if paymentHandler != nil {
			v1.POST("/billing/webhook", paymentHandler.Webhook)
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
	log.Println("Running database migrations...")

	// Add all models that need migration here
	// Order matters due to foreign key constraints
	models := []interface{}{
		&clinic_model.Clinic{},
		&user_model.User{},
		&professional_model.Professional{},
		&patient_model.Patient{},
		&appointment_model.Appointment{},
		&whatsapp_model.Message{},
		&whatsapp_model.MessageLog{},
		&payment_model.Customer{},
		&payment_model.Subscription{},
		&payment_model.Payment{},
		&calendar_model.CalendarIntegration{},
		&calendar_model.CalendarSync{},
		&sessionplan_model.SessionPlan{},
		&invite_model.Invite{},
		&patientrecord_model.PatientRecord{},
		&patientrecord_model.SessionNote{},
		&auth_model.PasswordReset{},
		&auth_model.EmailVerification{},
		&notification_model.Notification{},
		&waitlist_model.WaitlistEntry{},
		&settings_model.SystemSetting{},
	}

	if err := db.AutoMigrate(models...); err != nil {
		return err
	}

	log.Println("Database migrations completed")
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
		ID:               clinic.ID,
		Name:             clinic.Name,
		WhatsAppInstance: clinic.WhatsAppInstance,
		WhatsAppConnected: clinic.WhatsAppConnected,
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
		log.Printf("[Reschedule] Failed to auto-confirm appointment: %v", err)
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
