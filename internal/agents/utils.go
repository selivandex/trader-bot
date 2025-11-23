package agents

// ============ Shared Utility Functions ============

// truncate cuts string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// min returns minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minFloat returns minimum of two floats
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
