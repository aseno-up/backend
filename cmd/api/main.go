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

	"github.com/saas/backend/internal/config"
	"github.com/saas/backend/internal/database"
	"github.com/saas/backend/internal/worker"
	appointment_handler "github.com/saas/backend/pkg/appointment/handler"
	appointment_model "github.com/saas/backend/pkg/appointment/model"
	appointment_repository "github.com/saas/backend/pkg/appointment/repository"
	appointment_service "github.com/saas/backend/pkg/appointment/service"
	auth_google "github.com/saas/backend/pkg/auth/client/google"
	auth_handler "github.com/saas/backend/pkg/auth/handler"
	auth_middleware "github.com/saas/backend/pkg/auth/middleware"
	auth_service "github.com/saas/backend/pkg/auth/service"
	calendar_google "github.com/saas/backend/pkg/calendar/client/google"
	calendar_handler "github.com/saas/backend/pkg/calendar/handler"
	calendar_model "github.com/saas/backend/pkg/calendar/model"
	calendar_repository "github.com/saas/backend/pkg/calendar/repository"
	calendar_service "github.com/saas/backend/pkg/calendar/service"
	clinic_model "github.com/saas/backend/pkg/clinic/model"
	clinic_repository "github.com/saas/backend/pkg/clinic/repository"
	email_service "github.com/saas/backend/pkg/email/service"
	health_handler "github.com/saas/backend/pkg/health/handler"
	invite_handler "github.com/saas/backend/pkg/invite/handler"
	invite_model "github.com/saas/backend/pkg/invite/model"
	invite_repository "github.com/saas/backend/pkg/invite/repository"
	invite_service "github.com/saas/backend/pkg/invite/service"
	patient_handler "github.com/saas/backend/pkg/patient/handler"
	patient_model "github.com/saas/backend/pkg/patient/model"
	patient_repository "github.com/saas/backend/pkg/patient/repository"
	patient_service "github.com/saas/backend/pkg/patient/service"
	patientrecord_handler "github.com/saas/backend/pkg/patientrecord/handler"
	patientrecord_model "github.com/saas/backend/pkg/patientrecord/model"
	patientrecord_repository "github.com/saas/backend/pkg/patientrecord/repository"
	patientrecord_service "github.com/saas/backend/pkg/patientrecord/service"
	payment_asaas "github.com/saas/backend/pkg/payment/client/asaas"
	payment_handler "github.com/saas/backend/pkg/payment/handler"
	payment_model "github.com/saas/backend/pkg/payment/model"
	payment_repository "github.com/saas/backend/pkg/payment/repository"
	payment_service "github.com/saas/backend/pkg/payment/service"
	professional_handler "github.com/saas/backend/pkg/professional/handler"
	professional_model "github.com/saas/backend/pkg/professional/model"
	professional_repository "github.com/saas/backend/pkg/professional/repository"
	professional_service "github.com/saas/backend/pkg/professional/service"
	sessionplan_handler "github.com/saas/backend/pkg/sessionplan/handler"
	sessionplan_model "github.com/saas/backend/pkg/sessionplan/model"
	sessionplan_repository "github.com/saas/backend/pkg/sessionplan/repository"
	sessionplan_service "github.com/saas/backend/pkg/sessionplan/service"
	user_handler "github.com/saas/backend/pkg/user/handler"
	user_model "github.com/saas/backend/pkg/user/model"
	user_repository "github.com/saas/backend/pkg/user/repository"
	user_service "github.com/saas/backend/pkg/user/service"
	whatsapp_evolution "github.com/saas/backend/pkg/whatsapp/client/evolution"
	whatsapp_handler "github.com/saas/backend/pkg/whatsapp/handler"
	whatsapp_model "github.com/saas/backend/pkg/whatsapp/model"
	whatsapp_repository "github.com/saas/backend/pkg/whatsapp/repository"
	whatsapp_service "github.com/saas/backend/pkg/whatsapp/service"
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
	customerRepo := payment_repository.NewCustomerRepository(db.GetDB())
	paymentRepo := payment_repository.NewPaymentRepository(db.GetDB())
	subscriptionRepo := payment_repository.NewSubscriptionRepository(db.GetDB())
	calendarIntegrationRepo := calendar_repository.NewIntegrationRepository(db.GetDB())
	calendarSyncRepo := calendar_repository.NewSyncRepository(db.GetDB())
	sessionPlanRepo := sessionplan_repository.NewSessionPlanRepository(db.GetDB())
	inviteRepo := invite_repository.NewInviteRepository(db.GetDB())
	patientRecordRepo := patientrecord_repository.NewPatientRecordRepository(db.GetDB())

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

	// Initialize auth service with or without Google OAuth
	var authService auth_service.AuthService
	if googleAuthClient != nil {
		authService = auth_service.NewAuthServiceWithGoogle(userRepo, clinicRepo, jwtService, &googleAuthAdapter{client: googleAuthClient})
	} else {
		authService = auth_service.NewAuthService(userRepo, clinicRepo, jwtService)
	}
	professionalService := professional_service.NewProfessionalService(professionalRepo)
	patientService := patient_service.NewPatientService(patientRepo)
	appointmentService := appointment_service.NewAppointmentService(appointmentRepo)

	// Initialize WhatsApp service (if client is configured)
	var whatsappService whatsapp_service.WhatsAppService
	if whatsappClient != nil {
		whatsappService = whatsapp_service.NewWhatsAppService(whatsappClient, messageRepo, nil)
	}

	// Initialize Payment service (if Asaas client is configured)
	var paymentService payment_service.PaymentService
	if asaasClient != nil {
		paymentService = payment_service.NewPaymentService(asaasClient, customerRepo, paymentRepo, subscriptionRepo)
	}

	// Initialize Calendar service (if Google client is configured)
	var calendarService calendar_service.CalendarService
	if googleCalendarClient != nil {
		calendarService = calendar_service.NewCalendarService(googleCalendarClient, calendarIntegrationRepo, calendarSyncRepo)
	}

	// Initialize Session Plan service
	sessionPlanService := sessionplan_service.NewSessionPlanService(sessionPlanRepo, appointmentRepo)

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

	// Initialize Session Plan worker (runs every hour)
	sessionPlanWorker := worker.NewSessionPlanWorker(sessionPlanService, 1*time.Hour)
	go sessionPlanWorker.Start()

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

			// Appointment routes
			appointmentHandler.RegisterRoutes(protected)

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

			// Admin-only routes
			adminProtected := protected.Group("")
			adminProtected.Use(authMiddleware.RequireAdmin())
			{
				// Invite routes
				inviteHandler.RegisterProtectedRoutes(adminProtected)
				// User management routes
				userHandler.RegisterRoutes(adminProtected)
			}
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
		&payment_model.Customer{},
		&payment_model.Subscription{},
		&payment_model.Payment{},
		&calendar_model.CalendarIntegration{},
		&calendar_model.CalendarSync{},
		&sessionplan_model.SessionPlan{},
		&invite_model.Invite{},
		&patientrecord_model.PatientRecord{},
		&patientrecord_model.SessionNote{},
	}

	if err := db.AutoMigrate(models...); err != nil {
		return err
	}

	log.Println("Database migrations completed")
	return nil
}
