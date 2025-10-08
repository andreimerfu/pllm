package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestTeam_BudgetManagement(t *testing.T) {
	t.Run("IsBudgetExceeded", func(t *testing.T) {
		team := &Team{
			BaseModel:    BaseModel{ID: uuid.New()},
			Name:         "Test Team",
			MaxBudget:    1000.0,
			CurrentSpend: 500.0,
		}

		// Within budget
		assert.False(t, team.IsBudgetExceeded())

		// At budget limit
		team.CurrentSpend = 1000.0
		assert.True(t, team.IsBudgetExceeded())

		// Over budget
		team.CurrentSpend = 1500.0
		assert.True(t, team.IsBudgetExceeded())

		// No budget limit (unlimited)
		team.MaxBudget = 0
		assert.False(t, team.IsBudgetExceeded())
	})

	t.Run("ShouldAlertBudget", func(t *testing.T) {
		team := &Team{
			BaseModel:     BaseModel{ID: uuid.New()},
			Name:          "Test Team",
			MaxBudget:     1000.0,
			CurrentSpend:  0.0,
			BudgetAlertAt: 80.0,
		}

		// Below alert threshold
		team.CurrentSpend = 500.0 // 50%
		assert.False(t, team.ShouldAlertBudget())

		// At alert threshold
		team.CurrentSpend = 800.0 // 80%
		assert.True(t, team.ShouldAlertBudget())

		// Above alert threshold
		team.CurrentSpend = 950.0 // 95%
		assert.True(t, team.ShouldAlertBudget())

		// No budget limit
		team.MaxBudget = 0
		assert.False(t, team.ShouldAlertBudget())
	})

	t.Run("ShouldResetBudget", func(t *testing.T) {
		team := &Team{
			BaseModel:     BaseModel{ID: uuid.New()},
			Name:          "Test Team",
			MaxBudget:     1000.0,
			BudgetResetAt: time.Now().Add(1 * time.Hour),
		}

		// Before reset time
		assert.False(t, team.ShouldResetBudget())

		// After reset time
		team.BudgetResetAt = time.Now().Add(-1 * time.Hour)
		assert.True(t, team.ShouldResetBudget())
	})

	t.Run("ResetBudget", func(t *testing.T) {
		now := time.Now()

		testCases := []struct {
			name           string
			period         BudgetPeriod
			expectedDelta  time.Duration
			deltaRange     time.Duration
		}{
			{"Daily", BudgetPeriodDaily, 24 * time.Hour, 1 * time.Hour},
			{"Weekly", BudgetPeriodWeekly, 7 * 24 * time.Hour, 1 * time.Hour},
			{"Monthly", BudgetPeriodMonthly, 30 * 24 * time.Hour, 2 * 24 * time.Hour},
			{"Yearly", BudgetPeriodYearly, 365 * 24 * time.Hour, 2 * 24 * time.Hour},
			{"Custom", BudgetPeriodCustom, 30 * 24 * time.Hour, 2 * 24 * time.Hour},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				team := &Team{
					BaseModel:      BaseModel{ID: uuid.New()},
					Name:           "Test Team",
					MaxBudget:      1000.0,
					CurrentSpend:   750.0,
					BudgetDuration: tc.period,
					BudgetResetAt:  now.Add(-1 * time.Hour),
				}

				team.ResetBudget()

				// Budget should be reset
				assert.Equal(t, 0.0, team.CurrentSpend)

				// Reset time should be in the future
				assert.True(t, team.BudgetResetAt.After(now))

				// Check expected time delta (with range tolerance)
				actualDelta := team.BudgetResetAt.Sub(now)
				assert.InDelta(t, tc.expectedDelta.Seconds(), actualDelta.Seconds(), tc.deltaRange.Seconds())
			})
		}
	})

	t.Run("Budget_Increase_During_Active_Usage", func(t *testing.T) {
		team := &Team{
			BaseModel:    BaseModel{ID: uuid.New()},
			Name:         "Test Team",
			MaxBudget:    100.0,
			CurrentSpend: 90.0,
		}

		// Budget not exceeded yet (90 < 100)
		assert.False(t, team.IsBudgetExceeded())

		// Increase budget
		team.MaxBudget = 200.0

		// Should no longer be exceeded
		assert.False(t, team.IsBudgetExceeded())

		// Continue spending
		team.CurrentSpend = 150.0
		assert.False(t, team.IsBudgetExceeded())

		// Exceed new limit
		team.CurrentSpend = 200.0
		assert.True(t, team.IsBudgetExceeded())
	})

	t.Run("Budget_Decrease_During_Active_Usage", func(t *testing.T) {
		team := &Team{
			BaseModel:    BaseModel{ID: uuid.New()},
			Name:         "Test Team",
			MaxBudget:    1000.0,
			CurrentSpend: 500.0,
		}

		// Within budget
		assert.False(t, team.IsBudgetExceeded())

		// Decrease budget below current spend
		team.MaxBudget = 400.0

		// Should now be exceeded
		assert.True(t, team.IsBudgetExceeded())
	})
}

