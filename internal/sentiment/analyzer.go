package sentiment

import (
	"strings"
)

// Analyzer performs simple keyword-based sentiment analysis
type Analyzer struct {
	positiveWords map[string]float64
	negativeWords map[string]float64
}

// NewAnalyzer creates new sentiment analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		positiveWords: buildPositiveWords(),
		negativeWords: buildNegativeWords(),
	}
}

// AnalyzeSentiment analyzes text and returns sentiment score (-1.0 to 1.0)
func (a *Analyzer) AnalyzeSentiment(text string) float64 {
	if text == "" {
		return 0.0
	}

	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return 0.0
	}

	var score float64
	matchCount := 0

	for _, word := range words {
		// Clean punctuation
		word = strings.Trim(word, ".,!?;:")

		if weight, ok := a.positiveWords[word]; ok {
			score += weight
			matchCount++
		}

		if weight, ok := a.negativeWords[word]; ok {
			score -= weight
			matchCount++
		}
	}

	if matchCount == 0 {
		return 0.0
	}

	// Normalize score
	normalizedScore := score / float64(len(words))

	// Clamp to -1.0 to 1.0
	if normalizedScore > 1.0 {
		normalizedScore = 1.0
	} else if normalizedScore < -1.0 {
		normalizedScore = -1.0
	}

	return normalizedScore
}

// buildPositiveWords returns positive keywords for crypto
func buildPositiveWords() map[string]float64 {
	return map[string]float64{
		// General positive
		"bullish":      1.0,
		"bull":         0.9,
		"rally":        0.9,
		"surge":        0.8,
		"soar":         0.8,
		"pump":         0.7,
		"moon":         0.7,
		"rocket":       0.7,
		"gain":         0.6,
		"profit":       0.6,
		"win":          0.6,
		"green":        0.6,
		"up":           0.5,
		"rise":         0.5,
		"grow":         0.5,
		"growth":       0.5,
		"increase":     0.5,
		"positive":     0.5,
		"optimistic":   0.5,
		"breakthrough": 0.6,
		"adoption":     0.6,
		"partnership":  0.5,
		"upgrade":      0.5,
		"innovation":   0.5,

		// Crypto specific
		"halving":       0.6,
		"breakout":      0.7,
		"ath":           0.8, // all-time high
		"institutional": 0.5,
		"etf":           0.7,
		"approved":      0.6,
		"accumulation":  0.5,
	}
}

// buildNegativeWords returns negative keywords for crypto
func buildNegativeWords() map[string]float64 {
	return map[string]float64{
		// General negative
		"bearish":     1.0,
		"bear":        0.9,
		"crash":       1.0,
		"dump":        0.9,
		"plunge":      0.8,
		"fall":        0.6,
		"drop":        0.6,
		"decline":     0.6,
		"loss":        0.7,
		"red":         0.6,
		"down":        0.5,
		"negative":    0.5,
		"pessimistic": 0.5,
		"fear":        0.6,
		"panic":       0.8,
		"sell":        0.5,
		"selloff":     0.7,
		"correction":  0.6,

		// Crypto specific
		"hack":         1.0,
		"exploit":      1.0,
		"scam":         1.0,
		"rug":          1.0,
		"ponzi":        1.0,
		"fraud":        1.0,
		"lawsuit":      0.7,
		"sec":          0.6, // (when negative context)
		"ban":          0.8,
		"regulation":   0.5,
		"crackdown":    0.7,
		"liquidation":  0.8,
		"capitulation": 0.8,
		"fud":          0.7, // fear, uncertainty, doubt
		"bubble":       0.6,
		"overvalued":   0.6,
	}
}
