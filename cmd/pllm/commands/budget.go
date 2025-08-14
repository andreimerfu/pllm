package commands

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/amerfu/pllm/internal/models"
)

// NewBudgetCommand creates a new budget management command
func NewBudgetCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Manage budgets",
		Long:  "View, set, and manage budgets for users, teams, and keys",
	}

	cmd.AddCommand(newBudgetStatusCommand(ctx))
	cmd.AddCommand(newBudgetSetCommand(ctx))
	cmd.AddCommand(newBudgetResetCommand(ctx))
	cmd.AddCommand(newBudgetUsageCommand(ctx))
	cmd.AddCommand(newBudgetReportCommand(ctx))

	return cmd
}

func newBudgetStatusCommand(ctx context.Context) *cobra.Command {
	var userID, teamID, keyID string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show budget status",
		Long:  "Show budget status for a user, team, or key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if userID == "" && teamID == "" && keyID == "" {
				// Show global budget status
				return showGlobalBudgetStatusDB(ctx)
			}

			if IsDirectDBAccess() {
				return showBudgetStatusDB(ctx, userID, teamID, keyID)
			} else if IsAPIAccess() {
				return showBudgetStatusAPI(ctx, userID, teamID, keyID)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "Show budget status for user")
	cmd.Flags().StringVar(&teamID, "team-id", "", "Show budget status for team")
	cmd.Flags().StringVar(&keyID, "key-id", "", "Show budget status for key")

	return cmd
}

func newBudgetSetCommand(ctx context.Context) *cobra.Command {
	var entityType, entityID string
	var amount float64
	var duration string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set budget",
		Long:  "Set budget for a user, team, or key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if entityType == "" || entityID == "" || amount <= 0 {
				return fmt.Errorf("entity-type, entity-id, and amount are required")
			}

			if entityType != "user" && entityType != "team" && entityType != "key" {
				return fmt.Errorf("entity-type must be 'user', 'team', or 'key'")
			}

			entityUUID, err := uuid.Parse(entityID)
			if err != nil {
				return fmt.Errorf("invalid entity ID: %w", err)
			}

			if IsDirectDBAccess() {
				return setBudgetDB(ctx, entityType, entityUUID, amount, duration)
			} else if IsAPIAccess() {
				return setBudgetAPI(ctx, entityType, entityUUID, amount, duration)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&entityType, "entity-type", "", "Entity type (user, team, key)")
	cmd.Flags().StringVar(&entityID, "entity-id", "", "Entity ID")
	cmd.Flags().Float64VarP(&amount, "amount", "a", 0, "Budget amount")
	cmd.Flags().StringVar(&duration, "duration", "monthly", "Budget duration (daily, weekly, monthly, yearly)")

	cmd.MarkFlagRequired("entity-type")
	cmd.MarkFlagRequired("entity-id")
	cmd.MarkFlagRequired("amount")

	return cmd
}

func newBudgetResetCommand(ctx context.Context) *cobra.Command {
	var entityType, entityID string

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset budget",
		Long:  "Reset budget spend for a user, team, or key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if entityType == "" || entityID == "" {
				return fmt.Errorf("entity-type and entity-id are required")
			}

			if entityType != "user" && entityType != "team" && entityType != "key" {
				return fmt.Errorf("entity-type must be 'user', 'team', or 'key'")
			}

			entityUUID, err := uuid.Parse(entityID)
			if err != nil {
				return fmt.Errorf("invalid entity ID: %w", err)
			}

			if IsDirectDBAccess() {
				return resetBudgetDB(ctx, entityType, entityUUID)
			} else if IsAPIAccess() {
				return resetBudgetAPI(ctx, entityType, entityUUID)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&entityType, "entity-type", "", "Entity type (user, team, key)")
	cmd.Flags().StringVar(&entityID, "entity-id", "", "Entity ID")

	cmd.MarkFlagRequired("entity-type")
	cmd.MarkFlagRequired("entity-id")

	return cmd
}

