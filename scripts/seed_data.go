package main

import (
	"fmt"
	"log"
	"time"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/database"
	"github.com/amerfu/pllm/internal/models"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	_ = godotenv.Load("../.env")

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize database
	dbConfig := &database.Config{
		DSN:             cfg.Database.URL,
		MaxConnections:  cfg.Database.MaxConnections,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}

	if err := database.Initialize(dbConfig); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer database.Close()

	db := database.GetDB()

	// Create test team
	team := &models.Team{
		ID:          uuid.New(),
		Name:        "Demo Team",
		Description: "Demo team for testing",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := db.Create(team).Error; err != nil {
		log.Println("Team might already exist:", err)
	} else {
		fmt.Println("Created team:", team.Name)
	}

	// Create test budget for team
	budget := &models.Budget{
		ID:        uuid.New(),
		Name:      "Demo Team Budget",
		Type:      models.BudgetTypeTeam,
		TeamID:    &team.ID,
		Amount:    1000.00,
		Spent:     0,
		Period:    models.BudgetPeriodMonthly,
		AlertAt:   80.0,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		StartsAt:  time.Now(),
		EndsAt:    time.Now().AddDate(0, 1, 0),
	}
	if err := db.Create(budget).Error; err != nil {
		log.Println("Budget might already exist:", err)
	} else {
		fmt.Println("Created budget:", budget.Name)
	}

	// Create test virtual key
	key := &models.VirtualKey{
		ID:           uuid.New(),
		Key:          "sk-demo-" + uuid.New().String()[:8],
		Name:         "Demo API Key",
		TeamID:       &team.ID,
		IsActive:     true,
		MaxBudget:    500.00,
		CurrentSpend: 0,
		TPM:          1000000,
		RPM:          10000,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(key).Error; err != nil {
		log.Println("Key might already exist:", err)
	} else {
		fmt.Println("Created virtual key:", key.Key)
	}

	fmt.Println("\nSeed data created successfully!")
	fmt.Println("You can now test the Teams and Keys pages in the UI")
}