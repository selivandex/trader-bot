package users

import (
	"context"
	"testing"

	"github.com/alexanderselivanov/trader/pkg/models"
	"github.com/alexanderselivanov/trader/test/testdb"
)

func TestRepository_CreateUser(t *testing.T) {
	db := testdb.Setup(t)
	repo := NewRepository(db.DB)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, 123456789, "testuser", "Test User")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.ID == 0 {
		t.Error("User ID should not be 0")
	}

	if user.TelegramID != 123456789 {
		t.Errorf("Expected telegram_id 123456789, got %d", user.TelegramID)
	}

	// Verify in database
	db.AssertUserExists(t, 123456789)
}

func TestRepository_GetUserByTelegramID(t *testing.T) {
	db := testdb.Setup(t)
	repo := NewRepository(db.DB)
	ctx := context.Background()

	// Create user
	created, err := repo.CreateUser(ctx, 987654321, "findme", "Find Me")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Find user
	found, err := repo.GetUserByTelegramID(ctx, 987654321)
	if err != nil {
		t.Fatalf("Failed to find user: %v", err)
	}

	if found == nil {
		t.Fatal("User should be found")
	}

	if found.ID != created.ID {
		t.Errorf("Expected user ID %d, got %d", created.ID, found.ID)
	}

	// Try non-existent user
	notFound, err := repo.GetUserByTelegramID(ctx, 999999999)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if notFound != nil {
		t.Error("Should return nil for non-existent user")
	}
}

func TestRepository_AddPairConfig(t *testing.T) {
	db := testdb.Setup(t)
	repo := NewRepository(db.DB)
	ctx := context.Background()

	// Create user
	userID := db.CreateTestUser(t, 111222333, "multitest")

	// Add first pair
	config1 := &models.UserConfig{
		UserID:             userID,
		Exchange:           "binance",
		APIKey:             "key1",
		APISecret:          "secret1",
		Testnet:            true,
		Symbol:             "BTC/USDT",
		InitialBalance:     models.NewDecimal(1000),
		MaxPositionPercent: models.NewDecimal(30),
		MaxLeverage:        3,
		StopLossPercent:    models.NewDecimal(2),
		TakeProfitPercent:  models.NewDecimal(5),
	}

	err := repo.AddPairConfig(ctx, config1)
	if err != nil {
		t.Fatalf("Failed to add pair: %v", err)
	}

	// Add second pair
	config2 := &models.UserConfig{
		UserID:             userID,
		Exchange:           "binance",
		APIKey:             "key1",
		APISecret:          "secret1",
		Testnet:            true,
		Symbol:             "ETH/USDT",
		InitialBalance:     models.NewDecimal(500),
		MaxPositionPercent: models.NewDecimal(30),
		MaxLeverage:        3,
		StopLossPercent:    models.NewDecimal(2),
		TakeProfitPercent:  models.NewDecimal(5),
	}

	err = repo.AddPairConfig(ctx, config2)
	if err != nil {
		t.Fatalf("Failed to add second pair: %v", err)
	}

	// Verify both configs exist
	db.AssertConfigExists(t, userID, "BTC/USDT")
	db.AssertConfigExists(t, userID, "ETH/USDT")

	// Get all configs
	configs, err := repo.GetAllConfigs(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get configs: %v", err)
	}

	if len(configs) != 2 {
		t.Errorf("Expected 2 configs, got %d", len(configs))
	}
}

func TestRepository_RemovePairConfig(t *testing.T) {
	db := testdb.Setup(t)
	repo := NewRepository(db.DB)
	ctx := context.Background()

	userID := db.CreateTestUser(t, 444555666, "removetest")
	db.CreateTestConfig(t, userID, "BTC/USDT", 1000)

	// Remove pair
	err := repo.RemovePairConfig(ctx, userID, "BTC/USDT")
	if err != nil {
		t.Fatalf("Failed to remove pair: %v", err)
	}

	// Verify removed
	config, err := repo.GetConfigBySymbol(ctx, userID, "BTC/USDT")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config != nil {
		t.Error("Config should be removed")
	}
}

func TestRepository_CannotRemoveActivePair(t *testing.T) {
	db := testdb.Setup(t)
	repo := NewRepository(db.DB)
	ctx := context.Background()

	userID := db.CreateTestUser(t, 777888999, "activetest")
	db.CreateTestConfig(t, userID, "BTC/USDT", 1000)

	// Set trading to active
	db.Exec(t, `UPDATE user_configs SET is_trading = true WHERE user_id = $1`, userID)

	// Try to remove active pair
	err := repo.RemovePairConfig(ctx, userID, "BTC/USDT")
	if err == nil {
		t.Error("Should not allow removing active trading pair")
	}
}
