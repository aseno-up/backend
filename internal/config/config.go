package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config contains all application configurations
type Config struct {
	// Server
	Port        string
	Environment string

	// Database (private - sensitive)
	databaseURL string

	// JWT (private - sensitive)
	jwtSecret          string
	JWTAccessTokenTTL  int // minutes
	JWTRefreshTokenTTL int // days

	// Google OAuth (private - sensitive)
	googleClientID        string
	googleClientSecret    string
	GoogleRedirectURL     string // For calendar integration
	GoogleAuthRedirectURL string // For authentication

	// WhatsApp Provider Selection
	whatsappProvider string // "evolution" or "waha"

	// Evolution API / WhatsApp (private - sensitive)
	evolutionAPIURL string
	evolutionAPIKey string

	// WAHA (WhatsApp HTTP API) (private - sensitive)
	wahaAPIURL string
	wahaAPIKey string

	whatsappWebhookSecret string
	whatsappWebhookURL    string

	// Asaas Payment (private - sensitive)
	asaasAPIURL        string
	asaasAPIKey        string
	asaasWebhookSecret string
	AsaasSandbox       bool

	// CORS
	AllowedOrigins []string

	// Logging
	LogLevel string

	// SMTP (private - sensitive)
	smtpHost                  string
	smtpPort                  int
	smtpUser                  string
	smtpPassword              string
	smtpFrom                  string
	emailSenderSupportAddress string
	emailSenderName           string

	// Frontend URL
	frontendURL string

	// AI Classifier
	aiClassifierEnabled             bool
	aiClassifierProvider            string
	aiClassifierModel               string
	aiClassifierTimeout             int // seconds
	aiClassifierConfidenceThreshold float64
	geminiAPIKey                    string
	anthropicAPIKey                 string
	openaiAPIKey                    string
	brevoAPIKey                     string

	// OpenRouter Configs
	openrouterAPIKey           string
	openrouterAPIURL           string
	openrouterHTTPReferer      string
	openrouterClassifierTitle  string
	openrouterTranscriberTitle string
}

