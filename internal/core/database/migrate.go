package database

import (
	"fmt"
	"log"

	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// AutoMigrate runs database migrations
func AutoMigrate(db *gorm.DB) error {
	log.Println("Running database migrations...")

	// Auto-migrate all models
	err := db.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.TeamMember{},
		&models.Key{},       // Unified key model
		&models.Budget{},
		&models.Usage{},
		&models.Audit{},     // Audit logging
		&models.UserModel{}, // User-created model configurations
		&models.Route{},     // Route configurations
		&models.RouteModel{}, // Route model entries
	)

	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// IsEmpty checks if the database is empty (no users)
func IsEmpty(db *gorm.DB) bool {
	var count int64
	db.Model(&models.User{}).Count(&count)
	return count == 0
}

// InitializeDatabase runs migrations and seeds if database is empty
func InitializeDatabase(db *gorm.DB, runSeeder bool) error {
	// Always run migrations to ensure schema is up to date
	if err := AutoMigrate(db); err != nil {
		return err
	}

	// Check if database needs seeding
	if runSeeder && IsEmpty(db) {
		log.Println("Database is empty, running initial seed...")
		seeder := NewSeeder(db)
		if err := seeder.SeedAll(); err != nil {
			return fmt.Errorf("failed to seed database: %w", err)
		}

		// Print summary
		printInitialSeedSummary(db)
	}

	return nil
}

func printInitialSeedSummary(db *gorm.DB) {
	var userCount, teamCount, keyCount int64
	db.Model(&models.User{}).Count(&userCount)
	db.Model(&models.Team{}).Count(&teamCount)
	db.Model(&models.Key{}).Count(&keyCount)

	log.Println("========================================")
	log.Println("Database initialized with seed data:")
	log.Printf("  • Users: %d", userCount)
	log.Printf("  • Teams: %d", teamCount)
	log.Printf("  • API Keys: %d", keyCount)
	log.Println("----------------------------------------")
	log.Println("Default authentication:")
	log.Println("  Method: Dex OAuth (configure Dex for user authentication)")
	log.Println("  Admin: Use PLLM_MASTER_KEY for bootstrap admin operations")
	log.Println("========================================")
	log.Println("⚠️  IMPORTANT: Configure Dex OAuth provider and set master key!")
	log.Println("========================================")
}
