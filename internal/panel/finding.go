package panel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Finding is a blocking concrete_changes_required item, never the verdict's
// reviewer-evidence findings array. Text is storage-safe; SHA256 pins RAW text.
type Finding struct {
	ID     string `json:"finding"`
	Text   string `json:"finding_text"`
	SHA256 string `json:"finding_sha256"`
}

// FindingVerdict is the payload needed by finding-aware verbs and pass rules.
type FindingVerdict struct {
	Verdict                 string   `json:"verdict"`
	HardBlock               bool     `json:"hard_block"`
	Rationale               string   `json:"rationale"`
	ConcreteChangesRequired []string `json:"concrete_changes_required"`
}

var localFindingPath = regexp.MustCompile(`(?:/tmp/|/Users/)[^\s<>"'` + "`" + `]*`)
var findingIDPattern = regexp.MustCompile(`^ccr\[(0|[1-9][0-9]*)\]$`)

func validFindingID(id string) bool { return id == "slot" || findingIDPattern.MatchString(id) }

// RedactFindingPaths replaces each machine-local path token before storage.
// Hashing always precedes this replacement. Callers also use this function on
// follow-up bead descriptions before comparing them with stored findings.
func RedactFindingPaths(text string) string { return localFindingPath.ReplaceAllString(text, "<path>") }

func findingHash(text []byte) string {
	sum := sha256.Sum256(text)
	return hex.EncodeToString(sum[:])
}

// Findings implements OQ-4. An empty REQUEST_CHANGES verdict is INVALID rather
// than an empty, vacuously discharged slot. Other recognized verdicts have no
// blocking findings. The input is a single latest-round verdict selected by
// ReadFindingVerdict (or by a caller that already selected the latest round).
func Findings(v FindingVerdict) ([]Finding, error) {
	verdict := strings.ToUpper(strings.TrimSpace(v.Verdict))
	switch verdict {
	case VerdictApprove, VerdictReject:
		return nil, nil
	case VerdictRequestChanges:
	default:
		return nil, fmt.Errorf("INVALID verdict")
	}
	texts := v.ConcreteChangesRequired
	if len(texts) == 0 {
		if strings.TrimSpace(v.Rationale) == "" {
			return nil, fmt.Errorf("INVALID REQUEST_CHANGES: empty list and rationale")
		}
		texts = []string{v.Rationale}
	}
	out := make([]Finding, len(texts))
	for i, text := range texts {
		out[i] = Finding{ID: "ccr[" + strconv.Itoa(i) + "]", Text: RedactFindingPaths(text), SHA256: findingHash([]byte(text))}
	}
	return out, nil
}

// SlotFinding pins the entire ordered list using its compact JSON encoding,
// or the raw rationale when the list is empty. Its storage text is a readable
// list of the redacted findings. A list reorder or edit invalidates this hash.
func SlotFinding(v FindingVerdict) (Finding, error) {
	findings, err := Findings(v)
	if err != nil {
		return Finding{}, err
	}
	if len(findings) == 0 {
		return Finding{}, fmt.Errorf("slot has no REQUEST_CHANGES findings")
	}
	raw := []byte(v.Rationale)
	if len(v.ConcreteChangesRequired) > 0 {
		raw, err = json.Marshal(v.ConcreteChangesRequired)
		if err != nil {
			return Finding{}, err
		}
	}
	texts := make([]string, len(findings))
	for i, f := range findings {
		texts[i] = f.Text
	}
	return Finding{ID: "slot", Text: strings.Join(texts, "\n"), SHA256: findingHash(raw)}, nil
}

// ReadFindingVerdict refuses stale rounds and absent slots, then reads the
// precise latest-round file. An off-round discharge can never be minted here.
func ReadFindingVerdict(dir, slot string, round int) (FindingVerdict, error) {
	if slot == "" || strings.ContainsAny(slot, `/\\`) || slot == "." || slot == ".." {
		return FindingVerdict{}, fmt.Errorf("invalid slot")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return FindingVerdict{}, err
	}
	latest := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := verdictFileRE.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		n, err := strconv.Atoi(match[2])
		if err != nil {
			return FindingVerdict{}, err
		}
		if n > latest {
			latest = n
		}
	}
	if round < 1 || round != latest {
		return FindingVerdict{}, fmt.Errorf("round %d is not latest round %d", round, latest)
	}
	data, err := ReadVerdictFile(filepath.Join(dir, fmt.Sprintf("%s-round-%d.json", slot, round)))
	if err != nil {
		return FindingVerdict{}, err
	}
	var v FindingVerdict
	if err := json.Unmarshal(data, &v); err != nil {
		return FindingVerdict{}, err
	}
	if _, err := Findings(v); err != nil {
		return FindingVerdict{}, err
	}
	return v, nil
}
