package approve

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func implDocMetadata(id string) (map[string]interface{}, error) {
	if err := idvalidate.BeadID(id); err != nil {
		return nil, err
	}
	out, err := implRunBDFn("show", id, "--json")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("cannot read doc records for %s", idrender.Bead(id))
	}
	return rows[0].Metadata, nil
}

// implDocGate checks each source-changing first-parent step. A direct spec
// commit cannot borrow a declaring bead's exemption, even for the same path.
func implDocGate(root, base, branch, specID string, exec executor.Executor, ids []string, docs *validate.Result, override string) ([]validate.DocDeferral, error) {
	declarations := map[string]map[string]bool{}
	var deferrals []validate.DocDeferral
	for _, id := range ids {
		meta, err := implDocMetadata(id)
		if err != nil {
			return nil, fmt.Errorf("doc-sync: %w", err)
		}
		rows, err := validate.ReadDocRecords[validate.DocDeclaration](meta, validate.NoDocImpactKey)
		if err != nil {
			return nil, fmt.Errorf("doc-sync: %w", err)
		}
		declarations[id] = map[string]bool{}
		for _, row := range rows {
			if row.Domain != "" && strings.TrimSpace(row.Reason) != "" && row.By != "" && row.At != "" {
				declarations[id][row.Domain] = true
			}
		}
		deferred, err := validate.ReadDocRecords[validate.DocDeferral](meta, validate.DocDeferralsKey)
		if err != nil {
			return nil, fmt.Errorf("doc-sync: %w", err)
		}
		for _, d := range deferred {
			if d.ID == "" || d.Source == "" || d.Owner == "" || len(d.Domains) == 0 {
				return nil, fmt.Errorf("doc-sync: invalid doc deferral record on %s", idrender.Bead(id))
			}
			if d.Source == id {
				deferrals = append(deferrals, d)
			}
		}
	}
	// Owners can be follow-up beads. Their landed doc change still counts
	// when they are outside the set that carries source audit records.
	for _, d := range deferrals {
		if declarations[d.Owner] == nil {
			declarations[d.Owner] = map[string]bool{}
		}
	}
	exempt, seen := map[string]bool{}, map[string]bool{}
	landedDocs := map[string]map[string]bool{}
	if len(declarations) > 0 || len(deferrals) > 0 {
		state, err := exec.CompletionState(root, base, branch)
		if err != nil {
			return nil, fmt.Errorf("doc-sync: reading landed changes: %w", err)
		}
		for _, commit := range state.NewCommits {
			if len(commit.Parents) == 0 {
				return nil, fmt.Errorf("doc-sync: landed change has no parent")
			}
			result := validate.ValidateDocsRange(root, commit.Parents[0], commit.SHA, branch, exec)
			// Attribution errors are not an exemption. The branch gate remains the
			// authority for all other doc-sync lanes.
			if result.DocImpact == nil {
				continue
			}
			id := declaringMergeBead(commit, declarations)
			if id != "" {
				landedDocs[id] = result.DocImpact.Docs
			}
			for _, domain := range result.DocImpact.Sources {
				if domain == "" {
					continue
				}
				if !seen[domain] {
					exempt[domain] = true
					seen[domain] = true
				}
				if id == "" || !declarations[id][domain] {
					exempt[domain] = false
				}
			}
		}
	}
	docs = validate.ApplyDocExemptions(docs, exempt)
	var open []validate.DocDeferral
	var lines []string
	for _, d := range deferrals {
		resolved := true
		for _, domain := range d.Domains {
			if !landedDocs[d.Owner][domain] {
				resolved = false
			}
		}
		if !resolved {
			open = append(open, d)
			lines = append(lines, fmt.Sprintf("%s (owner %s; domains %s)", termsafe.Escape(d.ID), idrender.Bead(d.Owner), strings.Join(escapeDocDomains(d.Domains), ", ")))
		}
	}
	if override == "" && (docs.HasFailures() || len(open) > 0) {
		message := "doc-sync: " + joinResultErrorMessages(docs)
		if len(lines) > 0 {
			message += "\nopen doc deferrals:\n" + strings.Join(lines, "\n")
		}
		return open, guard.NewFailure(message, "update the affected docs and complete their owner beads, then run mindspec impl approve "+idrender.Spec(specID), "mindspec impl approve "+idrender.Spec(specID)+" --allow-doc-skew \"<reason>\"")
	}
	return open, nil
}

func declaringMergeBead(commit executor.MergeIdentity, known map[string]map[string]bool) string {
	if len(commit.Parents) != 2 {
		return ""
	}
	start := strings.Index(commit.Subject, workspace.BeadBranchPrefix)
	if start < 0 {
		return ""
	}
	tokens := strings.FieldsFunc(commit.Subject[start:], func(r rune) bool { return r < 0x20 || strings.ContainsRune(" '\x22\x60])>,;", r) })
	if len(tokens) == 0 {
		return ""
	}
	for id := range known {
		branch, err := workspace.BeadBranch(id)
		if err == nil && tokens[0] == branch {
			return id
		}
	}
	return ""
}

func escapeDocDomains(domains []string) []string {
	result := make([]string, len(domains))
	for i, domain := range domains {
		result[i] = termsafe.Escape(domain)
	}
	return result
}
