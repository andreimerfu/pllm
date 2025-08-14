package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/amerfu/pllm/internal/models"
)

// NewTeamCommand creates a new team management command
func NewTeamCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage teams",
		Long:  "Create, list, update, and manage teams in pLLM",
	}

	cmd.AddCommand(newTeamCreateCommand(ctx))
	cmd.AddCommand(newTeamListCommand(ctx))
	cmd.AddCommand(newTeamGetCommand(ctx))
	cmd.AddCommand(newTeamUpdateCommand(ctx))
	cmd.AddCommand(newTeamDeleteCommand(ctx))
	cmd.AddCommand(newTeamAddUserCommand(ctx))
	cmd.AddCommand(newTeamRemoveUserCommand(ctx))
	cmd.AddCommand(newTeamSetBudgetCommand(ctx))

	return cmd
}

func newTeamCreateCommand(ctx context.Context) *cobra.Command {
	var name, description, budgetDuration string
	var maxBudget float64
	var tpm, rpm, maxParallel int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new team",
		Long:  "Create a new team with specified name and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("team name is required")
			}

			team := &models.Team{
				Name:             name,
				Description:      description,
				MaxBudget:        maxBudget,
				BudgetDuration:   models.BudgetPeriod(budgetDuration),
				TPM:              tpm,
				RPM:              rpm,
				MaxParallelCalls: maxParallel,
				IsActive:         true,
			}

			if IsDirectDBAccess() {
				return createTeamDB(ctx, team)
			} else if IsAPIAccess() {
				return createTeamAPI(ctx, team)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Team name (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Team description")
	cmd.Flags().Float64Var(&maxBudget, "max-budget", 0, "Maximum budget")
	cmd.Flags().StringVar(&budgetDuration, "budget-duration", "monthly", "Budget duration (daily, weekly, monthly, yearly)")
	cmd.Flags().IntVar(&tpm, "tpm", 0, "Tokens per minute limit")
	cmd.Flags().IntVar(&rpm, "rpm", 0, "Requests per minute limit")
	cmd.Flags().IntVar(&maxParallel, "max-parallel", 0, "Maximum parallel calls")

	cmd.MarkFlagRequired("name")

	return cmd
}

func newTeamListCommand(ctx context.Context) *cobra.Command {
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List teams",
		Long:  "List all teams",
		RunE: func(cmd *cobra.Command, args []string) error {
			if IsDirectDBAccess() {
				return listTeamsDB(ctx, limit, offset)
			} else if IsAPIAccess() {
				return listTeamsAPI(ctx, limit, offset)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Limit number of results")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")

	return cmd
}

func newTeamGetCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [TEAM_ID]",
		Short: "Get team details",
		Long:  "Get detailed information about a specific team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid team ID: %w", err)
			}

			if IsDirectDBAccess() {
				return getTeamDB(ctx, teamID)
			} else if IsAPIAccess() {
				return getTeamAPI(ctx, teamID)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	return cmd
}

func newTeamUpdateCommand(ctx context.Context) *cobra.Command {
	var name, description string
	var maxBudget float64
	var isActive bool

	cmd := &cobra.Command{
		Use:   "update [TEAM_ID]",
		Short: "Update team",
		Long:  "Update team properties",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid team ID: %w", err)
			}

			updates := make(map[string]interface{})
			
			if cmd.Flags().Changed("name") {
				updates["name"] = name
			}
			if cmd.Flags().Changed("description") {
				updates["description"] = description
			}
			if cmd.Flags().Changed("max-budget") {
				updates["max_budget"] = maxBudget
			}
			if cmd.Flags().Changed("is-active") {
				updates["is_active"] = isActive
			}

			if len(updates) == 0 {
				return fmt.Errorf("no updates specified")
			}

			if IsDirectDBAccess() {
				return updateTeamDB(ctx, teamID, updates)
			} else if IsAPIAccess() {
				return updateTeamAPI(ctx, teamID, updates)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Update team name")
	cmd.Flags().StringVar(&description, "description", "", "Update description")
	cmd.Flags().Float64Var(&maxBudget, "max-budget", 0, "Update max budget")
	cmd.Flags().BoolVar(&isActive, "is-active", true, "Set team active status")

	return cmd
}

func newTeamDeleteCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [TEAM_ID]",
		Short: "Delete team",
		Long:  "Delete a team (soft delete)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			teamID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid team ID: %w", err)
			}

			if IsDirectDBAccess() {
				return deleteTeamDB(ctx, teamID)
			} else if IsAPIAccess() {
				return deleteTeamAPI(ctx, teamID)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	return cmd
}

