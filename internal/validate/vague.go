package validate

import "strings"

// vaguePatterns is a blocklist of phrases that indicate vague acceptance criteria.
var vaguePatterns = []string{
	"works correctly",
	"is fast",
	"properly handles",
	"is good",
	"functions as expected",
	"works as expected",
	"is reliable",
	"is efficient",
	"performs well",
	"handles correctly",
}

// IsVagueCriterion returns true if the text contains a vague phrase.
func IsVagueCriterion(text string) bool {
	lower := strings.ToLower(text)
	for _, pattern := range vaguePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
