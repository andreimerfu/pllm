package database

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/models"
)

type Seeder struct {
	db *gorm.DB
}

func NewSeeder(db *gorm.DB) *Seeder {
	return &Seeder{db: db}
}

// SeedAll runs all seeders
func (s *Seeder) SeedAll() error {
	log.Println("Starting database seeding...")

	// Note: Users are auto-provisioned via Dex OAuth
	// No default users or keys are seeded

	if err := s.SeedTeams(); err != nil {
		return fmt.Errorf("failed to seed teams: %w", err)
	}

	if err := s.SeedBudgets(); err != nil {
		return fmt.Errorf("failed to seed budgets: %w", err)
	}

	log.Println("Database seeding completed successfully!")
	return nil
}

// SeedUsers - Users are auto-provisioned via Dex OAuth, no seeding needed
func (s *Seeder) SeedUsers() error {
	log.Println("Skipping user seeding - users auto-provisioned via Dex OAuth")
	return nil
}

// SeedUsersLegacy - Legacy user seeding (kept for reference, not used)
func (s *Seeder) SeedUsersLegacy() error {
	log.Println("Legacy user seeding disabled - users auto-provisioned via Dex OAuth")
	return nil

	/*
		// This code is commented out since we use Dex OAuth now
		// Hash passwords
		// adminPass, _ := bcrypt.GenerateFromPassword([]byte("admin123"), 12)
		// userPass, _ := bcrypt.GenerateFromPassword([]byte("user123"), 12)
		// demoPass, _ := bcrypt.GenerateFromPassword([]byte("demo123"), 12)

		/*
		users := []models.User{
			{
				BaseModel: models.BaseModel{
					ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				},
				Email:          "admin@pllm.local",
				Username:       "admin",
				Password:       string(adminPass),
				FirstName:      "Admin",
				LastName:       "User",
				Role:           models.RoleAdmin,
				IsActive:       true,
				EmailVerified:  true,
				MaxBudget:      10000,
				BudgetDuration: models.BudgetPeriodMonthly,
				BudgetResetAt:  time.Now().AddDate(0, 1, 0),
				TPM:            1000000,
				RPM:            1000,
			},
			{
				BaseModel: models.BaseModel{
					ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				},
				Email:          "manager@pllm.local",
				Username:       "manager",
				Password:       string(userPass),
				FirstName:      "Manager",
				LastName:       "User",
				Role:           models.RoleManager,
				IsActive:       true,
				EmailVerified:  true,
				MaxBudget:      5000,
				BudgetDuration: models.BudgetPeriodMonthly,
				BudgetResetAt:  time.Now().AddDate(0, 1, 0),
				TPM:            500000,
				RPM:            500,
			},
			{
				BaseModel: models.BaseModel{
					ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
				},
				Email:          "user@pllm.local",
				Username:       "user",
				Password:       string(userPass),
				FirstName:      "Regular",
				LastName:       "User",
				Role:           models.RoleUser,
				IsActive:       true,
				EmailVerified:  true,
				MaxBudget:      1000,
				BudgetDuration: models.BudgetPeriodMonthly,
				BudgetResetAt:  time.Now().AddDate(0, 1, 0),
				TPM:            100000,
				RPM:            100,
			},
			{
				BaseModel: models.BaseModel{
					ID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
				},
				Email:          "demo@pllm.local",
				Username:       "demo",
				Password:       string(demoPass),
				FirstName:      "Demo",
				LastName:       "Account",
				Role:           models.RoleViewer,
				IsActive:       true,
				EmailVerified:  true,
				MaxBudget:      100,
				BudgetDuration: models.BudgetPeriodDaily,
				BudgetResetAt:  time.Now().AddDate(0, 0, 1),
				TPM:            10000,
				RPM:            10,
			},
		}

		for _, user := range users {
			var existingUser models.User
			if err := s.db.Where("email = ?", user.Email).First(&existingUser).Error; err == nil {
				log.Printf("User %s already exists, skipping...", user.Email)
				continue
			}

			if err := s.db.Create(&user).Error; err != nil {
				return fmt.Errorf("failed to create user %s: %w", user.Email, err)
			}
			log.Printf("Created user: %s", user.Email)
		}

		return nil
	*/
}

