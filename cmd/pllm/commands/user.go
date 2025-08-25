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

// NewUserCommand creates a new user management command
func NewUserCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
		Long:  "Create, list, update, and delete users in pLLM",
	}

	cmd.AddCommand(newUserCreateCommand(ctx))
	cmd.AddCommand(newUserListCommand(ctx))
	cmd.AddCommand(newUserGetCommand(ctx))
	cmd.AddCommand(newUserUpdateCommand(ctx))
	cmd.AddCommand(newUserDeleteCommand(ctx))

	return cmd
}

func newUserCreateCommand(ctx context.Context) *cobra.Command {
	var email, role, firstName, lastName, username string
	var maxBudget float64
	var budgetDuration string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Long:  "Create a new user with specified email and role",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				return fmt.Errorf("email is required")
			}

			user := &models.User{
				Email:     email,
				Username:  username,
				FirstName: firstName,
				LastName:  lastName,
				Role:      models.UserRole(role),
				IsActive:  true,
			}

			if maxBudget > 0 {
				user.MaxBudget = maxBudget
				if budgetDuration != "" {
					user.BudgetDuration = models.BudgetPeriod(budgetDuration)
				}
			}

			if username == "" {
				// Generate username from email if not provided
				user.Username = email
			}

			if IsDirectDBAccess() {
				return createUserDB(ctx, user)
			} else if IsAPIAccess() {
				return createUserAPI(ctx, user)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVarP(&email, "email", "e", "", "User email (required)")
	cmd.Flags().StringVarP(&role, "role", "r", "user", "User role (admin, manager, user, viewer)")
	cmd.Flags().StringVar(&firstName, "first-name", "", "First name")
	cmd.Flags().StringVar(&lastName, "last-name", "", "Last name")
	cmd.Flags().StringVar(&username, "username", "", "Username (defaults to email)")
	cmd.Flags().Float64Var(&maxBudget, "max-budget", 0, "Maximum budget")
	cmd.Flags().StringVar(&budgetDuration, "budget-duration", "monthly", "Budget duration (daily, weekly, monthly, yearly)")

	cmd.MarkFlagRequired("email")

	return cmd
}

func newUserListCommand(ctx context.Context) *cobra.Command {
	var teamID string
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Long:  "List all users or users in a specific team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if IsDirectDBAccess() {
				return listUsersDB(ctx, teamID, limit, offset)
			} else if IsAPIAccess() {
				return listUsersAPI(ctx, teamID, limit, offset)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&teamID, "team-id", "", "Filter by team ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Limit number of results")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")

	return cmd
}

func newUserGetCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [USER_ID]",
		Short: "Get user details",
		Long:  "Get detailed information about a specific user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			userID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid user ID: %w", err)
			}

			if IsDirectDBAccess() {
				return getUserDB(ctx, userID)
			} else if IsAPIAccess() {
				return getUserAPI(ctx, userID)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	return cmd
}

func newUserUpdateCommand(ctx context.Context) *cobra.Command {
	var role string
	var maxBudget float64
	var isActive bool
	var budgetDuration string

	cmd := &cobra.Command{
		Use:   "update [USER_ID]",
		Short: "Update user",
		Long:  "Update user properties",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			userID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid user ID: %w", err)
			}

			updates := make(map[string]interface{})

			if cmd.Flags().Changed("role") {
				updates["role"] = role
			}
			if cmd.Flags().Changed("max-budget") {
				updates["max_budget"] = maxBudget
			}
			if cmd.Flags().Changed("is-active") {
				updates["is_active"] = isActive
			}
			if cmd.Flags().Changed("budget-duration") {
				updates["budget_duration"] = budgetDuration
			}

			if len(updates) == 0 {
				return fmt.Errorf("no updates specified")
			}

			if IsDirectDBAccess() {
				return updateUserDB(ctx, userID, updates)
			} else if IsAPIAccess() {
				return updateUserAPI(ctx, userID, updates)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&role, "role", "", "Update user role")
	cmd.Flags().Float64Var(&maxBudget, "max-budget", 0, "Update max budget")
	cmd.Flags().BoolVar(&isActive, "is-active", true, "Set user active status")
	cmd.Flags().StringVar(&budgetDuration, "budget-duration", "", "Update budget duration")

	return cmd
}

func newUserDeleteCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [USER_ID]",
		Short: "Delete user",
		Long:  "Delete a user (soft delete)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			userID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid user ID: %w", err)
			}

			if IsDirectDBAccess() {
				return deleteUserDB(ctx, userID)
			} else if IsAPIAccess() {
				return deleteUserAPI(ctx, userID)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	return cmd
}

