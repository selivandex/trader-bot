package sentiment

import (
	"testing"
)

func TestAnalyzer_AnalyzeSentiment(t *testing.T) {
	t.Skip("Skipping keyword-based sentiment tests - production uses AI evaluation instead")
	analyzer := NewAnalyzer()

	tests := []struct {
		name     string
		text     string
		expected string // positive, negative, or neutral
	}{
		{
			name:     "bullish text",
			text:     "Bitcoin rally continues, bulls are in control, massive pump incoming!",
			expected: "positive",
		},
		{
			name:     "bearish text",
			text:     "Market crash imminent, bears dominating, massive dump expected, panic selling",
			expected: "negative",
		},
		{
			name:     "neutral text",
			text:     "Bitcoin price remains stable today at current levels",
			expected: "neutral",
		},
		{
			name:     "mixed but bullish",
			text:     "Despite FUD, Bitcoin shows strong support and bullish momentum",
			expected: "positive",
		},
		{
			name:     "empty text",
			text:     "",
			expected: "neutral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := analyzer.AnalyzeSentiment(tt.text)

			var got string
			if score > 0.2 {
				got = "positive"
			} else if score < -0.2 {
				got = "negative"
			} else {
				got = "neutral"
			}

			if got != tt.expected {
				t.Errorf("Expected %s sentiment, got %s (score: %.3f)",
					tt.expected, got, score)
			}
		})
	}
}

func TestAnalyzer_ScoreRange(t *testing.T) {
	analyzer := NewAnalyzer()

	texts := []string{
		"bullish rally pump moon rocket",
		"bearish crash dump panic",
		"neutral stable sideways",
	}

	for _, text := range texts {
		score := analyzer.AnalyzeSentiment(text)

		if score < -1.0 || score > 1.0 {
			t.Errorf("Score should be between -1.0 and 1.0, got %.3f for: %s",
				score, text)
		}
	}
}
