package trace

// EstimateTokens returns a rough token count for the given string.
// Uses bytes/4 as a Claude-family approximation.
func EstimateTokens(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4 // round up
}

// EstimateTokensBytes returns a rough token count for raw bytes.
func EstimateTokensBytes(b []byte) int {
	n := len(b)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}