func TestTeam_ModelAccess(t *testing.T) {
	t.Run("IsModelAllowed_NoRestrictions", func(t *testing.T) {
		team := &Team{
			BaseModel:     BaseModel{ID: uuid.New()},
			Name:          "Test Team",
			AllowedModels: []string{},
			BlockedModels: []string{},
		}

		// All models allowed by default
		assert.True(t, team.IsModelAllowed("gpt-4"))
		assert.True(t, team.IsModelAllowed("gpt-3.5-turbo"))
		assert.True(t, team.IsModelAllowed("claude-3-opus"))
	})

	t.Run("IsModelAllowed_WithAllowedList", func(t *testing.T) {
		team := &Team{
			BaseModel:     BaseModel{ID: uuid.New()},
			Name:          "Test Team",
			AllowedModels: []string{"gpt-4", "gpt-4-turbo"},
			BlockedModels: []string{},
		}

		// Allowed models
		assert.True(t, team.IsModelAllowed("gpt-4"))
		assert.True(t, team.IsModelAllowed("gpt-4-turbo"))

		// Not in allowed list
		assert.False(t, team.IsModelAllowed("gpt-3.5-turbo"))
		assert.False(t, team.IsModelAllowed("claude-3-opus"))
	})

	t.Run("IsModelAllowed_WithBlockedList", func(t *testing.T) {
		team := &Team{
			BaseModel:     BaseModel{ID: uuid.New()},
			Name:          "Test Team",
			AllowedModels: []string{},
			BlockedModels: []string{"gpt-3.5-turbo"},
		}

		// Not blocked
		assert.True(t, team.IsModelAllowed("gpt-4"))
		assert.True(t, team.IsModelAllowed("claude-3-opus"))

		// Blocked
		assert.False(t, team.IsModelAllowed("gpt-3.5-turbo"))
	})

	t.Run("IsModelAllowed_BlockedTakesPrecedence", func(t *testing.T) {
		team := &Team{
			BaseModel:     BaseModel{ID: uuid.New()},
			Name:          "Test Team",
			AllowedModels: []string{"gpt-4"},
			BlockedModels: []string{"gpt-4"},
		}

		// Blocked takes precedence over allowed
		assert.False(t, team.IsModelAllowed("gpt-4"))
	})

	t.Run("IsModelAllowed_WildcardAllowed", func(t *testing.T) {
		team := &Team{
			BaseModel:     BaseModel{ID: uuid.New()},
			Name:          "Test Team",
			AllowedModels: []string{"*"},
			BlockedModels: []string{},
		}

		// All models allowed
		assert.True(t, team.IsModelAllowed("gpt-4"))
		assert.True(t, team.IsModelAllowed("any-model"))
	})

	t.Run("IsModelAllowed_WildcardBlocked", func(t *testing.T) {
		team := &Team{
			BaseModel:     BaseModel{ID: uuid.New()},
			Name:          "Test Team",
			AllowedModels: []string{},
			BlockedModels: []string{"*"},
		}

		// All models blocked
		assert.False(t, team.IsModelAllowed("gpt-4"))
		assert.False(t, team.IsModelAllowed("any-model"))
	})
}