func newTeamAddUserCommand(ctx context.Context) *cobra.Command {
	var userID, role string
	var maxBudget float64

	cmd := &cobra.Command{
		Use:   "add-user",
		Short: "Add user to team",
		Long:  "Add a user to a team with specified role",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("team ID is required")
			}

			teamID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid team ID: %w", err)
			}

			userUUID, err := uuid.Parse(userID)
			if err != nil {
				return fmt.Errorf("invalid user ID: %w", err)
			}

			member := &models.TeamMember{
				TeamID: teamID,
				UserID: userUUID,
				Role:   models.TeamRole(role),
			}

			if maxBudget > 0 {
				member.MaxBudget = &maxBudget
			}

			if IsDirectDBAccess() {
				return addTeamMemberDB(ctx, member)
			} else if IsAPIAccess() {
				return addTeamMemberAPI(ctx, member)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "User ID to add (required)")
	cmd.Flags().StringVarP(&role, "role", "r", "member", "Team role (owner, admin, member, viewer)")
	cmd.Flags().Float64Var(&maxBudget, "max-budget", 0, "Maximum budget for user")

	cmd.MarkFlagRequired("user-id")

	return cmd
}

func newTeamRemoveUserCommand(ctx context.Context) *cobra.Command {
	var userID string

	cmd := &cobra.Command{
		Use:   "remove-user",
		Short: "Remove user from team",
		Long:  "Remove a user from a team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("team ID is required")
			}

			teamID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid team ID: %w", err)
			}

			userUUID, err := uuid.Parse(userID)
			if err != nil {
				return fmt.Errorf("invalid user ID: %w", err)
			}

			if IsDirectDBAccess() {
				return removeTeamMemberDB(ctx, teamID, userUUID)
			} else if IsAPIAccess() {
				return removeTeamMemberAPI(ctx, teamID, userUUID)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "User ID to remove (required)")
	cmd.MarkFlagRequired("user-id")

	return cmd
}

func newTeamSetBudgetCommand(ctx context.Context) *cobra.Command {
	var amount float64

	cmd := &cobra.Command{
		Use:   "set-budget",
		Short: "Set team budget",
		Long:  "Set the budget for a team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("team ID is required")
			}

			teamID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid team ID: %w", err)
			}

			updates := map[string]interface{}{
				"max_budget": amount,
			}

			if IsDirectDBAccess() {
				return updateTeamDB(ctx, teamID, updates)
			} else if IsAPIAccess() {
				return updateTeamAPI(ctx, teamID, updates)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().Float64VarP(&amount, "amount", "a", 0, "Budget amount (required)")
	cmd.MarkFlagRequired("amount")

	return cmd
}

