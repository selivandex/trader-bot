package toolkit

import (
	"fmt"
	"time"
)

// ============ Shared Utility Functions for Toolkit Package ============

// abs returns absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// minFloat returns minimum of two floats
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// maxFloat returns maximum of two floats
// truncate cuts string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// formatDuration formats duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// truncateAddress shortens blockchain address
func truncateAddress(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}
