package readiness

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// MF-3 failure classes form the stable record vocabulary (spec 129 R6.2).
const (
	FailureIndeterminate  = "indeterminate"
	FailureNotClosed      = "not-closed"
	FailureNoCandidate    = "no-candidate"
	FailureOffFirstParent = "off-first-parent"
	FailureNoEvidence     = "no-evidence"
	FailureContradiction  = "contradiction"
	FailureReverted       = "reverted"
)

// DependencyFailureRecord constructs an MF-3 dep-id:class[:candidate-sha]
// record. Inputs are raw identities, never rendered detail or error text.
func DependencyFailureRecord(depID, class, candidateSHA string) string {
	record := depID + ":" + class
	if candidateSHA != "" {
		record += ":" + candidateSHA
	}
	return record
}

// IsIndeterminateRecord reports whether an MF-3 record names a read failure
// that cannot be covered by an override, regardless of its candidate identity.
func IsIndeterminateRecord(rec string) bool {
	parts := strings.Split(rec, ":")
	return (len(parts) == 2 || len(parts) == 3) && parts[0] != "" && parts[1] == FailureIndeterminate
}

// FailureRecordDigests returns sorted lowercase SHA-256 digests of the raw
// failure records. Override writers and readers share this function; terminal
// escaping and human Detail text never participate in failure identity.
func FailureRecordDigests(s Signal) []string {
	out := make([]string, 0, len(s.Failures))
	for _, record := range s.Failures {
		out = append(out, recordDigest(record))
	}
	sort.Strings(out)
	return out
}

func recordDigest(record string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(record)))
}

func sortedUnique(records []string) []string {
	sort.Strings(records)
	out := records[:0]
	for _, rec := range records {
		if len(out) == 0 || rec != out[len(out)-1] {
			out = append(out, rec)
		}
	}
	return out
}

// occurrenceRecord retains multiplicity among identical blocking lines.
// Whitespace is trimmed and collapsed; all other raw bytes are retained.
func occurrenceRecord(kind, source, section, line string, counts map[string]int) string {
	hash := recordDigest(strings.Join(strings.Fields(line), " "))
	key := kind + ":" + source + ":" + section + ":" + hash
	counts[key]++
	return fmt.Sprintf("%s:%s:%s:%d:%s", kind, source, section, counts[key], hash)
}
