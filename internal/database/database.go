package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Database wraps the GORM DB connection
type Database struct {
	db *gorm.DB
}

// NewDatabase creates a new database connection
func NewDatabase(dsn string, isDebug bool) (*Database, error) {
	logLevel := gormlogger.Silent
	if isDebug {
		logLevel = gormlogger.Info
	}

	gormLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	// Parse DSN with pgx to set search_path as a startup parameter.
	// This ensures every connection from the pool gets the correct schema,
	// even through Supabase's connection pooler (Supavisor on port 6543).
	pgxConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database DSN: %w", err)
	}
	pgxConfig.RuntimeParams["search_path"] = "aseno"

	// Disable prepared statements at the pgx level because Supabase's PgBouncer
	// (in transaction mode) does not support them and throws "prepared statement already exists".
	pgxConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Open connection pool via pgx stdlib (compatible with database/sql)
	sqlDB := stdlib.OpenDB(*pgxConfig)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true, // disable prepared statements for connection pooler compatibility
	}), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Ensure the schema exists so AutoMigrate doesn't fail with "no schema has been selected to create in"
	db.Exec("CREATE SCHEMA IF NOT EXISTS aseno")

	return &Database{db: db}, nil
}

// GetDB returns the underlying GORM DB instance
func (d *Database) GetDB() *gorm.DB {
	return d.db
}

// Close closes the database connection
func (d *Database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping checks if the database connection is alive
func (d *Database) Ping() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// AutoMigrate runs auto-migration for the given models
func (d *Database) AutoMigrate(models ...interface{}) error {
	return d.db.AutoMigrate(models...)
}

// Transaction executes a function within a database transaction
func (d *Database) Transaction(fn func(tx *gorm.DB) error) error {
	return d.db.Transaction(fn)
}

// WithTenant returns a new DB instance scoped to a specific clinic (tenant)
func (d *Database) WithTenant(clinicID string) *gorm.DB {
	return d.db.Where("clinic_id = ?", clinicID)
}
