package guardrails

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/config"
	"github.com/amerfu/pllm/internal/services/guardrails/providers"
)

// Factory creates and configures guardrails services
type Factory struct {
	config *config.Config
	logger *zap.Logger
}

// NewFactory creates a new guardrails factory
func NewFactory(config *config.Config, logger *zap.Logger) *Factory {
	return &Factory{
		config: config,
		logger: logger.Named("guardrails_factory"),
	}
}

// CreateExecutor creates a guardrails executor with configured providers
func (f *Factory) CreateExecutor() (*Executor, error) {
	if !f.config.Guardrails.Enabled {
		f.logger.Info("Guardrails disabled in configuration")
		return NewExecutor(&f.config.Guardrails, f.logger), nil
	}

	executor := NewExecutor(&f.config.Guardrails, f.logger)

	// Register configured guardrails
	for _, railConfig := range f.config.Guardrails.Guardrails {
		if !railConfig.Enabled {
			f.logger.Info("Skipping disabled guardrail", zap.String("name", railConfig.Name))
			continue
		}

		guardrail, err := f.createGuardrail(railConfig)
		if err != nil {
			f.logger.Error("Failed to create guardrail",
				zap.String("name", railConfig.Name),
				zap.String("provider", railConfig.Provider),
				zap.Error(err))
			continue
		}

		if err := executor.RegisterGuardrail(guardrail); err != nil {
			f.logger.Error("Failed to register guardrail",
				zap.String("name", railConfig.Name),
				zap.Error(err))
			continue
		}

		f.logger.Info("Successfully registered guardrail",
			zap.String("name", railConfig.Name),
			zap.String("provider", railConfig.Provider),
			zap.Strings("modes", railConfig.Mode))
	}

	return executor, nil
}

// createGuardrail creates a specific guardrail based on its configuration
func (f *Factory) createGuardrail(railConfig config.GuardrailConfig) (Guardrail, error) {
	switch railConfig.Provider {
	case "presidio":
		return f.createPresidioGuardrail(railConfig)
	case "lakera":
		return f.createLakeraGuardrail(railConfig)
	case "openai":
		return f.createOpenAIGuardrail(railConfig)
	case "aporia":
		return f.createAporiaGuardrail(railConfig)
	default:
		return nil, fmt.Errorf("unsupported guardrail provider: %s", railConfig.Provider)
	}
}

// createPresidioGuardrail creates a Presidio PII detection guardrail
func (f *Factory) createPresidioGuardrail(railConfig config.GuardrailConfig) (Guardrail, error) {
	if len(railConfig.Mode) == 0 {
		return nil, fmt.Errorf("no execution modes specified for guardrail %s", railConfig.Name)
	}

	// Use the first mode for now (could be enhanced to support multiple modes per guardrail)
	mode := ParseGuardrailMode(railConfig.Mode[0])

	// Get Presidio provider config
	providerConfig := f.config.Guardrails.Providers.Presidio

	// Override with guardrail-specific config if provided
	if railConfig.APIBase != "" {
		providerConfig.AnalyzerURL = railConfig.APIBase
	}
	if railConfig.Timeout > 0 {
		providerConfig.Timeout = railConfig.Timeout
	}

	// Apply custom config from railConfig.Config
	if customConfig, ok := railConfig.Config["analyzer_url"].(string); ok && customConfig != "" {
		providerConfig.AnalyzerURL = customConfig
	}
	if customConfig, ok := railConfig.Config["anonymizer_url"].(string); ok && customConfig != "" {
		providerConfig.AnonymizerURL = customConfig
	}
	if customConfig, ok := railConfig.Config["language"].(string); ok && customConfig != "" {
		providerConfig.Language = customConfig
	}

	return providers.NewPresidioGuardrail(
		railConfig.Name,
		&providerConfig,
		mode,
		railConfig.Enabled,
		f.logger,
	), nil
}

// createLakeraGuardrail creates a Lakera security guardrail
func (f *Factory) createLakeraGuardrail(railConfig config.GuardrailConfig) (Guardrail, error) {
	// TODO: Implement Lakera guardrail
	// This would follow the same pattern as Presidio but with Lakera-specific logic
	return nil, fmt.Errorf("lakera guardrail not yet implemented")
}

