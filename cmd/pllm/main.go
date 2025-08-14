package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/cmd/pllm/commands"
	"github.com/amerfu/pllm/internal/models"
)

var (
	cfgFile    string
	dbURL      string
	apiURL     string
	apiKey     string
	outputJSON bool
	verbose    bool
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "pllm",
		Short: "pLLM Management CLI",
		Long: `A comprehensive CLI tool for managing pLLM users, teams, keys, and budgets.
Supports both direct database access (when run on server) and API access (when run remotely).`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initConfig()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pllm.yaml)")
	rootCmd.PersistentFlags().StringVar(&dbURL, "db-url", "", "database URL for direct access")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "API base URL for remote access")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for remote access")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")

	// Add subcommands
	ctx := context.Background()
	rootCmd.AddCommand(commands.NewUserCommand(ctx))
	rootCmd.AddCommand(commands.NewTeamCommand(ctx))
	rootCmd.AddCommand(commands.NewKeyCommand(ctx))
	rootCmd.AddCommand(commands.NewBudgetCommand(ctx))
	rootCmd.AddCommand(commands.NewConfigCommand())

	return rootCmd
}

func initConfig() error {
	// Initialize configuration from environment variables, config file, or flags
	if cfgFile != "" {
		// Use config file from flag
		fmt.Printf("Using config file: %s\n", cfgFile)
	}

	// Set up database connection if URL is provided
	if dbURL != "" {
		db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		
		// Auto-migrate models
		if err := db.AutoMigrate(
			&models.User{},
			&models.Team{},
			&models.TeamMember{},
			&models.Key{},
			&models.Budget{},
			&models.BudgetTracking{},
			&models.BudgetAlert{},
		); err != nil {
			return fmt.Errorf("failed to migrate database: %w", err)
		}

		// Store DB connection in context for commands to use
		commands.SetDB(db)
	}

	// Set API configuration if provided
	if apiURL != "" && apiKey != "" {
		commands.SetAPIConfig(apiURL, apiKey)
	}

	// Set output format
	commands.SetOutputJSON(outputJSON)
	commands.SetVerbose(verbose)

	return nil
}