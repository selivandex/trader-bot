package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/pkg/logger"
)

// DB wraps database connection (now using sqlx)
type DB struct {
	conn *sqlx.DB
}

// New creates new database connection
func New(cfg *config.DatabaseConfig) (*DB, error) {
	dsn := cfg.GetDSN()

	conn, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set connection pool parameters
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connection established",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Name),
	)

	return &DB{conn: conn}, nil
}

// Close closes database connection
func (db *DB) Close() error {
	if db.conn != nil {
		logger.Info("closing database connection")
		return db.conn.Close()
	}
	return nil
}

// Conn returns underlying *sql.DB connection (for migrations and legacy code)
func (db *DB) Conn() *sql.DB {
	return db.conn.DB
}

// DB returns sqlx.DB for modern code
func (db *DB) DB() *sqlx.DB {
	return db.conn
}

// Ping checks if database is alive
func (db *DB) Ping() error {
	return db.conn.Ping()
}

// Begin starts a new transaction
func (db *DB) Begin() (*sql.Tx, error) {
	return db.conn.Begin()
}

// BeginTxx starts a new sqlx transaction
func (db *DB) BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error) {
	return db.conn.BeginTxx(ctx, opts)
}

// Health checks database health
func (db *DB) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.conn.PingContext(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}