func TestTeamMember_BudgetManagement(t *testing.T) {
	teamBudget := 1000.0
	teamTPM := 5000
	teamRPM := 100

	t.Run("GetEffectiveBudget_UseTeamDefault", func(t *testing.T) {
		member := &TeamMember{
			ID:           uuid.New(),
			MaxBudget:    nil,
			CurrentSpend: 0.0,
		}

		assert.Equal(t, teamBudget, member.GetEffectiveBudget(teamBudget))
	})

	t.Run("GetEffectiveBudget_UseMemberOverride", func(t *testing.T) {
		memberBudget := 500.0
		member := &TeamMember{
			ID:           uuid.New(),
			MaxBudget:    &memberBudget,
			CurrentSpend: 0.0,
		}

		assert.Equal(t, memberBudget, member.GetEffectiveBudget(teamBudget))
	})

	t.Run("GetEffectiveTPM_UseTeamDefault", func(t *testing.T) {
		member := &TeamMember{
			ID:        uuid.New(),
			CustomTPM: nil,
		}

		assert.Equal(t, teamTPM, member.GetEffectiveTPM(teamTPM))
	})

	t.Run("GetEffectiveTPM_UseMemberOverride", func(t *testing.T) {
		memberTPM := 2000
		member := &TeamMember{
			ID:        uuid.New(),
			CustomTPM: &memberTPM,
		}

		assert.Equal(t, memberTPM, member.GetEffectiveTPM(teamTPM))
	})

	t.Run("GetEffectiveRPM_UseTeamDefault", func(t *testing.T) {
		member := &TeamMember{
			ID:        uuid.New(),
			CustomRPM: nil,
		}

		assert.Equal(t, teamRPM, member.GetEffectiveRPM(teamRPM))
	})

	t.Run("GetEffectiveRPM_UseMemberOverride", func(t *testing.T) {
		memberRPM := 50
		member := &TeamMember{
			ID:        uuid.New(),
			CustomRPM: &memberRPM,
		}

		assert.Equal(t, memberRPM, member.GetEffectiveRPM(teamRPM))
	})

	t.Run("IsBudgetExceeded_WithTeamBudget", func(t *testing.T) {
		member := &TeamMember{
			ID:           uuid.New(),
			MaxBudget:    nil,
			CurrentSpend: 500.0,
		}

		// Within team budget
		assert.False(t, member.IsBudgetExceeded(teamBudget))

		// Exceed team budget
		member.CurrentSpend = 1000.0
		assert.True(t, member.IsBudgetExceeded(teamBudget))
	})

	t.Run("IsBudgetExceeded_WithMemberBudget", func(t *testing.T) {
		memberBudget := 300.0
		member := &TeamMember{
			ID:           uuid.New(),
			MaxBudget:    &memberBudget,
			CurrentSpend: 200.0,
		}

		// Within member budget
		assert.False(t, member.IsBudgetExceeded(teamBudget))

		// Exceed member budget (even though under team budget)
		member.CurrentSpend = 300.0
		assert.True(t, member.IsBudgetExceeded(teamBudget))
	})

	t.Run("IsBudgetExceeded_NoBudgetLimit", func(t *testing.T) {
		member := &TeamMember{
			ID:           uuid.New(),
			MaxBudget:    nil,
			CurrentSpend: 5000.0,
		}

		// No team budget limit
		assert.False(t, member.IsBudgetExceeded(0))

		// Zero budget with member override also means no limit
		zeroBudget := 0.0
		member.MaxBudget = &zeroBudget
		assert.False(t, member.IsBudgetExceeded(teamBudget))
	})
}