// Database implementations
func createTeamDB(ctx context.Context, team *models.Team) error {
	if err := db.Create(team).Error; err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	if outputJSON {
		OutputJSON(team)
	} else {
		fmt.Printf("Team created successfully:\n")
		fmt.Printf("ID: %s\n", team.ID)
		fmt.Printf("Name: %s\n", team.Name)
		fmt.Printf("Description: %s\n", team.Description)
		fmt.Printf("Budget: $%.2f\n", team.MaxBudget)
		fmt.Printf("Created: %s\n", team.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func listTeamsDB(ctx context.Context, limit, offset int) error {
	var teams []models.Team
	query := db.Preload("Members").Limit(limit).Offset(offset)

	if err := query.Find(&teams).Error; err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}

	if outputJSON {
		OutputJSON(teams)
	} else {
		headers := []string{"ID", "Name", "Description", "Budget", "Members", "Active", "Created"}
		var rows [][]string
		for _, team := range teams {
			rows = append(rows, []string{
				team.ID.String(),
				team.Name,
				team.Description,
				fmt.Sprintf("$%.2f", team.MaxBudget),
				strconv.Itoa(len(team.Members)),
				strconv.FormatBool(team.IsActive),
				team.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		OutputTable(headers, rows)
	}

	return nil
}

func getTeamDB(ctx context.Context, teamID uuid.UUID) error {
	var team models.Team
	if err := db.Preload("Members.User").First(&team, "id = ?", teamID).Error; err != nil {
		return fmt.Errorf("team not found: %w", err)
	}

	if outputJSON {
		OutputJSON(team)
	} else {
		fmt.Printf("Team Details:\n")
		fmt.Printf("ID: %s\n", team.ID)
		fmt.Printf("Name: %s\n", team.Name)
		fmt.Printf("Description: %s\n", team.Description)
		fmt.Printf("Active: %v\n", team.IsActive)
		fmt.Printf("Budget: $%.2f\n", team.MaxBudget)
		fmt.Printf("Current Spend: $%.2f\n", team.CurrentSpend)
		fmt.Printf("TPM: %d\n", team.TPM)
		fmt.Printf("RPM: %d\n", team.RPM)
		fmt.Printf("Max Parallel: %d\n", team.MaxParallelCalls)
		fmt.Printf("Created: %s\n", team.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Members: %d\n", len(team.Members))
		
		if len(team.Members) > 0 {
			fmt.Printf("\nTeam Members:\n")
			for _, member := range team.Members {
				if member.User != nil {
					fmt.Printf("- %s (%s) - Role: %s\n", member.User.Email, member.User.ID, member.Role)
				}
			}
		}
	}

	return nil
}

func updateTeamDB(ctx context.Context, teamID uuid.UUID, updates map[string]interface{}) error {
	result := db.Model(&models.Team{}).Where("id = ?", teamID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update team: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("team not found")
	}

	fmt.Printf("Team %s updated successfully\n", teamID)
	return nil
}

func deleteTeamDB(ctx context.Context, teamID uuid.UUID) error {
	result := db.Delete(&models.Team{}, "id = ?", teamID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete team: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("team not found")
	}

	fmt.Printf("Team %s deleted successfully\n", teamID)
	return nil
}

func addTeamMemberDB(ctx context.Context, member *models.TeamMember) error {
	if err := db.Create(member).Error; err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}

	fmt.Printf("User %s added to team %s with role %s\n", member.UserID, member.TeamID, member.Role)
	return nil
}

func removeTeamMemberDB(ctx context.Context, teamID, userID uuid.UUID) error {
	result := db.Delete(&models.TeamMember{}, "team_id = ? AND user_id = ?", teamID, userID)
	if result.Error != nil {
		return fmt.Errorf("failed to remove team member: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found in team")
	}

	fmt.Printf("User %s removed from team %s\n", userID, teamID)
	return nil
}

// API implementations
func createTeamAPI(ctx context.Context, team *models.Team) error {
	resp, err := APIRequest("POST", "/api/teams", team)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var createdTeam models.Team
	if err := json.NewDecoder(resp.Body).Decode(&createdTeam); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(createdTeam)
	} else {
		fmt.Printf("Team created successfully:\n")
		fmt.Printf("ID: %s\n", createdTeam.ID)
		fmt.Printf("Name: %s\n", createdTeam.Name)
	}

	return nil
}

func listTeamsAPI(ctx context.Context, limit, offset int) error {
	endpoint := fmt.Sprintf("/api/teams?limit=%d&offset=%d", limit, offset)

	resp, err := APIRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var teams []models.Team
	if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(teams)
	} else {
		headers := []string{"ID", "Name", "Budget", "Active"}
		var rows [][]string
		for _, team := range teams {
			rows = append(rows, []string{
				team.ID.String(),
				team.Name,
				fmt.Sprintf("$%.2f", team.MaxBudget),
				strconv.FormatBool(team.IsActive),
			})
		}
		OutputTable(headers, rows)
	}

	return nil
}

func getTeamAPI(ctx context.Context, teamID uuid.UUID) error {
	resp, err := APIRequest("GET", "/api/teams/"+teamID.String(), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var team models.Team
	if err := json.NewDecoder(resp.Body).Decode(&team); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(team)
	} else {
		fmt.Printf("Team Details:\n")
		fmt.Printf("ID: %s\n", team.ID)
		fmt.Printf("Name: %s\n", team.Name)
		fmt.Printf("Budget: $%.2f\n", team.MaxBudget)
		fmt.Printf("Active: %v\n", team.IsActive)
	}

	return nil
}

func updateTeamAPI(ctx context.Context, teamID uuid.UUID, updates map[string]interface{}) error {
	resp, err := APIRequest("PATCH", "/api/teams/"+teamID.String(), updates)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("Team %s updated successfully\n", teamID)
	return nil
}

func deleteTeamAPI(ctx context.Context, teamID uuid.UUID) error {
	resp, err := APIRequest("DELETE", "/api/teams/"+teamID.String(), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("Team %s deleted successfully\n", teamID)
	return nil
}

func addTeamMemberAPI(ctx context.Context, member *models.TeamMember) error {
	endpoint := fmt.Sprintf("/api/teams/%s/members", member.TeamID)
	resp, err := APIRequest("POST", endpoint, member)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("User %s added to team %s\n", member.UserID, member.TeamID)
	return nil
}

func removeTeamMemberAPI(ctx context.Context, teamID, userID uuid.UUID) error {
	endpoint := fmt.Sprintf("/api/teams/%s/members/%s", teamID, userID)
	resp, err := APIRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("User %s removed from team %s\n", userID, teamID)
	return nil
}