package database

import (
	"fmt"
	"log"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database connection
var DB *gorm.DB

// Connect establishes a connection to the PostgreSQL database
func Connect(databaseURL string, mode string) error {
	var err error

	// Configure GORM logger
	var gormLogLevel logger.LogLevel
	if strings.EqualFold(mode, "DEV") {
		gormLogLevel = logger.Info
	} else {
		gormLogLevel = logger.Error
	}
	config := &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	}

	// Connect to PostgreSQL
	DB, err = gorm.Open(postgres.Open(databaseURL), config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying sql.DB for connection pool configuration
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)

	log.Println("Successfully connected to database")
	return nil
}

// Migrate runs database migrations
func Migrate() error {
	if DB == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Enable UUID extension first
	err := DB.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error
	if err != nil {
		return fmt.Errorf("failed to enable UUID extension: %w", err)
	}

	// Auto-migrate all models
	err = DB.AutoMigrate(
		&Host{},
		&Stack{},
		&User{},
		&APIKey{},
		&RefreshToken{},
		&AuditLog{},
		&DashboardTask{},
		&NetworkTopology{},
		&VolumeTopology{},
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database migration completed successfully")
	return nil
}

// Close closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// GetDB returns the database connection
func GetDB() *gorm.DB {
	return DB
}