func TestTeam_ComplexScenarios(t *testing.T) {
	t.Run("Team_Budget_Reset_With_Members", func(t *testing.T) {
		team := &Team{
			BaseModel:      BaseModel{ID: uuid.New()},
			Name:           "Test Team",
			MaxBudget:      1000.0,
			CurrentSpend:   800.0,
			BudgetDuration: BudgetPeriodMonthly,
			BudgetResetAt:  time.Now().Add(-1 * time.Hour),
		}

		memberBudget := 500.0
		members := []TeamMember{
			{
				ID:           uuid.New(),
				TeamID:       team.ID,
				MaxBudget:    &memberBudget,
				CurrentSpend: 300.0,
			},
			{
				ID:           uuid.New(),
				TeamID:       team.ID,
				MaxBudget:    nil,
				CurrentSpend: 500.0,
			},
		}

		// Team budget should reset
		assert.True(t, team.ShouldResetBudget())
		team.ResetBudget()
		assert.Equal(t, 0.0, team.CurrentSpend)

		// Members still have their own spend tracking
		assert.Equal(t, 300.0, members[0].CurrentSpend)
		assert.Equal(t, 500.0, members[1].CurrentSpend)
	})

	t.Run("Team_With_Metadata_And_Settings", func(t *testing.T) {
		settings := map[string]interface{}{
			"webhook_url":          "https://example.com/webhook",
			"notification_emails":  []string{"admin@example.com"},
			"alert_on_budget":      true,
			"enable_caching":       true,
			"cache_ttl":            3600,
		}

		metadata := map[string]interface{}{
			"department": "Engineering",
			"cost_center": "CC-123",
		}

		settingsJSON, _ := datatypes.NewJSONType(settings).MarshalJSON()
		metadataJSON, _ := datatypes.NewJSONType(metadata).MarshalJSON()

		team := &Team{
			BaseModel:   BaseModel{ID: uuid.New()},
			Name:        "Engineering Team",
			MaxBudget:   5000.0,
			Settings:    settingsJSON,
			Metadata:    metadataJSON,
		}

		require.NotNil(t, team.Settings)
		require.NotNil(t, team.Metadata)
	})

	t.Run("Team_Budget_Alert_Threshold_Variations", func(t *testing.T) {
		testCases := []struct {
			name           string
			maxBudget      float64
			currentSpend   float64
			budgetAlertAt  float64
			shouldAlert    bool
		}{
			{"50% threshold at 40% usage", 1000.0, 400.0, 50.0, false},
			{"50% threshold at 50% usage", 1000.0, 500.0, 50.0, true},
			{"80% threshold at 79% usage", 1000.0, 790.0, 80.0, false},
			{"80% threshold at 80% usage", 1000.0, 800.0, 80.0, true},
			{"90% threshold at 95% usage", 1000.0, 950.0, 90.0, true},
			{"100% threshold at 99% usage", 1000.0, 990.0, 100.0, false},
			{"100% threshold at 100% usage", 1000.0, 1000.0, 100.0, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				team := &Team{
					BaseModel:     BaseModel{ID: uuid.New()},
					Name:          "Test Team",
					MaxBudget:     tc.maxBudget,
					CurrentSpend:  tc.currentSpend,
					BudgetAlertAt: tc.budgetAlertAt,
				}

				assert.Equal(t, tc.shouldAlert, team.ShouldAlertBudget())
			})
		}
	})

	t.Run("Team_Multiple_Budget_Periods", func(t *testing.T) {
		team := &Team{
			BaseModel:      BaseModel{ID: uuid.New()},
			Name:           "Test Team",
			MaxBudget:      1000.0,
			CurrentSpend:   900.0,
			BudgetDuration: BudgetPeriodDaily,
			BudgetResetAt:  time.Now().Add(-1 * time.Hour),
		}

		// Reset multiple times with different periods
		periods := []BudgetPeriod{
			BudgetPeriodDaily,
			BudgetPeriodWeekly,
			BudgetPeriodMonthly,
		}

		for _, period := range periods {
			team.BudgetDuration = period
			team.CurrentSpend = 900.0
			team.ResetBudget()

			assert.Equal(t, 0.0, team.CurrentSpend)
			assert.True(t, team.BudgetResetAt.After(time.Now()))
		}
	})
}