func newBudgetUsageCommand(ctx context.Context) *cobra.Command {
	var userID, teamID, keyID string
	var days int

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show budget usage",
		Long:  "Show detailed budget usage history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if IsDirectDBAccess() {
				return showBudgetUsageDB(ctx, userID, teamID, keyID, days)
			} else if IsAPIAccess() {
				return showBudgetUsageAPI(ctx, userID, teamID, keyID, days)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "Show usage for user")
	cmd.Flags().StringVar(&teamID, "team-id", "", "Show usage for team")
	cmd.Flags().StringVar(&keyID, "key-id", "", "Show usage for key")
	cmd.Flags().IntVar(&days, "days", 30, "Number of days to show")

	return cmd
}

func newBudgetReportCommand(ctx context.Context) *cobra.Command {
	var period string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate budget report",
		Long:  "Generate a comprehensive budget report",
		RunE: func(cmd *cobra.Command, args []string) error {
			if IsDirectDBAccess() {
				return generateBudgetReportDB(ctx, period)
			} else if IsAPIAccess() {
				return generateBudgetReportAPI(ctx, period)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVarP(&period, "period", "p", "monthly", "Report period (daily, weekly, monthly, yearly)")

	return cmd
}

// Database implementations
func showGlobalBudgetStatusDB(ctx context.Context) error {
	var userCount, teamCount, keyCount int64
	var totalUserBudget, totalTeamBudget, totalKeyBudget float64
	var totalUserSpend, totalTeamSpend, totalKeySpend float64

	// Count entities
	db.Model(&models.User{}).Count(&userCount)
	db.Model(&models.Team{}).Count(&teamCount)
	db.Model(&models.Key{}).Count(&keyCount)

	// Sum budgets and spends
	db.Model(&models.User{}).Select("COALESCE(SUM(max_budget), 0)").Scan(&totalUserBudget)
	db.Model(&models.User{}).Select("COALESCE(SUM(current_spend), 0)").Scan(&totalUserSpend)
	db.Model(&models.Team{}).Select("COALESCE(SUM(max_budget), 0)").Scan(&totalTeamBudget)
	db.Model(&models.Team{}).Select("COALESCE(SUM(current_spend), 0)").Scan(&totalTeamSpend)
	
	// For keys, we need to handle nullable max_budget
	var keyBudgetSum sql.NullFloat64
	var keySpendSum sql.NullFloat64
	db.Model(&models.Key{}).Select("SUM(max_budget)").Scan(&keyBudgetSum)
	db.Model(&models.Key{}).Select("SUM(current_spend)").Scan(&keySpendSum)
	
	if keyBudgetSum.Valid {
		totalKeyBudget = keyBudgetSum.Float64
	}
	if keySpendSum.Valid {
		totalKeySpend = keySpendSum.Float64
	}

	status := map[string]interface{}{
		"users": map[string]interface{}{
			"count":  userCount,
			"budget": totalUserBudget,
			"spend":  totalUserSpend,
		},
		"teams": map[string]interface{}{
			"count":  teamCount,
			"budget": totalTeamBudget,
			"spend":  totalTeamSpend,
		},
		"keys": map[string]interface{}{
			"count":  keyCount,
			"budget": totalKeyBudget,
			"spend":  totalKeySpend,
		},
		"totals": map[string]interface{}{
			"budget": totalUserBudget + totalTeamBudget + totalKeyBudget,
			"spend":  totalUserSpend + totalTeamSpend + totalKeySpend,
		},
	}

	if outputJSON {
		OutputJSON(status)
	} else {
		fmt.Printf("Global Budget Status:\n")
		fmt.Printf("==================\n\n")
		
		fmt.Printf("Users:\n")
		fmt.Printf("  Count: %d\n", userCount)
		fmt.Printf("  Total Budget: $%.2f\n", totalUserBudget)
		fmt.Printf("  Total Spend: $%.2f\n", totalUserSpend)
		fmt.Printf("  Utilization: %.1f%%\n\n", 
			calculateUtilization(totalUserSpend, totalUserBudget))
		
		fmt.Printf("Teams:\n")
		fmt.Printf("  Count: %d\n", teamCount)
		fmt.Printf("  Total Budget: $%.2f\n", totalTeamBudget)
		fmt.Printf("  Total Spend: $%.2f\n", totalTeamSpend)
		fmt.Printf("  Utilization: %.1f%%\n\n", 
			calculateUtilization(totalTeamSpend, totalTeamBudget))
		
		fmt.Printf("Keys:\n")
		fmt.Printf("  Count: %d\n", keyCount)
		fmt.Printf("  Total Budget: $%.2f\n", totalKeyBudget)
		fmt.Printf("  Total Spend: $%.2f\n", totalKeySpend)
		fmt.Printf("  Utilization: %.1f%%\n\n", 
			calculateUtilization(totalKeySpend, totalKeyBudget))
		
		totalBudget := totalUserBudget + totalTeamBudget + totalKeyBudget
		totalSpend := totalUserSpend + totalTeamSpend + totalKeySpend
		
		fmt.Printf("Overall:\n")
		fmt.Printf("  Total Budget: $%.2f\n", totalBudget)
		fmt.Printf("  Total Spend: $%.2f\n", totalSpend)
		fmt.Printf("  Total Utilization: %.1f%%\n", 
			calculateUtilization(totalSpend, totalBudget))
	}

	return nil
}


func calculateUtilization(spend, budget float64) float64 {
	if budget == 0 {
		return 0
	}
	return (spend / budget) * 100
}

func showBudgetStatusDB(ctx context.Context, userID, teamID, keyID string) error {
	if userID != "" {
		userUUID, err := uuid.Parse(userID)
		if err != nil {
			return fmt.Errorf("invalid user ID: %w", err)
		}
		return showUserBudgetDB(ctx, userUUID)
	}

	if teamID != "" {
		teamUUID, err := uuid.Parse(teamID)
		if err != nil {
			return fmt.Errorf("invalid team ID: %w", err)
		}
		return showTeamBudgetDB(ctx, teamUUID)
	}

	if keyID != "" {
		keyUUID, err := uuid.Parse(keyID)
		if err != nil {
			return fmt.Errorf("invalid key ID: %w", err)
		}
		return showKeyBudgetDB(ctx, keyUUID)
	}

	return fmt.Errorf("no entity specified")
}

func showUserBudgetDB(ctx context.Context, userID uuid.UUID) error {
	var user models.User
	if err := db.First(&user, "id = ?", userID).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	status := map[string]interface{}{
		"id":               user.ID,
		"email":            user.Email,
		"max_budget":       user.MaxBudget,
		"current_spend":    user.CurrentSpend,
		"remaining":        user.MaxBudget - user.CurrentSpend,
		"utilization":      calculateUtilization(user.CurrentSpend, user.MaxBudget),
		"budget_duration":  user.BudgetDuration,
		"budget_reset_at":  user.BudgetResetAt,
		"is_exceeded":      user.IsBudgetExceeded(),
	}

	if outputJSON {
		OutputJSON(status)
	} else {
		fmt.Printf("User Budget Status:\n")
		fmt.Printf("==================\n")
		fmt.Printf("User: %s (%s)\n", user.Email, user.ID)
		fmt.Printf("Budget: $%.2f\n", user.MaxBudget)
		fmt.Printf("Spent: $%.2f\n", user.CurrentSpend)
		fmt.Printf("Remaining: $%.2f\n", user.MaxBudget-user.CurrentSpend)
		fmt.Printf("Utilization: %.1f%%\n", calculateUtilization(user.CurrentSpend, user.MaxBudget))
		fmt.Printf("Duration: %s\n", user.BudgetDuration)
		fmt.Printf("Resets: %s\n", user.BudgetResetAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Exceeded: %v\n", user.IsBudgetExceeded())
	}

	return nil
}

func showTeamBudgetDB(ctx context.Context, teamID uuid.UUID) error {
	var team models.Team
	if err := db.First(&team, "id = ?", teamID).Error; err != nil {
		return fmt.Errorf("team not found: %w", err)
	}

	status := map[string]interface{}{
		"id":               team.ID,
		"name":             team.Name,
		"max_budget":       team.MaxBudget,
		"current_spend":    team.CurrentSpend,
		"remaining":        team.MaxBudget - team.CurrentSpend,
		"utilization":      calculateUtilization(team.CurrentSpend, team.MaxBudget),
		"budget_duration":  team.BudgetDuration,
		"budget_reset_at":  team.BudgetResetAt,
		"is_exceeded":      team.IsBudgetExceeded(),
	}

	if outputJSON {
		OutputJSON(status)
	} else {
		fmt.Printf("Team Budget Status:\n")
		fmt.Printf("==================\n")
		fmt.Printf("Team: %s (%s)\n", team.Name, team.ID)
		fmt.Printf("Budget: $%.2f\n", team.MaxBudget)
		fmt.Printf("Spent: $%.2f\n", team.CurrentSpend)
		fmt.Printf("Remaining: $%.2f\n", team.MaxBudget-team.CurrentSpend)
		fmt.Printf("Utilization: %.1f%%\n", calculateUtilization(team.CurrentSpend, team.MaxBudget))
		fmt.Printf("Duration: %s\n", team.BudgetDuration)
		fmt.Printf("Resets: %s\n", team.BudgetResetAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Exceeded: %v\n", team.IsBudgetExceeded())
	}

	return nil
}

func showKeyBudgetDB(ctx context.Context, keyID uuid.UUID) error {
	var key models.Key
	if err := db.First(&key, "id = ?", keyID).Error; err != nil {
		return fmt.Errorf("key not found: %w", err)
	}

	maxBudget := float64(0)
	if key.MaxBudget != nil {
		maxBudget = *key.MaxBudget
	}

	status := map[string]interface{}{
		"id":               key.ID,
		"name":             key.Name,
		"max_budget":       maxBudget,
		"current_spend":    key.CurrentSpend,
		"remaining":        maxBudget - key.CurrentSpend,
		"utilization":      calculateUtilization(key.CurrentSpend, maxBudget),
		"is_exceeded":      key.IsBudgetExceeded(),
	}

	if outputJSON {
		OutputJSON(status)
	} else {
		fmt.Printf("Key Budget Status:\n")
		fmt.Printf("=================\n")
		fmt.Printf("Key: %s (%s)\n", key.Name, key.ID)
		fmt.Printf("Budget: $%.2f\n", maxBudget)
		fmt.Printf("Spent: $%.2f\n", key.CurrentSpend)
		fmt.Printf("Remaining: $%.2f\n", maxBudget-key.CurrentSpend)
		fmt.Printf("Utilization: %.1f%%\n", calculateUtilization(key.CurrentSpend, maxBudget))
		fmt.Printf("Exceeded: %v\n", key.IsBudgetExceeded())
	}

	return nil
}

func setBudgetDB(ctx context.Context, entityType string, entityID uuid.UUID, amount float64, duration string) error {
	updates := map[string]interface{}{
		"max_budget":      amount,
		"budget_duration": duration,
	}

	var tableName string
	switch entityType {
	case "user":
		tableName = "users"
	case "team":
		tableName = "teams"
	case "key":
		tableName = "keys"
	}

	result := db.Table(tableName).Where("id = ?", entityID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update budget: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%s not found", entityType)
	}

	fmt.Printf("Budget set successfully for %s %s: $%.2f\n", entityType, entityID, amount)
	return nil
}

func resetBudgetDB(ctx context.Context, entityType string, entityID uuid.UUID) error {
	updates := map[string]interface{}{
		"current_spend": 0,
	}

	var tableName string
	switch entityType {
	case "user":
		tableName = "users"
	case "team":
		tableName = "teams"
	case "key":
		tableName = "keys"
	}

	result := db.Table(tableName).Where("id = ?", entityID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to reset budget: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%s not found", entityType)
	}

	fmt.Printf("Budget reset successfully for %s %s\n", entityType, entityID)
	return nil
}

func showBudgetUsageDB(ctx context.Context, userID, teamID, keyID string, days int) error {
	var tracking []models.BudgetTracking
	query := db.Where("created_at > NOW() - INTERVAL '%d days'", days)

	if userID != "" {
		userUUID, err := uuid.Parse(userID)
		if err != nil {
			return fmt.Errorf("invalid user ID: %w", err)
		}
		query = query.Where("user_id = ?", userUUID)
	}

	if teamID != "" {
		teamUUID, err := uuid.Parse(teamID)
		if err != nil {
			return fmt.Errorf("invalid team ID: %w", err)
		}
		query = query.Where("team_id = ?", teamUUID)
	}

	if keyID != "" {
		keyUUID, err := uuid.Parse(keyID)
		if err != nil {
			return fmt.Errorf("invalid key ID: %w", err)
		}
		query = query.Where("key_id = ?", keyUUID)
	}

	if err := query.Order("created_at DESC").Limit(100).Find(&tracking).Error; err != nil {
		return fmt.Errorf("failed to get budget usage: %w", err)
	}

	if outputJSON {
		OutputJSON(tracking)
	} else {
		if len(tracking) == 0 {
			fmt.Printf("No budget usage found for the specified criteria\n")
			return nil
		}

		headers := []string{"Date", "Model", "Provider", "Tokens", "Cost", "Entity"}
		var rows [][]string
		totalCost := 0.0
		totalTokens := 0

		for _, t := range tracking {
			entity := "Unknown"
			if t.UserID != nil {
				entity = "User: " + t.UserID.String()
			} else if t.TeamID != nil {
				entity = "Team: " + t.TeamID.String()
			} else if t.KeyID != nil {
				entity = "Key: " + t.KeyID.String()
			}

			rows = append(rows, []string{
				t.CreatedAt.Format("2006-01-02 15:04"),
				t.Model,
				t.Provider,
				strconv.Itoa(t.Tokens),
				fmt.Sprintf("$%.4f", t.Cost),
				entity,
			})

			totalCost += t.Cost
			totalTokens += t.Tokens
		}

		OutputTable(headers, rows)
		
		fmt.Printf("\nSummary (last %d days):\n", days)
		fmt.Printf("Total Entries: %d\n", len(tracking))
		fmt.Printf("Total Tokens: %d\n", totalTokens)
		fmt.Printf("Total Cost: $%.4f\n", totalCost)
	}

	return nil
}

func generateBudgetReportDB(ctx context.Context, period string) error {
	// This would generate a comprehensive budget report
	// For now, we'll show a simple summary
	fmt.Printf("Budget Report (%s):\n", period)
	fmt.Printf("===================\n")
	fmt.Printf("Feature not yet implemented - would show detailed analytics\n")
	return nil
}

// API implementations
func showBudgetStatusAPI(ctx context.Context, userID, teamID, keyID string) error {
	endpoint := "/api/budget/status?"
	if userID != "" {
		endpoint += "user_id=" + userID
	} else if teamID != "" {
		endpoint += "team_id=" + teamID
	} else if keyID != "" {
		endpoint += "key_id=" + keyID
	}

	resp, err := APIRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(status)
	} else {
		fmt.Printf("Budget Status:\n")
		for k, v := range status {
			fmt.Printf("%s: %v\n", k, v)
		}
	}

	return nil
}

func setBudgetAPI(ctx context.Context, entityType string, entityID uuid.UUID, amount float64, duration string) error {
	body := map[string]interface{}{
		"entity_type":     entityType,
		"entity_id":       entityID,
		"amount":          amount,
		"duration":        duration,
	}

	resp, err := APIRequest("POST", "/api/budget/set", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("Budget set successfully for %s %s: $%.2f\n", entityType, entityID, amount)
	return nil
}

func resetBudgetAPI(ctx context.Context, entityType string, entityID uuid.UUID) error {
	body := map[string]interface{}{
		"entity_type": entityType,
		"entity_id":   entityID,
	}

	resp, err := APIRequest("POST", "/api/budget/reset", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("Budget reset successfully for %s %s\n", entityType, entityID)
	return nil
}

func showBudgetUsageAPI(ctx context.Context, userID, teamID, keyID string, days int) error {
	endpoint := fmt.Sprintf("/api/budget/usage?days=%d", days)
	if userID != "" {
		endpoint += "&user_id=" + userID
	}
	if teamID != "" {
		endpoint += "&team_id=" + teamID
	}
	if keyID != "" {
		endpoint += "&key_id=" + keyID
	}

	resp, err := APIRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var usage []models.BudgetTracking
	if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(usage)
	} else {
		headers := []string{"Date", "Model", "Tokens", "Cost"}
		var rows [][]string
		for _, u := range usage {
			rows = append(rows, []string{
				u.CreatedAt.Format("2006-01-02 15:04"),
				u.Model,
				strconv.Itoa(u.Tokens),
				fmt.Sprintf("$%.4f", u.Cost),
			})
		}
		OutputTable(headers, rows)
	}

	return nil
}

func generateBudgetReportAPI(ctx context.Context, period string) error {
	endpoint := fmt.Sprintf("/api/budget/report?period=%s", period)

	resp, err := APIRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var report map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(report)
	} else {
		fmt.Printf("Budget Report (%s):\n", period)
		for k, v := range report {
			fmt.Printf("%s: %v\n", k, v)
		}
	}

	return nil
}