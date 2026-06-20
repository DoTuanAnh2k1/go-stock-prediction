package testutil

import (
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"os"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestDBConfig holds test database connection parameters.
// Override via environment variables: TEST_MYSQL_HOST, TEST_MYSQL_PORT, etc.
type TestDBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// DefaultTestDBConfig returns config from env vars with sensible defaults.
func DefaultTestDBConfig() TestDBConfig {
	return TestDBConfig{
		Host:     envOrDefault("TEST_MYSQL_HOST", "localhost"),
		Port:     envOrDefault("TEST_MYSQL_PORT", "3307"),
		User:     envOrDefault("TEST_MYSQL_USER", "root"),
		Password: envOrDefault("TEST_MYSQL_PASSWORD", "test123"),
		DBName:   envOrDefault("TEST_MYSQL_DB", "go_stock_prediction_test"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// SetupTestDB connects to the test MySQL, runs auto-migrate, and returns
// the *gorm.DB plus a cleanup function that truncates all tables.
// Call this in TestMain or at the top of integration tests.
// Skips the test (t.Skip) if the test DB is unreachable.
func SetupTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	// Ensure logger is initialized
	logger.Init()

	cfg := DefaultTestDBConfig()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Asia%%2FHo_Chi_Minh",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Skipf("Test DB not available (skipping integration test): %v", err)
	}

	// Verify connectivity
	sqlDB, err := db.DB()
	if err != nil {
		t.Skipf("Test DB not available: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Skipf("Test DB not reachable: %v", err)
	}

	// Auto-migrate all models
	if err := db.AutoMigrate(modelsdb.AllModels...); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	cleanup := func() {
		CleanupTestDB(t, db)
	}

	return db, cleanup
}

// CleanupTestDB truncates all test tables in the correct order (foreign keys).
func CleanupTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	tables := []string{
		"gold_prices",
		"macro_indicators",
		"sync_logs",
	}
	db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	for _, table := range tables {
		if err := db.Exec("TRUNCATE TABLE " + table).Error; err != nil {
			// Table might not exist yet, that's ok
			t.Logf("Warning: could not truncate %s: %v", table, err)
		}
	}
	db.Exec("SET FOREIGN_KEY_CHECKS = 1")
}

// SeedTestData loads all fixture data into the test database.
func SeedTestData(t *testing.T, db *gorm.DB) {
	t.Helper()

	fixtures, err := LoadAllFixtures()
	if err != nil {
		t.Fatalf("Failed to load fixtures: %v", err)
	}

	if len(fixtures.GoldPrices) > 0 {
		if err := db.Create(&fixtures.GoldPrices).Error; err != nil {
			t.Fatalf("Failed to seed gold_prices: %v", err)
		}
	}
}