// createOpenAIGuardrail creates an OpenAI content moderation guardrail
func (f *Factory) createOpenAIGuardrail(railConfig config.GuardrailConfig) (Guardrail, error) {
	// TODO: Implement OpenAI moderation guardrail
	// This would use OpenAI's moderation API
	return nil, fmt.Errorf("openai guardrail not yet implemented")
}

// createAporiaGuardrail creates an Aporia security guardrail
func (f *Factory) createAporiaGuardrail(railConfig config.GuardrailConfig) (Guardrail, error) {
	// TODO: Implement Aporia guardrail
	// This would integrate with Aporia's security platform
	return nil, fmt.Errorf("aporia guardrail not yet implemented")
}

// ValidateConfiguration validates the guardrails configuration
func (f *Factory) ValidateConfiguration() error {
	if !f.config.Guardrails.Enabled {
		return nil // No validation needed if disabled
	}

	for _, railConfig := range f.config.Guardrails.Guardrails {
		if err := f.validateGuardrailConfig(railConfig); err != nil {
			return fmt.Errorf("invalid guardrail config %s: %w", railConfig.Name, err)
		}
	}

	return nil
}

// validateGuardrailConfig validates a single guardrail configuration
func (f *Factory) validateGuardrailConfig(railConfig config.GuardrailConfig) error {
	if railConfig.Name == "" {
		return fmt.Errorf("guardrail name is required")
	}

	if railConfig.Provider == "" {
		return fmt.Errorf("guardrail provider is required")
	}

	if len(railConfig.Mode) == 0 {
		return fmt.Errorf("at least one execution mode is required")
	}

	// Validate execution modes
	for _, mode := range railConfig.Mode {
		parsedMode := ParseGuardrailMode(mode)
		if !f.isValidMode(parsedMode) {
			return fmt.Errorf("invalid execution mode: %s", mode)
		}
	}

	// Provider-specific validation
	switch railConfig.Provider {
	case "presidio":
		return f.validatePresidioConfig(railConfig)
	case "lakera":
		return f.validateLakeraConfig(railConfig)
	case "openai":
		return f.validateOpenAIConfig(railConfig)
	case "aporia":
		return f.validateAporiaConfig(railConfig)
	default:
		return fmt.Errorf("unsupported provider: %s", railConfig.Provider)
	}
}

// isValidMode checks if a guardrail mode is valid
func (f *Factory) isValidMode(mode GuardrailMode) bool {
	switch mode {
	case PreCall, PostCall, DuringCall, LoggingOnly:
		return true
	default:
		return false
	}
}

// validatePresidioConfig validates Presidio-specific configuration
func (f *Factory) validatePresidioConfig(railConfig config.GuardrailConfig) error {
	providerConfig := f.config.Guardrails.Providers.Presidio
	
	// Check if analyzer URL is configured
	analyzerURL := providerConfig.AnalyzerURL
	if customURL, ok := railConfig.Config["analyzer_url"].(string); ok && customURL != "" {
		analyzerURL = customURL
	}
	if analyzerURL == "" {
		return fmt.Errorf("presidio analyzer_url is required")
	}

	// Check if anonymizer URL is configured (required for masking modes)
	anonymizerURL := providerConfig.AnonymizerURL
	if customURL, ok := railConfig.Config["anonymizer_url"].(string); ok && customURL != "" {
		anonymizerURL = customURL
	}
	
	// Anonymizer is required for pre_call and logging_only modes that do masking
	needsAnonymizer := false
	for _, mode := range railConfig.Mode {
		if mode == "pre_call" || mode == "logging_only" {
			needsAnonymizer = true
			break
		}
	}
	
	if needsAnonymizer && anonymizerURL == "" {
		return fmt.Errorf("presidio anonymizer_url is required for masking modes (pre_call, logging_only)")
	}

	return nil
}

// validateLakeraConfig validates Lakera-specific configuration
func (f *Factory) validateLakeraConfig(railConfig config.GuardrailConfig) error {
	// TODO: Add Lakera-specific validation
	return nil
}

// validateOpenAIConfig validates OpenAI-specific configuration
func (f *Factory) validateOpenAIConfig(railConfig config.GuardrailConfig) error {
	// TODO: Add OpenAI-specific validation
	return nil
}

// validateAporiaConfig validates Aporia-specific configuration
func (f *Factory) validateAporiaConfig(railConfig config.GuardrailConfig) error {
	// TODO: Add Aporia-specific validation
	return nil
}