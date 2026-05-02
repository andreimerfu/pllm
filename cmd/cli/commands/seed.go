package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amerfu/pllm/cmd/cli/commands/seed"
)

// NewSeedCommand exposes `pllm seed` with sub-scenarios. Today: only
// `bank`. The structure leaves room for `pllm seed insurance` etc.
func NewSeedCommand(_ context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Populate demo data into a pllm install",
		Long:  "Idempotent fixtures driven through the public admin API.",
	}
	cmd.AddCommand(newSeedBankCommand())
	return cmd
}

func newSeedBankCommand() *cobra.Command {
	var opts seed.Options
	var cleanup bool
	cmd := &cobra.Command{
		Use:   "bank",
		Short: "Seed the bank-MRM demo scenario",
		Long: `Materializes a bank governance scenario:
  - 4 use cases (T1 blocking, T1 flag under review, T2 blocking, T4 sandbox)
  - registry agents, prompts, skills, and 5 servers
  - 3 demo API keys bound to use cases
  - 7 days of scope-event history (when --with-events)
  - one disabled webhook subscription (when --with-webhook)

Idempotent: re-running updates existing fixtures. Use --cleanup to remove.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.BaseURL == "" {
				opts.BaseURL = envOr("PLLM_BASE_URL", "http://localhost:8080")
			}
			if opts.MasterKey == "" {
				opts.MasterKey = os.Getenv("PLLM_MASTER_KEY")
			}
			if opts.MasterKey == "" {
				return fmt.Errorf("master key required: pass --master-key or set $PLLM_MASTER_KEY")
			}
			if opts.LLMRoute == "" {
				opts.LLMRoute = envOr("PLLM_DEMO_ROUTE", "free-route")
			}
			if cleanup {
				return seed.CleanupBank(opts)
			}
			return seed.RunBank(opts)
		},
	}
	cmd.Flags().StringVar(&opts.BaseURL, "base-url", "", "gateway URL (default $PLLM_BASE_URL or http://localhost:8080)")
	cmd.Flags().StringVar(&opts.MasterKey, "master-key", "", "admin key (default $PLLM_MASTER_KEY)")
	cmd.Flags().StringVar(&opts.LLMRoute, "llm-route", "", "route name use cases declare (default free-route)")
	cmd.Flags().BoolVar(&opts.WithDeployments, "with-deployments", false, "also publish 2 live MCP server entries (no auto-deploy)")
	cmd.Flags().BoolVar(&opts.WithEvents, "with-events", true, "seed 7-day scope event history")
	cmd.Flags().BoolVar(&opts.WithWebhook, "with-webhook", true, "seed disabled webhook subscription")
	cmd.Flags().StringVar(&opts.WebhookURL, "webhook-url", "https://webhook.site/REPLACE-ME", "URL for the seeded webhook")
	cmd.Flags().BoolVar(&opts.JSONOutput, "json", false, "machine-readable JSON output")
	cmd.Flags().BoolVar(&cleanup, "cleanup", false, "remove all bank.demo/* and demo-* fixtures and exit")
	return cmd
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