// SeedTeams creates default teams and assigns members
func (s *Seeder) SeedTeams() error {
	log.Println("Seeding teams...")

	teams := []models.Team{
		{
			BaseModel: models.BaseModel{
				ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
			},
			Name:             "Engineering",
			Description:      "Engineering team with full model access",
			MaxBudget:        50000,
			BudgetDuration:   models.BudgetPeriodMonthly,
			BudgetResetAt:    time.Now().AddDate(0, 1, 0),
			IsActive:         true,
			TPM:              5000000,
			RPM:              5000,
			MaxParallelCalls: 100,
			AllowedModels:    []string{"*"},
		},
		{
			BaseModel: models.BaseModel{
				ID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
			},
			Name:             "Marketing",
			Description:      "Marketing team with limited model access",
			MaxBudget:        10000,
			BudgetDuration:   models.BudgetPeriodMonthly,
			BudgetResetAt:    time.Now().AddDate(0, 1, 0),
			IsActive:         true,
			TPM:              1000000,
			RPM:              1000,
			MaxParallelCalls: 20,
			AllowedModels:    []string{"gpt-3.5-turbo", "gpt-4", "claude-3-haiku", "claude-3-sonnet"},
		},
		{
			BaseModel: models.BaseModel{
				ID: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
			},
			Name:             "Research",
			Description:      "Research team with access to advanced models",
			MaxBudget:        100000,
			BudgetDuration:   models.BudgetPeriodMonthly,
			BudgetResetAt:    time.Now().AddDate(0, 1, 0),
			IsActive:         true,
			TPM:              10000000,
			RPM:              10000,
			MaxParallelCalls: 200,
			AllowedModels:    []string{"gpt-4", "gpt-4-turbo", "claude-3-opus", "claude-3-sonnet"},
			BlockedModels:    []string{"gpt-3.5-turbo"},
		},
	}

	for _, team := range teams {
		var existingTeam models.Team
		if err := s.db.Where("name = ?", team.Name).First(&existingTeam).Error; err == nil {
			log.Printf("Team %s already exists, skipping...", team.Name)
			continue
		}

		if err := s.db.Create(&team).Error; err != nil {
			return fmt.Errorf("failed to create team %s: %w", team.Name, err)
		}
		log.Printf("Created team: %s", team.Name)
	}

	// Assign users to teams
	teamMemberships := []models.TeamMember{
		{
			TeamID:   uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), // Engineering
			UserID:   uuid.MustParse("11111111-1111-1111-1111-111111111111"), // Admin
			Role:     models.TeamRoleOwner,
			JoinedAt: time.Now(),
		},
		{
			TeamID:   uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), // Engineering
			UserID:   uuid.MustParse("22222222-2222-2222-2222-222222222222"), // Manager
			Role:     models.TeamRoleAdmin,
			JoinedAt: time.Now(),
		},
		{
			TeamID:   uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), // Engineering
			UserID:   uuid.MustParse("33333333-3333-3333-3333-333333333333"), // User
			Role:     models.TeamRoleMember,
			JoinedAt: time.Now(),
		},
		{
			TeamID:   uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), // Marketing
			UserID:   uuid.MustParse("22222222-2222-2222-2222-222222222222"), // Manager
			Role:     models.TeamRoleOwner,
			JoinedAt: time.Now(),
		},
		{
			TeamID:   uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), // Marketing
			UserID:   uuid.MustParse("44444444-4444-4444-4444-444444444444"), // Demo
			Role:     models.TeamRoleViewer,
			JoinedAt: time.Now(),
		},
	}

	for _, membership := range teamMemberships {
		var existing models.TeamMember
		if err := s.db.Where("team_id = ? AND user_id = ?", membership.TeamID, membership.UserID).First(&existing).Error; err == nil {
			log.Printf("Team membership already exists, skipping...")
			continue
		}

		if err := s.db.Create(&membership).Error; err != nil {
			log.Printf("Warning: failed to create team membership: %v", err)
		}
	}

	return nil
}

// SeedVirtualKeys - Keys are created by authenticated users, no seeding needed
func (s *Seeder) SeedVirtualKeys() error {
	log.Println("Skipping key seeding - keys created by authenticated users")
	return nil
}

