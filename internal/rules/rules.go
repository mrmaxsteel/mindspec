// Package rules holds the portable conduct rules used by guidance and panels.
package rules

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed rules.yaml
var table []byte

// Rule is one conduct rule. An absent Gates list applies to every gate.
type Rule struct {
	ID          string   `yaml:"id" json:"id"`
	Scope       []string `yaml:"scope" json:"scope"`
	Prohibition string   `yaml:"prohibition" json:"prohibition"`
	Replacement string   `yaml:"replacement" json:"replacement"`
	EnforcedBy  string   `yaml:"enforced_by" json:"enforced_by"`
	Gates       []string `yaml:"gates,omitempty" json:"gates,omitempty"`
	Target      string   `yaml:"target,omitempty" json:"target,omitempty"`
}

var scopes = []string{"orchestrator", "implementer", "reviewer", "fixer"}
var gates = []string{"spec_approve", "plan_approve", "bead", "final_review", "adhoc"}
var ruleID = regexp.MustCompile(`^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$`)

// Load reads and validates the embedded table. Each call returns independent data.
func Load() ([]Rule, error) {
	return parseTable(table)
}

func parseTable(data []byte) ([]Rule, error) {
	var rules []Rule
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&rules); err != nil {
		return nil, fmt.Errorf("invalid rule table: %w\nrecovery: reinstall mindspec", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("rule table must contain one document\nrecovery: reinstall mindspec")
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("rule table is empty\nrecovery: reinstall mindspec")
	}
	seen := map[string]bool{}
	for _, r := range rules {
		if !ruleID.MatchString(r.ID) || seen[r.ID] || len(r.Scope) == 0 ||
			strings.TrimSpace(r.Prohibition) == "" || strings.TrimSpace(r.Replacement) == "" || strings.TrimSpace(r.EnforcedBy) == "" {
			return nil, fmt.Errorf("invalid or duplicate rule %q\nrecovery: reinstall mindspec", r.ID)
		}
		seen[r.ID] = true
		for _, s := range r.Scope {
			if !slices.Contains(scopes, s) {
				return nil, fmt.Errorf("invalid scope %q in rule %q\nrecovery: reinstall mindspec", s, r.ID)
			}
		}
		for _, g := range r.Gates {
			if !slices.Contains(gates, g) {
				return nil, fmt.Errorf("invalid gate %q in rule %q\nrecovery: reinstall mindspec", g, r.ID)
			}
		}
	}
	return rules, nil
}

// Lookup finds a rule by its exact ID.
func Lookup(id string) (Rule, error) {
	rules, err := Load()
	if err != nil {
		return Rule{}, err
	}
	for _, r := range rules {
		if r.ID == id {
			return r, nil
		}
	}
	return Rule{}, fmt.Errorf("unknown rule %q\nrecovery: run mindspec rules to list rule ids", id)
}

// Filter selects rules in table order. Empty scope or gate means no filter.
// A gate filter includes rules with no Gates restriction.
func Filter(scope, gate string) ([]Rule, error) {
	if scope != "" && !slices.Contains(scopes, scope) {
		return nil, fmt.Errorf("unknown rule scope %q\nrecovery: use --scope with one of %s", scope, strings.Join(scopes, ", "))
	}
	if gate != "" && !slices.Contains(gates, gate) {
		return nil, fmt.Errorf("unknown rule gate %q\nrecovery: use one of %s", gate, strings.Join(gates, ", "))
	}
	rules, err := Load()
	if err != nil {
		return nil, err
	}
	selected := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if (scope == "" || slices.Contains(r.Scope, scope)) &&
			(gate == "" || len(r.Gates) == 0 || slices.Contains(r.Gates, gate)) {
			selected = append(selected, r)
		}
	}
	return selected, nil
}

// StagedMarker records a skill rule's removal date. Err retains malformed
// markers so callers can report them instead of silently ignoring them.
// Lookup checks whether ID names a known rule; parsing permits unknown IDs.
type StagedMarker struct {
	ID       string
	RemoveBy time.Time
	Err      error
}

var stagedComment = regexp.MustCompile(`(?s)<!--\s*staged-rule\s*:(.*?)-->`)

// ParseStagedMarkers reads every staged-rule comment, including invalid ones.
func ParseStagedMarkers(content string) []StagedMarker {
	var markers []StagedMarker
	for _, match := range stagedComment.FindAllStringSubmatch(content, -1) {
		var m StagedMarker
		fields := map[string]string{}
		for _, field := range strings.Split(match[1], ";") {
			key, value, ok := strings.Cut(strings.TrimSpace(field), "=")
			_, duplicate := fields[key]
			if !ok || (key != "id" && key != "remove-by") || duplicate {
				m.Err = fmt.Errorf("invalid staged-rule field\nrecovery: use id and remove-by once in each staged-rule marker")
			}
			fields[key] = strings.TrimSpace(value)
		}
		m.ID = fields["id"]
		date, err := time.Parse("2006-01-02", fields["remove-by"])
		m.RemoveBy = date
		if !ruleID.MatchString(m.ID) || err != nil {
			m.Err = fmt.Errorf("invalid staged-rule id or removal date\nrecovery: set id=<rule-id>; remove-by=YYYY-MM-DD in the marker")
		}
		markers = append(markers, m)
	}
	return markers
}
