package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/amerfu/pllm/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

type Config struct {
	DSN             string
	MaxConnections  int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	LogLevel        logger.LogLevel
}

func Initialize(cfg *Config) error {
	if cfg.DSN == "" {
		cfg.DSN = os.Getenv("DATABASE_URL")
	}

	if cfg.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}

	// Set defaults
	if cfg.MaxConnections == 0 {
		cfg.MaxConnections = 100
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 10
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = time.Hour
	}
	if cfg.LogLevel == 0 {
		cfg.LogLevel = logger.Info
	}

	// Configure logger
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  cfg.LogLevel,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)

	// Open connection
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger:                                   newLogger,
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxConnections)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db

	// Run migrations
	if err := Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Check if database needs seeding (auto-seed if empty)
	if shouldAutoSeed() {
		log.Println("Database is empty, running initial seed...")
		if err := RunInitialSeed(); err != nil {
			log.Printf("Warning: Failed to seed database: %v", err)
			// Don't fail initialization if seeding fails
		}
	}

	return nil
}

func shouldAutoSeed() bool {
	// Skip seeding if disabled via environment variable
	if os.Getenv("DB_AUTO_SEED") == "false" {
		return false
	}

	// Check if database has any users
	var count int64
	DB.Model(&models.User{}).Count(&count)
	return count == 0
}

func RunInitialSeed() error {
	seeder := NewSeeder(DB)
	return seeder.SeedAll()
}

func Migrate() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// Create extensions (PostgreSQL specific, will be ignored for SQLite)
	DB.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")

	// Auto migrate all models including new ones
	if err := DB.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.TeamMember{},
		&models.Key{}, // Unified key model
		&models.Provider{},
		&models.Model{},
		&models.Budget{},
		&models.Usage{},
		&models.Audit{}, // New audit model
	); err != nil {
		return fmt.Errorf("failed to migrate models: %w", err)
	}

	// Create indexes
	if err := createIndexes(); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

func createIndexes() error {
	// User indexes
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_users_role ON users(role)")

	// New unified Key indexes
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_keys_key ON keys(key)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_keys_key_hash ON keys(key_hash)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_keys_key_prefix ON keys(key_prefix)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_keys_user_id ON keys(user_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_keys_team_id ON keys(team_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_keys_type ON keys(type)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_keys_is_active ON keys(is_active)")

	// Virtual Key indexes (legacy, for backward compatibility)
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_virtual_keys_key ON virtual_keys(key)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_virtual_keys_user_id ON virtual_keys(user_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_virtual_keys_team_id ON virtual_keys(team_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_virtual_keys_is_active ON virtual_keys(is_active)")

	// Team indexes
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_teams_name ON teams(name)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_teams_is_active ON teams(is_active)")

	// Team Member indexes
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_team_members_team_id ON team_members(team_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_team_members_team_user ON team_members(team_id, user_id)")

	// API Key indexes (legacy, kept for backward compatibility)
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON api_keys(key_prefix)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_api_keys_group_id ON api_keys(group_id)")

	// Usage indexes for analytics
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage_logs(timestamp)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_usage_user_id_timestamp ON usage_logs(user_id, timestamp)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_usage_group_id_timestamp ON usage_logs(group_id, timestamp)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_usage_provider_model ON usage_logs(provider, model)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_usage_request_id ON usage_logs(request_id)")

	// Budget indexes
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_budgets_user_id ON budgets(user_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_budgets_group_id ON budgets(group_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_budgets_type_period ON budgets(type, period)")

	// Provider indexes
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_providers_type ON providers(type)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_providers_is_active ON providers(is_active)")

	// Model indexes
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_models_provider_id ON models(provider_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_models_is_active ON models(is_active)")

	// User indexes for external identity
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_users_external_id ON users(external_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_users_external_provider ON users(external_provider)")

	// Audit indexes
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audits(timestamp)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audits(event_type)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_event_result ON audits(event_result)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_user_id ON audits(user_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_team_id ON audits(team_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_key_id ON audits(key_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_ip_address ON audits(ip_address)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audits(request_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_resource_type ON audits(resource_type)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_audit_resource_id ON audits(resource_id)")

	return nil
}

func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

func GetDB() *gorm.DB {
	return DB
}

func IsHealthy() bool {
	if DB == nil {
		return false
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return false
	}

	if err := sqlDB.Ping(); err != nil {
		return false
	}

	return true
}

// TestConnection tests if a database connection can be established
func TestConnection(ctx context.Context, cfg *Config) error {
	if cfg.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}

	// Configure logger
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Silent, // Silent for test
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// Open connection
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Get SQL database
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}
	defer sqlDB.Close()

	// Test connection with context
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}
