package panel

import (
	"fmt"
	"strings"
)

// StatusEntry preserves the exact bytes of a porcelain v1 -z record.
// ParseStatusZ accepts only status produced with rename detection disabled.
type StatusEntry struct {
	Index    byte
	Worktree byte
	Path     string
}

func ParseStatusZ(status string) ([]StatusEntry, error) {
	var entries []StatusEntry
	for status != "" {
		record, rest, ok := strings.Cut(status, "\x00")
		if !ok || len(record) < 4 || record[2] != ' ' || strings.ContainsAny(record[:2], "RC") {
			return nil, fmt.Errorf("invalid NUL-separated worktree status record")
		}
		entries = append(entries, StatusEntry{Index: record[0], Worktree: record[1], Path: record[3:]})
		status = rest
	}
	return entries, nil
}
