package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/amerfu/pllm/internal/models"
)

// NewKeyCommand creates a new key management command
func NewKeyCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage API keys",
		Long:  "Create, list, revoke, and manage API keys in pLLM",
	}

	cmd.AddCommand(newKeyGenerateCommand(ctx))
	cmd.AddCommand(newKeyListCommand(ctx))
	cmd.AddCommand(newKeyGetCommand(ctx))
	cmd.AddCommand(newKeyRevokeCommand(ctx))
	cmd.AddCommand(newKeyInfoCommand(ctx))

	return cmd
}

func newKeyGenerateCommand(ctx context.Context) *cobra.Command {
	var keyType, name, userID, teamID string
	var maxBudget float64
	var budgetDuration string
	var duration int
	var tpm, rpm, maxParallel int

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new API key",
		Long:  "Generate a new API key for a user or team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("key name is required")
			}

			if userID == "" && teamID == "" {
				return fmt.Errorf("either user-id or team-id is required")
			}

			keyRequest := &models.KeyRequest{
				Name: name,
				Type: models.KeyType(keyType),
			}

			if userID != "" {
				userUUID, err := uuid.Parse(userID)
				if err != nil {
					return fmt.Errorf("invalid user ID: %w", err)
				}
				keyRequest.UserID = &userUUID
			}

			if teamID != "" {
				teamUUID, err := uuid.Parse(teamID)
				if err != nil {
					return fmt.Errorf("invalid team ID: %w", err)
				}
				keyRequest.TeamID = &teamUUID
			}

			if duration > 0 {
				keyRequest.Duration = &duration
			}

			if maxBudget > 0 {
				keyRequest.MaxBudget = &maxBudget
				if budgetDuration != "" {
					bd := models.BudgetPeriod(budgetDuration)
					keyRequest.BudgetDuration = &bd
				}
			}

			if tpm > 0 {
				keyRequest.TPM = &tpm
			}
			if rpm > 0 {
				keyRequest.RPM = &rpm
			}
			if maxParallel > 0 {
				keyRequest.MaxParallelCalls = &maxParallel
			}

			if IsDirectDBAccess() {
				return generateKeyDB(ctx, keyRequest)
			} else if IsAPIAccess() {
				return generateKeyAPI(ctx, keyRequest)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&keyType, "type", "api", "Key type (api, virtual, master)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Key name (required)")
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID (either user-id or team-id required)")
	cmd.Flags().StringVar(&teamID, "team-id", "", "Team ID (either user-id or team-id required)")
	cmd.Flags().Float64Var(&maxBudget, "max-budget", 0, "Maximum budget for key")
	cmd.Flags().StringVar(&budgetDuration, "budget-duration", "monthly", "Budget duration (daily, weekly, monthly, yearly)")
	cmd.Flags().IntVar(&duration, "duration", 0, "Key duration in seconds (0 for no expiration)")
	cmd.Flags().IntVar(&tpm, "tpm", 0, "Tokens per minute limit")
	cmd.Flags().IntVar(&rpm, "rpm", 0, "Requests per minute limit")
	cmd.Flags().IntVar(&maxParallel, "max-parallel", 0, "Maximum parallel calls")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func newKeyListCommand(ctx context.Context) *cobra.Command {
	var userID, teamID string
	var limit, offset int
	var showRevoked bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		Long:  "List all API keys or keys for a specific user/team",
		RunE: func(cmd *cobra.Command, args []string) error {
			if IsDirectDBAccess() {
				return listKeysDB(ctx, userID, teamID, limit, offset, showRevoked)
			} else if IsAPIAccess() {
				return listKeysAPI(ctx, userID, teamID, limit, offset, showRevoked)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "Filter by user ID")
	cmd.Flags().StringVar(&teamID, "team-id", "", "Filter by team ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Limit number of results")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	cmd.Flags().BoolVar(&showRevoked, "show-revoked", false, "Include revoked keys")

	return cmd
}

func newKeyGetCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [KEY_ID]",
		Short: "Get key details",
		Long:  "Get detailed information about a specific API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid key ID: %w", err)
			}

			if IsDirectDBAccess() {
				return getKeyDB(ctx, keyID)
			} else if IsAPIAccess() {
				return getKeyAPI(ctx, keyID)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	return cmd
}

func newKeyRevokeCommand(ctx context.Context) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "revoke [KEY_ID]",
		Short: "Revoke an API key",
		Long:  "Revoke an API key with optional reason",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid key ID: %w", err)
			}

			if IsDirectDBAccess() {
				return revokeKeyDB(ctx, keyID, reason)
			} else if IsAPIAccess() {
				return revokeKeyAPI(ctx, keyID, reason)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "", "Revocation reason")

	return cmd
}

func newKeyInfoCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "info [KEY_ID]",
		Short:   "Get key info",
		Long:    "Get usage and status information about a specific API key",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			keyID, err := uuid.Parse(args[0])
			if err != nil {
				return fmt.Errorf("invalid key ID: %w", err)
			}

			if IsDirectDBAccess() {
				return getKeyInfoDB(ctx, keyID)
			} else if IsAPIAccess() {
				return getKeyInfoAPI(ctx, keyID)
			}

			return fmt.Errorf("no database or API access configured")
		},
	}

	return cmd
}

// Database implementations
func generateKeyDB(ctx context.Context, keyRequest *models.KeyRequest) error {
	// Generate the actual key
	keyValue, keyHash, err := models.GenerateKey(keyRequest.Type)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	key := &models.Key{
		Name:             keyRequest.Name,
		Type:             keyRequest.Type,
		Key:              keyValue,
		KeyHash:          keyHash,
		KeyPrefix:        keyHash[:8],
		UserID:           keyRequest.UserID,
		TeamID:           keyRequest.TeamID,
		IsActive:         true,
		MaxBudget:        keyRequest.MaxBudget,
		TPM:              keyRequest.TPM,
		RPM:              keyRequest.RPM,
		MaxParallelCalls: keyRequest.MaxParallelCalls,
	}

	if keyRequest.Duration != nil && *keyRequest.Duration > 0 {
		expiresAt := time.Now().Add(time.Duration(*keyRequest.Duration) * time.Second)
		key.ExpiresAt = &expiresAt
	}

	if keyRequest.BudgetDuration != nil {
		key.BudgetDuration = keyRequest.BudgetDuration
		key.ResetBudget()
	}

	if err := db.Create(key).Error; err != nil {
		return fmt.Errorf("failed to create key: %w", err)
	}

	if outputJSON {
		response := models.KeyResponse{
			Key:      *key,
			KeyValue: keyValue,
		}
		// Clear the actual key from the response for security
		response.Key.Key = ""
		OutputJSON(response)
	} else {
		fmt.Printf("API Key generated successfully:\n")
		fmt.Printf("ID: %s\n", key.ID)
		fmt.Printf("Name: %s\n", key.Name)
		fmt.Printf("Type: %s\n", key.Type)
		fmt.Printf("Key: %s\n", keyValue)
		fmt.Printf("Prefix: %s\n", key.KeyPrefix)
		if key.ExpiresAt != nil {
			fmt.Printf("Expires: %s\n", key.ExpiresAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("Created: %s\n", key.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("\n⚠️  Save this key securely - it won't be shown again!\n")
	}

	return nil
}

func listKeysDB(ctx context.Context, userID, teamID string, limit, offset int, showRevoked bool) error {
	var keys []models.Key
	query := db.Preload("User").Preload("Team").Limit(limit).Offset(offset)

	if !showRevoked {
		query = query.Where("revoked_at IS NULL")
	}

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

	if err := query.Find(&keys).Error; err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	if outputJSON {
		// Clear actual keys for security
		for i := range keys {
			keys[i].Key = ""
		}
		OutputJSON(keys)
	} else {
		headers := []string{"ID", "Name", "Type", "Prefix", "Owner", "Budget", "Active", "Expires", "Created"}
		var rows [][]string
		for _, key := range keys {
			owner := ""
			if key.User != nil {
				owner = key.User.Email
			} else if key.Team != nil {
				owner = key.Team.Name + " (team)"
			}

			budget := "N/A"
			if key.MaxBudget != nil {
				budget = fmt.Sprintf("$%.2f", *key.MaxBudget)
			}

			expires := "Never"
			if key.ExpiresAt != nil {
				expires = key.ExpiresAt.Format("2006-01-02")
			}

			status := "Active"
			if key.IsRevoked() {
				status = "Revoked"
			} else if key.IsExpired() {
				status = "Expired"
			} else if !key.IsActive {
				status = "Inactive"
			}

			rows = append(rows, []string{
				key.ID.String(),
				key.Name,
				string(key.Type),
				key.KeyPrefix,
				owner,
				budget,
				status,
				expires,
				key.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		OutputTable(headers, rows)
	}

	return nil
}

func getKeyDB(ctx context.Context, keyID uuid.UUID) error {
	var key models.Key
	if err := db.Preload("User").Preload("Team").First(&key, "id = ?", keyID).Error; err != nil {
		return fmt.Errorf("key not found: %w", err)
	}

	if outputJSON {
		// Clear actual key for security
		key.Key = ""
		OutputJSON(key)
	} else {
		fmt.Printf("API Key Details:\n")
		fmt.Printf("ID: %s\n", key.ID)
		fmt.Printf("Name: %s\n", key.Name)
		fmt.Printf("Type: %s\n", key.Type)
		fmt.Printf("Prefix: %s\n", key.KeyPrefix)

		if key.User != nil {
			fmt.Printf("User: %s (%s)\n", key.User.Email, key.User.ID)
		}
		if key.Team != nil {
			fmt.Printf("Team: %s (%s)\n", key.Team.Name, key.Team.ID)
		}

		fmt.Printf("Active: %v\n", key.IsActive)
		fmt.Printf("Revoked: %v\n", key.IsRevoked())
		fmt.Printf("Expired: %v\n", key.IsExpired())

		if key.MaxBudget != nil {
			fmt.Printf("Budget: $%.2f / $%.2f\n", key.CurrentSpend, *key.MaxBudget)
		}

		fmt.Printf("Usage: %d requests, %d tokens, $%.4f\n",
			key.UsageCount, key.TotalTokens, key.TotalCost)

		if key.ExpiresAt != nil {
			fmt.Printf("Expires: %s\n", key.ExpiresAt.Format("2006-01-02 15:04:05"))
		}
		if key.LastUsedAt != nil {
			fmt.Printf("Last Used: %s\n", key.LastUsedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("Created: %s\n", key.CreatedAt.Format("2006-01-02 15:04:05"))

		if key.IsRevoked() {
			fmt.Printf("Revoked: %s\n", key.RevokedAt.Format("2006-01-02 15:04:05"))
			if key.RevocationReason != "" {
				fmt.Printf("Reason: %s\n", key.RevocationReason)
			}
		}
	}

	return nil
}

func revokeKeyDB(ctx context.Context, keyID uuid.UUID, reason string) error {
	var key models.Key
	if err := db.First(&key, "id = ?", keyID).Error; err != nil {
		return fmt.Errorf("key not found: %w", err)
	}

	if key.IsRevoked() {
		return fmt.Errorf("key is already revoked")
	}

	// For now, we'll use a system user ID - in a real implementation,
	// this would come from the authenticated user context
	systemUserID := uuid.New()
	key.Revoke(systemUserID, reason)

	if err := db.Save(&key).Error; err != nil {
		return fmt.Errorf("failed to revoke key: %w", err)
	}

	fmt.Printf("Key %s revoked successfully\n", keyID)
	if reason != "" {
		fmt.Printf("Reason: %s\n", reason)
	}

	return nil
}

func getKeyInfoDB(ctx context.Context, keyID uuid.UUID) error {
	return getKeyDB(ctx, keyID) // Same as get for now
}

// API implementations
func generateKeyAPI(ctx context.Context, keyRequest *models.KeyRequest) error {
	resp, err := APIRequest("POST", "/api/keys", keyRequest)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 201 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var keyResponse models.KeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&keyResponse); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(keyResponse)
	} else {
		fmt.Printf("API Key generated successfully:\n")
		fmt.Printf("ID: %s\n", keyResponse.ID)
		fmt.Printf("Name: %s\n", keyResponse.Name)
		fmt.Printf("Key: %s\n", keyResponse.KeyValue)
		fmt.Printf("\n⚠️  Save this key securely - it won't be shown again!\n")
	}

	return nil
}

func listKeysAPI(ctx context.Context, userID, teamID string, limit, offset int, showRevoked bool) error {
	endpoint := fmt.Sprintf("/api/keys?limit=%d&offset=%d", limit, offset)

	if userID != "" {
		endpoint += "&user_id=" + userID
	}
	if teamID != "" {
		endpoint += "&team_id=" + teamID
	}
	if showRevoked {
		endpoint += "&show_revoked=true"
	}

	resp, err := APIRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var keys []models.Key
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(keys)
	} else {
		headers := []string{"ID", "Name", "Type", "Prefix", "Budget", "Active"}
		var rows [][]string
		for _, key := range keys {
			budget := "N/A"
			if key.MaxBudget != nil {
				budget = fmt.Sprintf("$%.2f", *key.MaxBudget)
			}

			status := "Active"
			if key.IsRevoked() {
				status = "Revoked"
			} else if !key.IsActive {
				status = "Inactive"
			}

			rows = append(rows, []string{
				key.ID.String(),
				key.Name,
				string(key.Type),
				key.KeyPrefix,
				budget,
				status,
			})
		}
		OutputTable(headers, rows)
	}

	return nil
}

func getKeyAPI(ctx context.Context, keyID uuid.UUID) error {
	resp, err := APIRequest("GET", "/api/keys/"+keyID.String(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var key models.Key
	if err := json.NewDecoder(resp.Body).Decode(&key); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputJSON {
		OutputJSON(key)
	} else {
		fmt.Printf("API Key Details:\n")
		fmt.Printf("ID: %s\n", key.ID)
		fmt.Printf("Name: %s\n", key.Name)
		fmt.Printf("Type: %s\n", key.Type)
		fmt.Printf("Active: %v\n", key.IsActive)
	}

	return nil
}

func revokeKeyAPI(ctx context.Context, keyID uuid.UUID, reason string) error {
	body := map[string]interface{}{}
	if reason != "" {
		body["reason"] = reason
	}

	resp, err := APIRequest("POST", "/api/keys/"+keyID.String()+"/revoke", body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("Key %s revoked successfully\n", keyID)
	return nil
}

func getKeyInfoAPI(ctx context.Context, keyID uuid.UUID) error {
	return getKeyAPI(ctx, keyID) // Same as get for now
}
