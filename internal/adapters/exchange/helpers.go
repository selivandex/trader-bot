package exchange

// Helper functions used by exchange adapters

func safeFloat(ptr *float64) float64 {
	if ptr != nil {
		return *ptr
	}
	return 0
}

func safeInt64(ptr *int64) int64 {
	if ptr != nil {
		return *ptr
	}
	return 0
}

func safeStringPtr(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
