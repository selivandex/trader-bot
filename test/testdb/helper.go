package testdb

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/selivandex/trader-bot/internal/adapters/database"
)

// TestDB wraps database for testing with automatic rollback
type TestDB struct {
	DB *database.DB
	tx *sql.Tx
}

// Setup creates test database connection and begins transaction
func Setup(t *testing.T) *TestDB {
	t.Helper()

	// Get test database DSN from env or use default
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=trader password=trader dbname=trader_test sslmode=disable"
	}

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		t.Fatalf("failed to ping test database: %v (DSN: %s)", err, dsn)
	}

	// Begin transaction
	tx, err := conn.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Wrap in database.DB
	db := &database.DB{}
	// Note: We would need to expose SetConn() in database.DB or use transaction directly

	testDB := &TestDB{
		DB: db,
		tx: tx,
	}

	// Register cleanup
	t.Cleanup(func() {
		testDB.Teardown(t)
	})

	return testDB
}

// Teardown rolls back transaction and closes connection
func (tdb *TestDB) Teardown(t *testing.T) {
	t.Helper()

	if tdb.tx != nil {
		// Rollback transaction - this removes all test data
		if err := tdb.tx.Rollback(); err != nil {
			t.Logf("warning: failed to rollback transaction: %v", err)
		}
	}

	if tdb.DB != nil {
		if err := tdb.DB.Close(); err != nil {
			t.Logf("warning: failed to close database: %v", err)
		}
	}
}

// Exec executes SQL in test transaction
func (tdb *TestDB) Exec(t *testing.T, query string, args ...interface{}) sql.Result {
	t.Helper()

	result, err := tdb.tx.Exec(query, args...)
	if err != nil {
		t.Fatalf("failed to execute query: %v\nQuery: %s", err, query)
	}

	return result
}

// Query executes query in test transaction
func (tdb *TestDB) Query(t *testing.T, query string, args ...interface{}) *sql.Rows {
	t.Helper()

	rows, err := tdb.tx.Query(query, args...)
	if err != nil {
		t.Fatalf("failed to query: %v\nQuery: %s", err, query)
	}

	return rows
}

// QueryRow executes query and returns single row
func (tdb *TestDB) QueryRow(t *testing.T, query string, args ...interface{}) *sql.Row {
	t.Helper()
	return tdb.tx.QueryRow(query, args...)
}

// GetTx returns underlying transaction for direct use
func (tdb *TestDB) GetTx() *sql.Tx {
	return tdb.tx
}

// SeedTestData inserts common test data
func (tdb *TestDB) SeedTestData(t *testing.T) {
	t.Helper()

	// Insert test user
	tdb.Exec(t, `
		INSERT INTO users (id, telegram_id, username, first_name, is_active, created_at, updated_at)
		VALUES (1, 123456789, 'testuser', 'Test', true, NOW(), NOW())
	`)

	// Insert test config
	tdb.Exec(t, `
		INSERT INTO user_configs (
			user_id, exchange, api_key, api_secret, testnet, symbol,
			initial_balance, max_position_percent, max_leverage,
			stop_loss_percent, take_profit_percent, is_trading,
			created_at, updated_at
		) VALUES (
			1, 'binance', 'test_key', 'test_secret', true, 'BTC/USDT',
			1000, 30, 3, 2, 5, false, NOW(), NOW()
		)
	`)

	// Insert test state
	tdb.Exec(t, `
		INSERT INTO user_states (
			user_id, symbol, mode, status, balance, equity, daily_pnl, peak_equity, updated_at
		) VALUES (
			1, 'BTC/USDT', 'paper', 'stopped', 1000, 1000, 0, 1000, NOW()
		)
	`)
}

// CreateTestUser creates test user and returns ID
func (tdb *TestDB) CreateTestUser(t *testing.T, telegramID int64, username string) int64 {
	t.Helper()

	var userID int64
	err := tdb.tx.QueryRow(`
		INSERT INTO users (telegram_id, username, first_name, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test User', true, NOW(), NOW())
		RETURNING id
	`, telegramID, username).Scan(&userID)

	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return userID
}

// CreateTestConfig creates test configuration for user
func (tdb *TestDB) CreateTestConfig(t *testing.T, userID int64, symbol string, balance float64) {
	t.Helper()

	tdb.Exec(t, `
		INSERT INTO user_configs (
			user_id, exchange, api_key, api_secret, testnet, symbol,
			initial_balance, max_position_percent, max_leverage,
			stop_loss_percent, take_profit_percent, is_trading,
			created_at, updated_at
		) VALUES (
			$1, 'binance', 'test_key', 'test_secret', true, $2,
			$3, 30, 3, 2, 5, false, NOW(), NOW()
		)
	`, userID, symbol, balance)

	// Create state
	tdb.Exec(t, `
		INSERT INTO user_states (
			user_id, symbol, mode, status, balance, equity, daily_pnl, peak_equity, updated_at
		) VALUES (
			$1, $2, 'paper', 'stopped', $3, $3, 0, $3, NOW()
		)
	`, userID, symbol, balance)
}

// AssertUserExists checks if user exists
func (tdb *TestDB) AssertUserExists(t *testing.T, telegramID int64) {
	t.Helper()

	var count int
	err := tdb.tx.QueryRow("SELECT COUNT(*) FROM users WHERE telegram_id = $1", telegramID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to check user: %v", err)
	}

	if count == 0 {
		t.Errorf("user with telegram_id %d does not exist", telegramID)
	}
}

// AssertConfigExists checks if config exists
func (tdb *TestDB) AssertConfigExists(t *testing.T, userID int64, symbol string) {
	t.Helper()

	var count int
	err := tdb.tx.QueryRow(`
		SELECT COUNT(*) FROM user_configs WHERE user_id = $1 AND symbol = $2
	`, userID, symbol).Scan(&count)

	if err != nil {
		t.Fatalf("failed to check config: %v", err)
	}

	if count == 0 {
		t.Errorf("config for user %d, symbol %s does not exist", userID, symbol)
	}
}

// AssertTradeCount checks trade count for user
func (tdb *TestDB) AssertTradeCount(t *testing.T, userID int64, expectedCount int) {
	t.Helper()

	var count int
	err := tdb.tx.QueryRow("SELECT COUNT(*) FROM trades WHERE user_id = $1", userID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count trades: %v", err)
	}

	if count != expectedCount {
		t.Errorf("expected %d trades, got %d", expectedCount, count)
	}
}