// Database implementations
func createUserDB(ctx context.Context, user *models.User) error {
	if err := db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	if outputJSON {
		OutputJSON(user)
	} else {
		fmt.Printf("User created successfully:\n")
		fmt.Printf("ID: %s\n", user.ID)
		fmt.Printf("Email: %s\n", user.Email)
		fmt.Printf("Role: %s\n", user.Role)
		fmt.Printf("Created: %s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func listUsersDB(ctx context.Context, teamID string, limit, offset int) error {
	var users []models.User
	query := db.Preload("Teams").Limit(limit).Offset(offset)

	if teamID != "" {
		teamUUID, err := uuid.Parse(teamID)
		if err != nil {
			return fmt.Errorf("invalid team ID: %w", err)
		}
		query = query.Joins("JOIN team_members ON team_members.user_id = users.id").
			Where("team_members.team_id = ?", teamUUID)
	}

	if err := query.Find(&users).Error; err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if outputJSON {
		OutputJSON(users)
	} else {
		headers := []string{"ID", "Email", "Username", "Role", "Active", "Created"}
		var rows [][]string
		for _, user := range users {
			rows = append(rows, []string{
				user.ID.String(),
				user.Email,
				user.Username,
				string(user.Role),
				strconv.FormatBool(user.IsActive),
				user.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		OutputTable(headers, rows)
	}

	return nil
}

func getUserDB(ctx context.Context, userID uuid.UUID) error {
	var user models.User
	if err := db.Preload("Teams").First(&user, "id = ?", userID).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if outputJSON {
		OutputJSON(user)
	} else {
		fmt.Printf("User Details:\n")
		fmt.Printf("ID: %s\n", user.ID)
		fmt.Printf("Email: %s\n", user.Email)
		fmt.Printf("Username: %s\n", user.Username)
		fmt.Printf("Name: %s %s\n", user.FirstName, user.LastName)
		fmt.Printf("Role: %s\n", user.Role)
		fmt.Printf("Active: %v\n", user.IsActive)
		fmt.Printf("Budget: $%.2f\n", user.MaxBudget)
		fmt.Printf("Current Spend: $%.2f\n", user.CurrentSpend)
		fmt.Printf("Created: %s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Teams: %d\n", len(user.Teams))
	}

	return nil
}

func updateUserDB(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) error {
	result := db.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	fmt.Printf("User %s updated successfully\n", userID)
	return nil
}

func deleteUserDB(ctx context.Context, userID uuid.UUID) error {
	result := db.Delete(&models.User{}, "id = ?", userID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	fmt.Printf("User %s deleted successfully\n", userID)
	return nil
}

// API implementations
func createUserAPI(ctx context.Context, user *models.User) error {
	resp, err := APIRequest("POST", "/api/users", user)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var createdUser models.User
	if err := json.NewDecoder(resp.Body).Decode(&createdUser); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(createdUser)
	} else {
		fmt.Printf("User created successfully:\n")
		fmt.Printf("ID: %s\n", createdUser.ID)
		fmt.Printf("Email: %s\n", createdUser.Email)
		fmt.Printf("Role: %s\n", createdUser.Role)
	}

	return nil
}

func listUsersAPI(ctx context.Context, teamID string, limit, offset int) error {
	endpoint := fmt.Sprintf("/api/users?limit=%d&offset=%d", limit, offset)
	if teamID != "" {
		endpoint += "&team_id=" + teamID
	}

	resp, err := APIRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var users []models.User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(users)
	} else {
		headers := []string{"ID", "Email", "Role", "Active"}
		var rows [][]string
		for _, user := range users {
			rows = append(rows, []string{
				user.ID.String(),
				user.Email,
				string(user.Role),
				strconv.FormatBool(user.IsActive),
			})
		}
		OutputTable(headers, rows)
	}

	return nil
}

func getUserAPI(ctx context.Context, userID uuid.UUID) error {
	resp, err := APIRequest("GET", "/api/users/"+userID.String(), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(user)
	} else {
		fmt.Printf("User Details:\n")
		fmt.Printf("ID: %s\n", user.ID)
		fmt.Printf("Email: %s\n", user.Email)
		fmt.Printf("Role: %s\n", user.Role)
		fmt.Printf("Active: %v\n", user.IsActive)
	}

	return nil
}

func updateUserAPI(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) error {
	resp, err := APIRequest("PATCH", "/api/users/"+userID.String(), updates)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("User %s updated successfully\n", userID)
	return nil
}

func deleteUserAPI(ctx context.Context, userID uuid.UUID) error {
	resp, err := APIRequest("DELETE", "/api/users/"+userID.String(), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("User %s deleted successfully\n", userID)
	return nil
}
