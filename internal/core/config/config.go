package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Admin    AdminConfig    `mapstructure:"admin"`
	Auth     AuthConfig     `mapstructure:"auth"`

	// Model configuration - internal format after conversion
	ModelList []ModelInstance `mapstructure:"-"` // Will be populated from RawModelList

	// Raw configuration from YAML
	RawModelList []ModelConfig       `mapstructure:"model_list"` // User-friendly format
	ModelGroups  []ModelGroup        `mapstructure:"model_groups"`
	Router       RouterSettings      `mapstructure:"router"`
	ModelAliases map[string][]string `mapstructure:"model_aliases"`

	Cache      CacheConfig      `mapstructure:"cache"`
	RateLimit  RateLimitConfig  `mapstructure:"rate_limit"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	CORS       CORSConfig       `mapstructure:"cors"`
	Realtime   RealtimeConfig   `mapstructure:"realtime"`
	Guardrails GuardrailsConfig `mapstructure:"guardrails"`
}

type ServerConfig struct {
	Port             int           `mapstructure:"port"`
	AdminPort        int           `mapstructure:"admin_port"`
	MetricsPort      int           `mapstructure:"metrics_port"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout"`
	IdleTimeout      time.Duration `mapstructure:"idle_timeout"`
	GracefulShutdown time.Duration `mapstructure:"graceful_shutdown"`
}

type DatabaseConfig struct {
	URL             string        `mapstructure:"url"`
	MaxConnections  int           `mapstructure:"max_connections"`
	MaxIdleConns    int           `mapstructure:"max_idle_connections"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type JWTConfig struct {
	SecretKey            string        `mapstructure:"secret_key"`
	AccessTokenDuration  time.Duration `mapstructure:"access_token_duration"`
	RefreshTokenDuration time.Duration `mapstructure:"refresh_token_duration"`
}

type AdminConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Email    string `mapstructure:"email"`
}

type AuthConfig struct {
	MasterKey   string    `mapstructure:"master_key"`
	JWT         JWTConfig `mapstructure:"jwt"`
	Dex         DexConfig `mapstructure:"dex"`
	RequireAuth bool      `mapstructure:"require_auth"`
}

type DexConfig struct {
	Enabled          bool              `mapstructure:"enabled"`
	Issuer           string            `mapstructure:"issuer"`        // Backend connection URL
	PublicIssuer     string            `mapstructure:"public_issuer"` // Frontend OAuth URL
	ClientID         string            `mapstructure:"client_id"`
	ClientSecret     string            `mapstructure:"client_secret"`
	RedirectURL      string            `mapstructure:"redirect_url"`
	Scopes           []string          `mapstructure:"scopes"`
	GroupMappings    map[string]string `mapstructure:"group_mappings"`
	EnabledProviders []string          `mapstructure:"enabled_providers"` // Which OAuth providers are enabled
}

type CacheConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	TTL      time.Duration `mapstructure:"ttl"`
	MaxSize  int           `mapstructure:"max_size"`
	Strategy string        `mapstructure:"strategy"`
}

type RateLimitConfig struct {
	Enabled            bool          `mapstructure:"enabled"`
	GlobalRPM          int           `mapstructure:"global_rpm"`
	ChatCompletionsRPM int           `mapstructure:"chat_completions_rpm"`
	CompletionsRPM     int           `mapstructure:"completions_rpm"`
	EmbeddingsRPM      int           `mapstructure:"embeddings_rpm"`
	RequestsPerMinute  int           `mapstructure:"requests_per_minute"`
	Burst              int           `mapstructure:"burst"`
	CleanupInterval    time.Duration `mapstructure:"cleanup_interval"`
}

type MonitoringConfig struct {
	EnableMetrics  bool   `mapstructure:"enable_metrics"`
	EnableTracing  bool   `mapstructure:"enable_tracing"`
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"`
	ServiceName    string `mapstructure:"service_name"`
}

type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
}

type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

type RealtimeConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	MaxSessions      int           `mapstructure:"max_sessions"`
	SessionTimeout   time.Duration `mapstructure:"session_timeout"`
	MaxMessageSize   int64         `mapstructure:"max_message_size"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout"`
	PingInterval     time.Duration `mapstructure:"ping_interval"`
	PongTimeout      time.Duration `mapstructure:"pong_timeout"`
	BufferSize       int           `mapstructure:"buffer_size"`
	EnableCompression bool         `mapstructure:"enable_compression"`
	AudioFormat      string        `mapstructure:"audio_format"`
	AudioSampleRate  int           `mapstructure:"audio_sample_rate"`
}

var cfg *Config

func Load(configPath string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	if configPath != "" {
		viper.AddConfigPath(configPath)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/etc/pllm")
	}

	// Set defaults
	setDefaults()

	// Bind environment variables
	viper.AutomaticEnv()
	bindEnvVars()

	// Read config file if exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Expand environment variables in model_list configs
	modelList := viper.Get("model_list")
	if models, ok := modelList.([]interface{}); ok {
		for i, modelRaw := range models {
			if model, ok := modelRaw.(map[string]interface{}); ok {
				// Check both "params" (new format) and "provider" (old format)
				var paramsMap map[string]interface{}
				if params, ok := model["params"].(map[string]interface{}); ok {
					paramsMap = params
				} else if provider, ok := model["provider"].(map[string]interface{}); ok {
					paramsMap = provider
				}

				if paramsMap != nil {
					if apiKey, ok := paramsMap["api_key"].(string); ok {
						// Expand environment variable if it starts with $
						if len(apiKey) > 2 && apiKey[0] == '$' && apiKey[1] == '{' {
							// Find the closing }
							endIdx := len(apiKey) - 1
							if apiKey[endIdx] == '}' {
								envVar := apiKey[2:endIdx] // Remove ${ and }
								if envVal := os.Getenv(envVar); envVal != "" {
									paramsMap["api_key"] = envVal
								}
							}
						}
					}
				}
			}
			models[i] = modelRaw
		}
		viper.Set("model_list", models)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Set default router settings if not configured
	if config.Router.RoutingStrategy == "" {
		config.Router.RoutingStrategy = "priority"
		config.Router.EnableLoadBalancing = true
		config.Router.MaxRetries = 3
		config.Router.DefaultTimeout = 60 * time.Second
		config.Router.HealthCheckInterval = 30 * time.Second
	}

	// Convert ModelConfig to ModelInstance format
	var convertedModels []ModelInstance
	for _, model := range config.RawModelList {
		instance := ConvertToModelInstance(model)
		instance.Source = "system"
		convertedModels = append(convertedModels, instance)
	}

	// Set the converted models
	config.ModelList = convertedModels

	// Initialize and load pricing manager
	pricingManager := GetPricingManager()
	if err := pricingManager.LoadDefaultPricing("internal/config"); err != nil {
		// Log warning but don't fail - pricing can work without default file
		fmt.Printf("Warning: Failed to load default pricing: %v\n", err)
	}
	
	// Add config overrides from model instances
	pricingManager.AddConfigOverrides(convertedModels)

	cfg = &config
	return cfg, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.admin_port", 8081)
	viper.SetDefault("server.metrics_port", 9090)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "300s")
	viper.SetDefault("server.idle_timeout", "120s")
	viper.SetDefault("server.graceful_shutdown", "30s")

	// Database defaults
	viper.SetDefault("database.max_connections", 100)
	viper.SetDefault("database.max_idle_connections", 10)
	viper.SetDefault("database.conn_max_lifetime", "1h")

	// Redis defaults
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 100)

	// JWT defaults
	viper.SetDefault("jwt.access_token_duration", "15m")
	viper.SetDefault("jwt.refresh_token_duration", "168h")

	// Cache defaults
	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.ttl", "3600s")
	viper.SetDefault("cache.max_size", 1000)
	viper.SetDefault("cache.strategy", "lru")

	// Rate limit defaults
	viper.SetDefault("rate_limit.enabled", true)
	viper.SetDefault("rate_limit.requests_per_minute", 600)
	viper.SetDefault("rate_limit.burst", 10)
	viper.SetDefault("rate_limit.cleanup_interval", "1m")

	// Monitoring defaults
	viper.SetDefault("monitoring.enable_metrics", true)
	viper.SetDefault("monitoring.enable_tracing", true)
	viper.SetDefault("monitoring.service_name", "pllm")

	// Logging defaults
	viper.SetDefault("logging.level", "debug")
	viper.SetDefault("logging.format", "console")
	viper.SetDefault("logging.output_path", "")

	// CORS defaults
	viper.SetDefault("cors.allow_credentials", true)
	viper.SetDefault("cors.max_age", 86400)

	// Auth defaults
	viper.SetDefault("auth.require_auth", false)
	viper.SetDefault("auth.dex.enabled", false)
	viper.SetDefault("auth.dex.scopes", []string{"openid", "profile", "email", "groups"})
	viper.SetDefault("auth.dex.enabled_providers", []string{})

	// Realtime defaults
	viper.SetDefault("realtime.enabled", false)
	viper.SetDefault("realtime.max_sessions", 1000)
	viper.SetDefault("realtime.session_timeout", "30m")
	viper.SetDefault("realtime.max_message_size", 1048576) // 1MB
	viper.SetDefault("realtime.write_timeout", "10s")
	viper.SetDefault("realtime.read_timeout", "60s")
	viper.SetDefault("realtime.ping_interval", "30s")
	viper.SetDefault("realtime.pong_timeout", "10s")
	viper.SetDefault("realtime.buffer_size", 4096)
	viper.SetDefault("realtime.enable_compression", false)
	viper.SetDefault("realtime.audio_format", "pcm16")
	viper.SetDefault("realtime.audio_sample_rate", 24000)

	// Guardrails defaults
	viper.SetDefault("guardrails.enabled", false)
	viper.SetDefault("guardrails.providers.presidio.analyzer_url", "http://localhost:5002")
	viper.SetDefault("guardrails.providers.presidio.anonymizer_url", "http://localhost:5001")
	viper.SetDefault("guardrails.providers.presidio.language", "en")
	viper.SetDefault("guardrails.providers.presidio.timeout", "10s")
	viper.SetDefault("guardrails.providers.lakera.base_url", "https://api.lakera.ai")
	viper.SetDefault("guardrails.providers.lakera.timeout", "5s")
	viper.SetDefault("guardrails.providers.openai.base_url", "https://api.openai.com/v1")
	viper.SetDefault("guardrails.providers.openai.timeout", "30s")
	viper.SetDefault("guardrails.providers.aporia.timeout", "10s")
}

