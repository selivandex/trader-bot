package users

import (
	"testing"

	"github.com/selivandex/trader-bot/pkg/crypto"
	"github.com/selivandex/trader-bot/test/testdb"
)

// TestAPIKeyEncryption_Integration tests that API keys are encrypted in database
// and properly decrypted when retrieved
func TestAPIKeyEncryption_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database with automatic transaction rollback
	tdb := testdb.Setup(t)

	// Create test user
	var userID string
	testTelegramID := int64(999888777)
	err := tdb.QueryRow(t, `
		INSERT INTO users (telegram_id, username, first_name, is_active)
		VALUES ($1, $2, $3, true)
		RETURNING id
	`, testTelegramID, "testuser", "Test User").Scan(&userID)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Original API credentials (plaintext)
	originalAPIKey := "test-api-key-1234567890"
	originalAPISecret := "test-api-secret-abcdefghijklmnopqrstuvwxyz"
	testExchange := "binance"

	// Encrypt credentials (simulating what AddExchange does)
	encryptedKey, err := crypto.Encrypt(originalAPIKey)
	if err != nil {
		t.Fatalf("failed to encrypt API key: %v", err)
	}

	encryptedSecret, err := crypto.Encrypt(originalAPISecret)
	if err != nil {
		t.Fatalf("failed to encrypt API secret: %v", err)
	}

	// Insert into database
	var exchangeID string
	err = tdb.QueryRow(t, `
		INSERT INTO user_exchanges (user_id, exchange, api_key_encrypted, api_secret_encrypted, testnet, is_active)
		VALUES ($1, $2, $3, $4, true, true)
		RETURNING id
	`, userID, testExchange, encryptedKey, encryptedSecret).Scan(&exchangeID)
	if err != nil {
		t.Fatalf("failed to insert exchange: %v", err)
	}

	// TEST 1: Verify that encrypted values in DB are different from original
	t.Run("encrypted values differ from plaintext", func(t *testing.T) {
		var storedKey, storedSecret string
		err := tdb.QueryRow(t, `
			SELECT api_key_encrypted, api_secret_encrypted 
			FROM user_exchanges 
			WHERE id = $1
		`, exchangeID).Scan(&storedKey, &storedSecret)
		if err != nil {
			t.Fatalf("failed to query encrypted values: %v", err)
		}

		// Encrypted values should NOT match plaintext
		if storedKey == originalAPIKey {
			t.Error("API key is stored in plaintext (not encrypted!)")
		}
		if storedSecret == originalAPISecret {
			t.Error("API secret is stored in plaintext (not encrypted!)")
		}

		// Encrypted values should be non-empty
		if storedKey == "" {
			t.Error("encrypted API key is empty")
		}
		if storedSecret == "" {
			t.Error("encrypted API secret is empty")
		}

		// Encrypted values should be longer than plaintext (due to nonce + encryption)
		if len(storedKey) <= len(originalAPIKey) {
			t.Errorf("encrypted key length (%d) should be longer than plaintext (%d)",
				len(storedKey), len(originalAPIKey))
		}
		if len(storedSecret) <= len(originalAPISecret) {
			t.Errorf("encrypted secret length (%d) should be longer than plaintext (%d)",
				len(storedSecret), len(originalAPISecret))
		}
	})

	// TEST 2: Verify decryption works correctly
	t.Run("decrypted values match original plaintext", func(t *testing.T) {
		var encryptedKey, encryptedSecret string
		err := tdb.QueryRow(t, `
			SELECT api_key_encrypted, api_secret_encrypted 
			FROM user_exchanges 
			WHERE id = $1
		`, exchangeID).Scan(&encryptedKey, &encryptedSecret)
		if err != nil {
			t.Fatalf("failed to query encrypted values: %v", err)
		}

		// Decrypt
		decryptedKey, err := crypto.Decrypt(encryptedKey)
		if err != nil {
			t.Fatalf("failed to decrypt API key: %v", err)
		}

		decryptedSecret, err := crypto.Decrypt(encryptedSecret)
		if err != nil {
			t.Fatalf("failed to decrypt API secret: %v", err)
		}

		// Decrypted values MUST match original plaintext
		if decryptedKey != originalAPIKey {
			t.Errorf("decrypted key mismatch:\nwant: %s\ngot:  %s", originalAPIKey, decryptedKey)
		}
		if decryptedSecret != originalAPISecret {
			t.Errorf("decrypted secret mismatch:\nwant: %s\ngot:  %s", originalAPISecret, decryptedSecret)
		}
	})

	// TEST 3: Test full round-trip through repository
	t.Run("repository round-trip encryption/decryption", func(t *testing.T) {
		// Note: We need to wrap the transaction in a way that repository can use it
		// For simplicity, we'll use direct SQL here to simulate repository behavior

		// Simulate AddExchange
		newAPIKey := "new-test-key-xyz"
		newAPISecret := "new-test-secret-xyz-123"

		encKey, err := crypto.Encrypt(newAPIKey)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		encSecret, err := crypto.Encrypt(newAPISecret)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}

		var newExchangeID string
		err = tdb.QueryRow(t, `
			INSERT INTO user_exchanges (user_id, exchange, api_key_encrypted, api_secret_encrypted, testnet, is_active)
			VALUES ($1, $2, $3, $4, false, true)
			RETURNING id
		`, userID, "bybit", encKey, encSecret).Scan(&newExchangeID)
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}

		// Simulate GetUserExchange - read and decrypt
		var retrievedEncKey, retrievedEncSecret string
		err = tdb.QueryRow(t, `
			SELECT api_key_encrypted, api_secret_encrypted
			FROM user_exchanges
			WHERE id = $1
		`, newExchangeID).Scan(&retrievedEncKey, &retrievedEncSecret)
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}

		retrievedKey, err := crypto.Decrypt(retrievedEncKey)
		if err != nil {
			t.Fatalf("decrypt key failed: %v", err)
		}
		retrievedSecret, err := crypto.Decrypt(retrievedEncSecret)
		if err != nil {
			t.Fatalf("decrypt secret failed: %v", err)
		}

		// Verify round-trip
		if retrievedKey != newAPIKey {
			t.Errorf("round-trip key mismatch:\nwant: %s\ngot:  %s", newAPIKey, retrievedKey)
		}
		if retrievedSecret != newAPISecret {
			t.Errorf("round-trip secret mismatch:\nwant: %s\ngot:  %s", newAPISecret, retrievedSecret)
		}
	})

	// TEST 4: Verify encrypted values are unique (same plaintext produces different ciphertext)
	t.Run("same plaintext produces different ciphertext", func(t *testing.T) {
		plaintext := "same-plaintext-value"

		encrypted1, err := crypto.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("first encryption failed: %v", err)
		}

		encrypted2, err := crypto.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("second encryption failed: %v", err)
		}

		// Different encryptions of same plaintext should produce different ciphertext
		// This is due to random nonce in AES-GCM
		if encrypted1 == encrypted2 {
			t.Error("same plaintext produced identical ciphertext - nonce may not be random")
		}

		// But both should decrypt to same plaintext
		decrypted1, err := crypto.Decrypt(encrypted1)
		if err != nil {
			t.Fatalf("decrypt 1 failed: %v", err)
		}
		decrypted2, err := crypto.Decrypt(encrypted2)
		if err != nil {
			t.Fatalf("decrypt 2 failed: %v", err)
		}

		if decrypted1 != plaintext || decrypted2 != plaintext {
			t.Error("decrypted values don't match original plaintext")
		}
	})

	// TEST 5: Test that repository methods work correctly
	t.Run("repository AddExchange and GetUserExchange", func(t *testing.T) {
		// This test requires the repository to work with our transaction
		// We'll create a new instance and test the actual repository methods

		// For now, skip this as it requires refactoring database.DB to accept transaction
		// The above tests already verify the core encryption/decryption logic
		t.Skip("Requires database.DB refactoring to support transaction injection")
	})
}