// NewConfig creates a new configuration from environment variables
func NewConfig() *Config {
	// Load .env file if exists (development)
	_ = godotenv.Load()

	return &Config{
		Port:                      getEnv("PORT", "4901"),
		Environment:               getEnv("ENVIRONMENT", "development"),
		databaseURL:               getEnv("DATABASE_URL", ""),
		jwtSecret:                 getEnv("JWT_SECRET", ""),
		JWTAccessTokenTTL:         getEnvAsInt("JWT_ACCESS_TOKEN_TTL", 15),
		JWTRefreshTokenTTL:        getEnvAsInt("JWT_REFRESH_TOKEN_TTL", 7),
		googleClientID:            getEnv("GOOGLE_CLIENT_ID", ""),
		googleClientSecret:        getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:         getEnv("GOOGLE_REDIRECT_URL", ""),
		GoogleAuthRedirectURL:     getEnv("GOOGLE_AUTH_REDIRECT_URL", ""),
		whatsappProvider:          getEnv("WHATSAPP_PROVIDER", "evolution"), // Default to evolution for backwards compatibility
		evolutionAPIURL:           getEnv("EVOLUTION_API_URL", ""),
		evolutionAPIKey:           getEnv("EVOLUTION_API_KEY", ""),
		wahaAPIURL:                getEnv("WAHA_API_URL", ""),
		wahaAPIKey:                getEnv("WAHA_API_KEY", ""),
		whatsappWebhookSecret:     getEnv("WHATSAPP_WEBHOOK_SECRET", ""),
		whatsappWebhookURL:        getEnv("WHATSAPP_WEBHOOK_URL", ""),
		asaasAPIURL:               getEnv("ASAAS_API_URL", "https://sandbox.asaas.com/api/v3"),
		asaasAPIKey:               getEnv("ASAAS_API_KEY", ""),
		asaasWebhookSecret:        getEnv("ASAAS_WEBHOOK_SECRET", ""),
		AsaasSandbox:              getEnvAsBool("ASAAS_SANDBOX", true),
		AllowedOrigins:            getEnvAsSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		LogLevel:                  getEnv("LOG_LEVEL", "info"),
		smtpHost:                  getEnv("SMTP_HOST", ""),
		smtpPort:                  getEnvAsInt("SMTP_PORT", 587),
		smtpUser:                  getEnv("SMTP_USER", ""),
		smtpPassword:              getEnv("SMTP_PASSWORD", ""),
		smtpFrom:                  getEnv("SMTP_FROM", ""),
		emailSenderSupportAddress: getEnv("EMAIL_SENDER_SUPORT_ADDRESS", ""),
		emailSenderName:           getEnv("EMAIL_SENDER_NAME", "Aseno"),
		frontendURL:               getEnv("FRONTEND_URL", "http://localhost:3001"),

		// AI Classifier defaults
		aiClassifierEnabled:             getEnvAsBool("AI_CLASSIFIER_ENABLED", false),
		aiClassifierProvider:            getEnv("AI_CLASSIFIER_PROVIDER", "openrouter"),
		aiClassifierModel:               getEnv("AI_CLASSIFIER_MODEL", "google/gemini-2.0-flash-001"),
		aiClassifierTimeout:             getEnvAsInt("AI_CLASSIFIER_TIMEOUT", 5),
		aiClassifierConfidenceThreshold: getEnvAsFloat64("AI_CLASSIFIER_CONFIDENCE_THRESHOLD", 0.7),
		geminiAPIKey:                    getEnv("GEMINI_API_KEY", ""), // kept for backwards compatibility
		anthropicAPIKey:                 getEnv("ANTHROPIC_API_KEY", ""),
		openaiAPIKey:                    getEnv("OPENAI_API_KEY", ""),
		brevoAPIKey:                     getEnv("BREVO_API_KEY", ""),
		openrouterAPIKey:                getEnv("OPENROUTER_API_KEY", ""),
		openrouterAPIURL:                getEnv("OPENROUTER_API_URL", "https://openrouter.ai/api/v1/chat/completions"),
		openrouterHTTPReferer:           getEnv("OPENROUTER_HTTP_REFERER", "https://github.com/aseno/backend"),
		openrouterClassifierTitle:       getEnv("OPENROUTER_CLASSIFIER_TITLE", "Aseno WhatsApp Classifier"),
		openrouterTranscriberTitle:      getEnv("OPENROUTER_TRANSCRIBER_TITLE", "Aseno WhatsApp Transcriber"),
	}
}

// Validate validates if all required configurations are present
func (c *Config) Validate() error {
	if c.databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.jwtSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	return nil
}

// ValidateForProduction validates configurations required for production
func (c *Config) ValidateForProduction() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.Environment == "production" {
		if c.googleClientID == "" {
			return fmt.Errorf("GOOGLE_CLIENT_ID is required in production")
		}
		if c.googleClientSecret == "" {
			return fmt.Errorf("GOOGLE_CLIENT_SECRET is required in production")
		}
	}
	return nil
}

// GetDatabaseURL returns the database connection string
func (c *Config) GetDatabaseURL() string {
	return c.databaseURL
}

// GetJWTSecret returns the JWT signing secret
func (c *Config) GetJWTSecret() string {
	return c.jwtSecret
}

// GetGoogleClientID returns the Google OAuth client ID
func (c *Config) GetGoogleClientID() string {
	return c.googleClientID
}

// GetGoogleClientSecret returns the Google OAuth client secret
func (c *Config) GetGoogleClientSecret() string {
	return c.googleClientSecret
}

// GetWhatsAppProvider returns the WhatsApp provider (evolution or waha)
func (c *Config) GetWhatsAppProvider() string {
	return c.whatsappProvider
}

// GetEvolutionAPIURL returns the Evolution API URL
func (c *Config) GetEvolutionAPIURL() string {
	return c.evolutionAPIURL
}

// GetEvolutionAPIKey returns the Evolution API key
func (c *Config) GetEvolutionAPIKey() string {
	return c.evolutionAPIKey
}