func bindEnvVars() {
	// Server
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("server.admin_port", "ADMIN_PORT")
	_ = viper.BindEnv("server.metrics_port", "METRICS_PORT")
	_ = viper.BindEnv("server.read_timeout", "SERVER_READ_TIMEOUT")
	_ = viper.BindEnv("server.write_timeout", "SERVER_WRITE_TIMEOUT")
	_ = viper.BindEnv("server.idle_timeout", "SERVER_IDLE_TIMEOUT")

	// Database
	_ = viper.BindEnv("database.url", "DATABASE_URL")
	_ = viper.BindEnv("database.max_connections", "DATABASE_MAX_CONNECTIONS")
	_ = viper.BindEnv("database.max_idle_connections", "DATABASE_MAX_IDLE_CONNECTIONS")

	// Redis
	_ = viper.BindEnv("redis.url", "REDIS_URL")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")

	// JWT
	_ = viper.BindEnv("jwt.secret_key", "JWT_SECRET_KEY")
	_ = viper.BindEnv("jwt.access_token_duration", "JWT_ACCESS_TOKEN_DURATION")
	_ = viper.BindEnv("jwt.refresh_token_duration", "JWT_REFRESH_TOKEN_DURATION")

	// Admin
	_ = viper.BindEnv("admin.username", "ADMIN_USERNAME")
	_ = viper.BindEnv("admin.password", "ADMIN_PASSWORD")
	_ = viper.BindEnv("admin.email", "ADMIN_EMAIL")

	// Auth
	_ = viper.BindEnv("auth.master_key", "PLLM_MASTER_KEY")
	_ = viper.BindEnv("auth.require_auth", "PLLM_REQUIRE_AUTH")

	// Dex OAuth
	_ = viper.BindEnv("auth.dex.enabled", "DEX_ENABLED")
	_ = viper.BindEnv("auth.dex.issuer", "DEX_ISSUER")
	_ = viper.BindEnv("auth.dex.public_issuer", "DEX_PUBLIC_ISSUER")
	_ = viper.BindEnv("auth.dex.client_id", "DEX_CLIENT_ID")
	_ = viper.BindEnv("auth.dex.client_secret", "DEX_CLIENT_SECRET")
	_ = viper.BindEnv("auth.dex.redirect_url", "DEX_REDIRECT_URL")
	_ = viper.BindEnv("auth.dex.enabled_providers", "DEX_ENABLED_PROVIDERS")

	// Cache
	_ = viper.BindEnv("cache.ttl", "CACHE_TTL")
	_ = viper.BindEnv("cache.max_size", "CACHE_MAX_SIZE")

	// Rate Limiting
	_ = viper.BindEnv("rate_limit.requests_per_minute", "RATE_LIMIT_REQUESTS_PER_MINUTE")
	_ = viper.BindEnv("rate_limit.burst", "RATE_LIMIT_BURST")

	// Monitoring
	_ = viper.BindEnv("monitoring.enable_metrics", "ENABLE_METRICS")
	_ = viper.BindEnv("monitoring.enable_tracing", "ENABLE_TRACING")
	_ = viper.BindEnv("monitoring.jaeger_endpoint", "JAEGER_ENDPOINT")

	// Logging
	_ = viper.BindEnv("logging.level", "LOG_LEVEL")
	_ = viper.BindEnv("logging.format", "LOG_FORMAT")

	// CORS
	_ = viper.BindEnv("cors.allowed_origins", "CORS_ALLOWED_ORIGINS")
	_ = viper.BindEnv("cors.allowed_methods", "CORS_ALLOWED_METHODS")
	_ = viper.BindEnv("cors.allowed_headers", "CORS_ALLOWED_HEADERS")

	// Realtime
	_ = viper.BindEnv("realtime.enabled", "REALTIME_ENABLED")
	_ = viper.BindEnv("realtime.max_sessions", "REALTIME_MAX_SESSIONS")
	_ = viper.BindEnv("realtime.session_timeout", "REALTIME_SESSION_TIMEOUT")
	_ = viper.BindEnv("realtime.max_message_size", "REALTIME_MAX_MESSAGE_SIZE")
	_ = viper.BindEnv("realtime.write_timeout", "REALTIME_WRITE_TIMEOUT")
	_ = viper.BindEnv("realtime.read_timeout", "REALTIME_READ_TIMEOUT")
	_ = viper.BindEnv("realtime.ping_interval", "REALTIME_PING_INTERVAL")
	_ = viper.BindEnv("realtime.pong_timeout", "REALTIME_PONG_TIMEOUT")
	_ = viper.BindEnv("realtime.buffer_size", "REALTIME_BUFFER_SIZE")
	_ = viper.BindEnv("realtime.enable_compression", "REALTIME_ENABLE_COMPRESSION")
	_ = viper.BindEnv("realtime.audio_format", "REALTIME_AUDIO_FORMAT")
	_ = viper.BindEnv("realtime.audio_sample_rate", "REALTIME_AUDIO_SAMPLE_RATE")
}

func Get() *Config {
	return cfg
}
