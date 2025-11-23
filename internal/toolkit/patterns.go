package toolkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ============ Pattern Recognition Tools Implementation ============

// FindSimilarPatterns finds similar historical patterns
func (t *LocalToolkit) FindSimilarPatterns(ctx context.Context, symbol, timeframe string, currentCandles []models.Candle, lookback int) ([]SimilarPattern, error) {
	logger.Debug("toolkit: find_similar_patterns",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
		zap.Int("current_candles", len(currentCandles)),
		zap.Int("lookback", lookback),
	)

	if len(currentCandles) < 5 {
		return nil, fmt.Errorf("need at least 5 candles for pattern matching")
	}

	// Get historical candles from cache
	historicalCandles, err := t.GetCandles(ctx, symbol, timeframe, lookback)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical candles: %w", err)
	}

	// Normalize current pattern
	currentPattern := normalizePattern(currentCandles)

	// Sliding window to find similar patterns
	patternLength := len(currentCandles)
	similarities := []SimilarPattern{}

	for i := 0; i <= len(historicalCandles)-patternLength-5; i++ {
		window := historicalCandles[i : i+patternLength]
		historicalPattern := normalizePattern(window)

		// Calculate similarity (cosine similarity)
		similarity := calculateSimilarity(currentPattern, historicalPattern)

		if similarity > 0.85 { // High similarity threshold
			// Check what happened next (next 5 candles)
			nextCandles := historicalCandles[i+patternLength : i+patternLength+5]
			outcome, outcomePercent := calculateOutcome(window[len(window)-1], nextCandles)

			patternHash := hashPattern(historicalPattern)

			similarities = append(similarities, SimilarPattern{
				StartTime:      window[0].Timestamp,
				Similarity:     similarity,
				Outcome:        outcome,
				OutcomePercent: outcomePercent,
				Duration:       nextCandles[len(nextCandles)-1].Timestamp.Sub(window[0].Timestamp),
				PatternHash:    patternHash,
			})
		}
	}

	// Sort by similarity (highest first)
	// Return top 10
	if len(similarities) > 10 {
		similarities = similarities[:10]
	}

	return similarities, nil
}

// GetPatternOutcome gets historical success rate of pattern
func (t *LocalToolkit) GetPatternOutcome(ctx context.Context, patternHash string) (*PatternStats, error) {
	logger.Debug("toolkit: get_pattern_outcome",
		zap.String("agent_id", t.agentID),
		zap.String("pattern_hash", patternHash),
	)

	// TODO: Store pattern outcomes in database for faster lookup
	// For now, return empty stats
	return &PatternStats{
		PatternHash:      patternHash,
		TotalOccurrences: 0,
		BullishCount:     0,
		BearishCount:     0,
		SuccessRate:      0.5,
		AvgOutcome:       0,
		BestOutcome:      0,
		WorstOutcome:     0,
	}, nil
}

// Helper functions for pattern recognition

// normalizePattern converts candles to normalized vector (0-1 range)
func normalizePattern(candles []models.Candle) []float64 {
	if len(candles) == 0 {
		return []float64{}
	}

	closes := make([]float64, len(candles))
	for i, candle := range candles {
		closes[i] = candle.Close.InexactFloat64()
	}

	// Find min and max
	minPrice := closes[0]
	maxPrice := closes[0]
	for _, price := range closes {
		if price < minPrice {
			minPrice = price
		}
		if price > maxPrice {
			maxPrice = price
		}
	}

	// Normalize to 0-1
	normalized := make([]float64, len(closes))
	priceRange := maxPrice - minPrice
	if priceRange == 0 {
		priceRange = 1 // Avoid division by zero
	}

	for i, price := range closes {
		normalized[i] = (price - minPrice) / priceRange
	}

	return normalized
}

// calculateSimilarity calculates cosine similarity between two patterns
func calculateSimilarity(pattern1, pattern2 []float64) float64 {
	if len(pattern1) != len(pattern2) {
		return 0
	}

	dotProduct := 0.0
	mag1 := 0.0
	mag2 := 0.0

	for i := 0; i < len(pattern1); i++ {
		dotProduct += pattern1[i] * pattern2[i]
		mag1 += pattern1[i] * pattern1[i]
		mag2 += pattern2[i] * pattern2[i]
	}

	mag1 = math.Sqrt(mag1)
	mag2 = math.Sqrt(mag2)

	if mag1 == 0 || mag2 == 0 {
		return 0
	}

	return dotProduct / (mag1 * mag2)
}

// calculateOutcome determines what happened after pattern
func calculateOutcome(lastCandle models.Candle, nextCandles []models.Candle) (string, float64) {
	if len(nextCandles) == 0 {
		return "unknown", 0
	}

	startPrice := lastCandle.Close.InexactFloat64()
	endPrice := nextCandles[len(nextCandles)-1].Close.InexactFloat64()

	percentChange := ((endPrice - startPrice) / startPrice) * 100

	outcome := "sideways"
	if percentChange > 1 {
		outcome = "up"
	} else if percentChange < -1 {
		outcome = "down"
	}

	return outcome, percentChange
}

// hashPattern creates hash of pattern for storage/lookup
func hashPattern(pattern []float64) string {
	// Convert to bytes
	data := ""
	for _, val := range pattern {
		data += fmt.Sprintf("%.4f,", val)
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // First 8 bytes
}
