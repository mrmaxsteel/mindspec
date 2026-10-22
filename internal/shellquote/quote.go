// Package shellquote quotes literal arguments for POSIX shell commands.
package shellquote

import "strings"

// Quote renders one literal shell argument, including an empty string.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
