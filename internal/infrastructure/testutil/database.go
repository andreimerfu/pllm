package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// NewTestDB creates a PostgreSQL test database using Testcontainers
func NewTestDB(t *testing.T) (*gorm.DB, func()) {
	ctx := context.Background()

	// Start PostgreSQL container with Testcontainers and proper wait strategies
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	// Get connection string
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	// Add a small delay to ensure PostgreSQL is fully ready
	time.Sleep(1 * time.Second)

	// Connect with GORM
	db, err := gorm.Open(postgresdriver.Open(connStr), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to test database")

	// Auto-migrate all models
	err = db.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.Key{},
		&models.Usage{},
		&models.TeamMember{},
		&models.Audit{},
		&models.SystemMetrics{},
		&models.ModelMetrics{},
		&models.UserMetrics{},
		&models.TeamMetrics{},
	)
	require.NoError(t, err, "Failed to migrate test database")

	// Return cleanup function that terminates the container
	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate PostgreSQL container: %v", err)
		}
	}

	return db, cleanup
}

// NewTestRedis creates a Redis test instance using Testcontainers
func NewTestRedis(t *testing.T) (*redis.Client, func()) {
	ctx := context.Background()

	// Start Redis container with Testcontainers
	container, err := testredis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err, "Failed to start Redis container")

	// Get connection string
	connStr, err := container.ConnectionString(ctx)
	require.NoError(t, err, "Failed to get Redis connection string")

	// Parse Redis URL
	opt, err := redis.ParseURL(connStr)
	require.NoError(t, err, "Failed to parse Redis URL")

	// Create Redis client
	client := redis.NewClient(opt)

	// Test connection
	err = client.Ping(ctx).Err()
	require.NoError(t, err, "Failed to ping Redis")

	// Return cleanup function that terminates the container
	cleanup := func() {
		client.Close()
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}

	return client, cleanup
}

// NewTestRedisWithURL creates a Redis test instance and returns client with connection URL
func NewTestRedisWithURL(t *testing.T) (*redis.Client, string, func()) {
	ctx := context.Background()

	// Start Redis container
	container, err := testredis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err, "Failed to start Redis container")

	// Get connection details
	host, err := container.Host(ctx)
	require.NoError(t, err, "Failed to get Redis host")

	port, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err, "Failed to get Redis port")

	// Build connection URL
	connURL := fmt.Sprintf("redis://%s:%s", host, port.Port())

	// Parse and create client
	opt, err := redis.ParseURL(connURL)
	require.NoError(t, err, "Failed to parse Redis URL")

	client := redis.NewClient(opt)

	// Test connection
	err = client.Ping(ctx).Err()
	require.NoError(t, err, "Failed to ping Redis")

	cleanup := func() {
		client.Close()
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}

	return client, connURL, cleanup
}