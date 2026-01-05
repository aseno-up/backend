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
	googleClientID     string
	googleClientSecret string
	GoogleRedirectURL  string

	// Evolution API / WhatsApp (private - sensitive)
	evolutionAPIURL   string
	evolutionAPIKey   string
	EvolutionInstance string

	// Asaas Payment (private - sensitive)
	asaasAPIURL string
	asaasAPIKey string
	AsaasSandbox bool

	// CORS
	AllowedOrigins []string

	// Logging
	LogLevel string
}

// NewConfig creates a new configuration from environment variables
func NewConfig() *Config {
	// Load .env file if exists (development)
	_ = godotenv.Load()

	return &Config{
		Port:               getEnv("PORT", "8080"),
		Environment:        getEnv("ENVIRONMENT", "development"),
		databaseURL:        getEnv("DATABASE_URL", ""),
		jwtSecret:          getEnv("JWT_SECRET", ""),
		JWTAccessTokenTTL:  getEnvAsInt("JWT_ACCESS_TOKEN_TTL", 15),
		JWTRefreshTokenTTL: getEnvAsInt("JWT_REFRESH_TOKEN_TTL", 7),
		googleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		googleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", ""),
		evolutionAPIURL:    getEnv("EVOLUTION_API_URL", ""),
		evolutionAPIKey:    getEnv("EVOLUTION_API_KEY", ""),
		EvolutionInstance:  getEnv("EVOLUTION_INSTANCE", ""),
		asaasAPIURL:        getEnv("ASAAS_API_URL", "https://sandbox.asaas.com/api/v3"),
		asaasAPIKey:        getEnv("ASAAS_API_KEY", ""),
		AsaasSandbox:       getEnvAsBool("ASAAS_SANDBOX", true),
		AllowedOrigins:     getEnvAsSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
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

// GetEvolutionAPIURL returns the Evolution API URL
func (c *Config) GetEvolutionAPIURL() string {
	return c.evolutionAPIURL
}

// GetEvolutionAPIKey returns the Evolution API key
func (c *Config) GetEvolutionAPIKey() string {
	return c.evolutionAPIKey
}

// GetAsaasAPIURL returns the Asaas API URL
func (c *Config) GetAsaasAPIURL() string {
	return c.asaasAPIURL
}

// GetAsaasAPIKey returns the Asaas API key
func (c *Config) GetAsaasAPIKey() string {
	return c.asaasAPIKey
}

// IsDevelopment returns true if environment is development
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if environment is production
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
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