// SeedVirtualKeysLegacy - Legacy key seeding (kept for reference, not used)
func (s *Seeder) SeedVirtualKeysLegacy() error {
	log.Println("Legacy virtual key seeding disabled")
	return nil

	/*
		// User keys
		userID1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		userID2 := uuid.MustParse("33333333-3333-3333-3333-333333333333")

		// Team keys
		teamID1 := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
		teamID2 := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

		// Budget settings
		monthlyBudget := float64(1000)
		dailyBudget := float64(100)
		monthlyDuration := models.BudgetPeriodMonthly
		dailyDuration := models.BudgetPeriodDaily

		// Rate limits
		highTPM := 1000000
		highRPM := 1000
		lowTPM := 100000
		lowRPM := 100

		/*
		keys := []models.VirtualKey{
			{
				Key:            "sk-admin-full-access-" + generateRandomString(32),
				Name:           "Admin Full Access Key",
				UserID:         &userID1,
				IsActive:       true,
				MaxBudget:      &monthlyBudget,
				BudgetDuration: &monthlyDuration,
				TPM:            &highTPM,
				RPM:            &highRPM,
				AllowedModels:  []string{"*"},
			},
			{
				Key:            "sk-user-limited-" + generateRandomString(32),
				Name:           "User Limited Key",
				UserID:         &userID2,
				IsActive:       true,
				MaxBudget:      &dailyBudget,
				BudgetDuration: &dailyDuration,
				TPM:            &lowTPM,
				RPM:            &lowRPM,
				AllowedModels:  []string{"gpt-3.5-turbo", "claude-3-haiku"},
			},
			{
				Key:            "sk-team-engineering-" + generateRandomString(32),
				Name:           "Engineering Team Key",
				TeamID:         &teamID1,
				IsActive:       true,
				MaxBudget:      &monthlyBudget,
				BudgetDuration: &monthlyDuration,
				TPM:            &highTPM,
				RPM:            &highRPM,
			},
			{
				Key:            "sk-team-marketing-" + generateRandomString(32),
				Name:           "Marketing Team Key",
				TeamID:         &teamID2,
				IsActive:       true,
				MaxBudget:      &dailyBudget,
				BudgetDuration: &dailyDuration,
				TPM:            &lowTPM,
				RPM:            &lowRPM,
				AllowedModels:  []string{"gpt-3.5-turbo", "gpt-4"},
			},
			{
				Key:            "sk-demo-readonly-" + generateRandomString(32),
				Name:           "Demo Read-Only Key",
				UserID:         &userID2,
				IsActive:       true,
				ExpiresAt:      timePtr(time.Now().AddDate(0, 0, 7)), // Expires in 7 days
				MaxBudget:      floatPtr(10),
				BudgetDuration: &dailyDuration,
				TPM:            intPtr(10000),
				RPM:            intPtr(10),
				AllowedModels:  []string{"gpt-3.5-turbo"},
				Tags:           []string{"demo", "limited"},
			},
		}

		for _, key := range keys {
			var existingKey models.VirtualKey
			if err := s.db.Where("name = ?", key.Name).First(&existingKey).Error; err == nil {
				log.Printf("Key %s already exists, skipping...", key.Name)
				continue
			}

			// Set budget reset time if budget is configured
			if key.BudgetDuration != nil {
				now := time.Now()
				switch *key.BudgetDuration {
				case models.BudgetPeriodDaily:
					key.BudgetResetAt = timePtr(now.AddDate(0, 0, 1))
				case models.BudgetPeriodWeekly:
					key.BudgetResetAt = timePtr(now.AddDate(0, 0, 7))
				case models.BudgetPeriodMonthly:
					key.BudgetResetAt = timePtr(now.AddDate(0, 1, 0))
				case models.BudgetPeriodYearly:
					key.BudgetResetAt = timePtr(now.AddDate(1, 0, 0))
				}
			}

			if err := s.db.Create(&key).Error; err != nil {
				return fmt.Errorf("failed to create key %s: %w", key.Name, err)
			}
			log.Printf("Created virtual key: %s (Key: %s...)", key.Name, key.Key[:20])
		}

		return nil
	*/
}

// SeedBudgets creates sample budgets
func (s *Seeder) SeedBudgets() error {
	log.Println("Seeding budgets...")

	userID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	teamID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	budgets := []models.Budget{
		{
			Name:     "Global Monthly Budget",
			Type:     models.BudgetTypeGlobal,
			Amount:   100000,
			Period:   models.BudgetPeriodMonthly,
			StartsAt: time.Now(),
			EndsAt:   time.Now().AddDate(0, 1, 0),
			IsActive: true,
			AlertAt:  80,
			Actions: models.BudgetActions{
				{
					Threshold: 50,
					Action:    "alert",
				},
				{
					Threshold: 80,
					Action:    "alert",
				},
				{
					Threshold: 95,
					Action:    "throttle",
				},
				{
					Threshold: 100,
					Action:    "block",
				},
			},
		},
		{
			Name:     "User Daily Budget",
			Type:     models.BudgetTypeUser,
			UserID:   &userID,
			Amount:   50,
			Period:   models.BudgetPeriodDaily,
			StartsAt: time.Now(),
			EndsAt:   time.Now().AddDate(0, 0, 1),
			IsActive: true,
			AlertAt:  75,
		},
		{
			Name:     "Engineering Team Budget",
			Type:     models.BudgetTypeTeam,
			TeamID:   &teamID,
			Amount:   10000,
			Period:   models.BudgetPeriodMonthly,
			StartsAt: time.Now(),
			EndsAt:   time.Now().AddDate(0, 1, 0),
			IsActive: true,
			AlertAt:  90,
		},
	}

	for _, budget := range budgets {
		var existingBudget models.Budget
		if err := s.db.Where("name = ?", budget.Name).First(&existingBudget).Error; err == nil {
			log.Printf("Budget %s already exists, skipping...", budget.Name)
			continue
		}

		if err := s.db.Create(&budget).Error; err != nil {
			return fmt.Errorf("failed to create budget %s: %w", budget.Name, err)
		}
		log.Printf("Created budget: %s", budget.Name)
	}

	return nil
}
