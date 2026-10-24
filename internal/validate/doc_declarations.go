package validate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const NoDocImpactKey = "mindspec_no_doc_impact"
const DocDeferralsKey = "mindspec_doc_deferrals"

// DocDeclaration records a domain-specific doc-sync exemption on one bead.
type DocDeclaration struct {
	Domain string `json:"domain"`
	Reason string `json:"reason"`
	By     string `json:"by"`
	At     string `json:"at"`
}

// DocDeferral keeps a stable identity when an owner transfers the obligation.
// The source bead holds the authoritative copy; owners also carry the record.
type DocDeferral struct {
	ID      string   `json:"id"`
	Domains []string `json:"domains"`
	Source  string   `json:"source"`
	Owner   string   `json:"owner"`
	Reason  string   `json:"reason"`
	By      string   `json:"by"`
	At      string   `json:"at"`
}

// ReadDocRecords accepts both decoded tracker metadata and in-process records.
// Malformed metadata fails closed instead of removing a doc obligation.
func ReadDocRecords[T any](meta map[string]interface{}, key string) ([]T, error) {
	if meta[key] == nil {
		return nil, nil
	}
	data, err := json.Marshal(meta[key])
	if err != nil {
		return nil, err
	}
	var records []T
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("invalid %s metadata: %w", key, err)
	}
	return records, nil
}

// DocImpact holds structured attribution from the same tree as the doc gate.
type DocImpact struct {
	Sources      map[string]string
	Docs         map[string]bool
	Touched      map[string]bool
	domainErrors map[int]string
	generalError int
	touchDomain  func(string) (bool, error)
}

func newDocImpact(source, docs []string) *DocImpact {
	d := &DocImpact{Sources: map[string]string{}, Docs: map[string]bool{}, Touched: map[string]bool{}, domainErrors: map[int]string{}, generalError: -1}
	for _, path := range source {
		d.Sources[path] = ""
	}
	for _, path := range docs {
		for _, prefix := range artifactPrefixes("domains") {
			if rest, ok := strings.CutPrefix(path, prefix); ok {
				domain, _, found := strings.Cut(rest, "/")
				if found {
					d.Docs[domain] = true
					d.Touched[domain] = true
				}
			}
		}
	}
	return d
}

// TouchesDomain also attributes non-source paths, such as tests, on demand.
// Normal doc validation needs no extra ownership read for those paths.
func (d *DocImpact) TouchesDomain(domain string) (bool, error) {
	if d == nil {
		return false, nil
	}
	if d.Touched[domain] {
		return true, nil
	}
	if d.touchDomain == nil {
		return false, nil
	}
	return d.touchDomain(domain)
}

// MissingDomains returns each domain with a blocking documentation mismatch.
func (d *DocImpact) MissingDomains() []string {
	set := map[string]bool{}
	if d != nil {
		for _, domain := range d.domainErrors {
			set[domain] = true
		}
	}
	return SortedDocDomains(set)
}

func SortedDocDomains(set map[string]bool) []string {
	var domains []string
	for domain, present := range set {
		if present {
			domains = append(domains, domain)
		}
	}
	sort.Strings(domains)
	return domains
}

// ApplyDocExemptions removes only errors for the named domains. Infrastructure
// and spec-artifact errors remain blocking. Unowned source is never exempt.
func ApplyDocExemptions(r *Result, exemptions map[string]bool) *Result {
	if r.DocImpact == nil || len(exemptions) == 0 {
		return r
	}
	copy := *r
	copy.Issues = nil
	impact := *r.DocImpact
	impact.domainErrors = map[int]string{}
	impact.generalError = -1
	copy.DocImpact = &impact
	allExempt := len(r.DocImpact.Sources) > 0
	for _, domain := range r.DocImpact.Sources {
		if domain == "" || !exemptions[domain] {
			allExempt = false
		}
	}
	for i, issue := range r.Issues {
		if domain := r.DocImpact.domainErrors[i]; domain != "" && exemptions[domain] {
			continue
		}
		if i == r.DocImpact.generalError && allExempt {
			continue
		}
		if domain := r.DocImpact.domainErrors[i]; domain != "" {
			impact.domainErrors[len(copy.Issues)] = domain
		}
		if i == r.DocImpact.generalError {
			impact.generalError = len(copy.Issues)
		}
		copy.Issues = append(copy.Issues, issue)
	}
	return &copy
}