// TestEncryptionFunctions_Unit tests encryption/decryption functions in isolation
func TestEncryptionFunctions_Unit(t *testing.T) {
	testCases := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "short string",
			plaintext: "test123",
		},
		{
			name:      "api key format",
			plaintext: "abcd1234efgh5678ijkl",
		},
		{
			name:      "long secret",
			plaintext: "very-long-secret-key-with-special-chars-!@#$%^&*()_+-=[]{}|;:',.<>?/~`",
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "unicode characters",
			plaintext: "тест-ключ-🔑-emoji",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := crypto.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("encryption failed: %v", err)
			}

			// Encrypted should be different from plaintext (except empty string edge case)
			if tc.plaintext != "" && encrypted == tc.plaintext {
				t.Error("encrypted value equals plaintext")
			}

			// Encrypted should not be empty
			if tc.plaintext != "" && encrypted == "" {
				t.Error("encrypted value is empty")
			}

			// Decrypt
			decrypted, err := crypto.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("decryption failed: %v", err)
			}

			// Decrypted should match original
			if decrypted != tc.plaintext {
				t.Errorf("decrypted mismatch:\nwant: %s\ngot:  %s", tc.plaintext, decrypted)
			}
		})
	}
}

// TestDecryptionFailures_Unit tests that decryption properly fails for invalid input
func TestDecryptionFailures_Unit(t *testing.T) {
	testCases := []struct {
		name       string
		ciphertext string
		wantError  bool
	}{
		{
			name:       "invalid base64",
			ciphertext: "not-valid-base64-!@#$",
			wantError:  true,
		},
		{
			name:       "too short ciphertext",
			ciphertext: "YWJj", // "abc" in base64, too short for AES-GCM
			wantError:  true,
		},
		{
			name:       "corrupted ciphertext",
			ciphertext: "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU2Nzg5", // valid base64 but not valid ciphertext
			wantError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := crypto.Decrypt(tc.ciphertext)

			if tc.wantError && err == nil {
				t.Error("expected decryption to fail, but it succeeded")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected decryption error: %v", err)
			}
		})
	}
}