// GetWahaAPIURL returns the WAHA API URL
func (c *Config) GetWahaAPIURL() string {
	return c.wahaAPIURL
}

// GetWahaAPIKey returns the WAHA API key
func (c *Config) GetWahaAPIKey() string {
	return c.wahaAPIKey
}

// GetWhatsAppWebhookSecret returns the WhatsApp webhook secret
func (c *Config) GetWhatsAppWebhookSecret() string {
	return c.whatsappWebhookSecret
}

func (c *Config) GetWhatsAppWebhookURL() string {
	return c.whatsappWebhookURL
}

// GetAsaasAPIURL returns the Asaas API URL
func (c *Config) GetAsaasAPIURL() string {
	return c.asaasAPIURL
}

// GetAsaasAPIKey returns the Asaas API key
func (c *Config) GetAsaasAPIKey() string {
	return c.asaasAPIKey
}

// GetAsaasWebhookSecret returns the Asaas webhook secret
func (c *Config) GetAsaasWebhookSecret() string {
	return c.asaasWebhookSecret
}

// IsDevelopment returns true if environment is development
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if environment is production
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// GetSMTPHost returns the SMTP host
func (c *Config) GetSMTPHost() string {
	return c.smtpHost
}

// GetSMTPPort returns the SMTP port
func (c *Config) GetSMTPPort() int {
	return c.smtpPort
}

// GetSMTPUser returns the SMTP user
func (c *Config) GetSMTPUser() string {
	return c.smtpUser
}

// GetSMTPPassword returns the SMTP password
func (c *Config) GetSMTPPassword() string {
	return c.smtpPassword
}

// GetSMTPFrom returns the SMTP from address
func (c *Config) GetSMTPFrom() string {
	return c.smtpFrom
}

func (c *Config) GetEmailSenderSupportAddress() string {
	return c.emailSenderSupportAddress
}

func (c *Config) GetEmailSenderName() string {
	return c.emailSenderName
}

// GetFrontendURL returns the frontend URL
func (c *Config) GetFrontendURL() string {
	return c.frontendURL
}

// AIClassifierEnabled returns whether AI classification is enabled
func (c *Config) AIClassifierEnabled() bool {
	return c.aiClassifierEnabled
}

// AIClassifierProvider returns the AI provider (openrouter, claude, openai)
func (c *Config) AIClassifierProvider() string {
	return c.aiClassifierProvider
}

// AIClassifierModel returns the AI model to use
func (c *Config) AIClassifierModel() string {
	return c.aiClassifierModel
}

// AIClassifierTimeout returns the timeout in seconds for AI classification
func (c *Config) AIClassifierTimeout() int {
	return c.aiClassifierTimeout
}

// AIClassifierConfidenceThreshold returns the minimum confidence threshold
func (c *Config) AIClassifierConfidenceThreshold() float64 {
	return c.aiClassifierConfidenceThreshold
}

// GetGeminiAPIKey returns the Gemini API key
func (c *Config) GetGeminiAPIKey() string {
	return c.geminiAPIKey
}

// GetAnthropicAPIKey returns the Anthropic API key
func (c *Config) GetAnthropicAPIKey() string {
	return c.anthropicAPIKey
}

// GetOpenAIAPIKey returns the OpenAI API key
func (c *Config) GetOpenAIAPIKey() string {
	return c.openaiAPIKey
}

func (c *Config) GetBrevoAPIKey() string {
	return c.brevoAPIKey
}

func (c *Config) GetOpenRouterAPIKey() string {
	return c.openrouterAPIKey
}

func (c *Config) GetOpenRouterAPIURL() string {
	return c.openrouterAPIURL
}

func (c *Config) GetOpenRouterHTTPReferer() string {
	return c.openrouterHTTPReferer
}

func (c *Config) GetOpenRouterClassifierTitle() string {
	return c.openrouterClassifierTitle
}

func (c *Config) GetOpenRouterTranscriberTitle() string {
	return c.openrouterTranscriberTitle
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvAsFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return splitAndTrim(value, ",")
	}
	return defaultValue
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
