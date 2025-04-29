package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/viper"
)

// InitPostgres initializes and configures PostgreSQL database connection
func InitPostgres(cfg *viper.Viper) (*sqlx.DB, error) {
	// Build connection string from configuration values
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.GetString("pg-host"),     // Database host address
		cfg.GetInt("pg-port"),        // Connection port
		cfg.GetString("pg-user"),     // Database user
		cfg.GetString("pg-password"), // User password
		cfg.GetString("pg-db"),       // Database name
		cfg.GetString("pg-ssl"),      // SSL mode (disable/require/verify-full)
	)

	// Establish database connection
	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(25)                 // Maximum open connections
	db.SetMaxIdleConns(25)                 // Maximum idle connections
	db.SetConnMaxLifetime(5 * time.Minute) // Maximum connection lifetime

	// Verify connection with ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connection verification failed: %w", err)
	}

	return db, nil
}
