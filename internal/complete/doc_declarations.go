package complete

import (
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

type docCompletionAudit struct {
	declarations []validate.DocDeclaration
	deferrals    []validate.DocDeferral
}

var docOwnerScopeFn = validateDocOwner

func validateDocOwner(beadID, owner string) error {
	if err := idvalidate.BeadID(owner); err != nil {
		return err
	}
	if beadID == owner {
		return fmt.Errorf("--doc-owner must name another open bead in the same epic")
	}
	info, err := fetchBeadByIDFn(owner)
	if err != nil {
		return err
	}
	if strings.TrimSpace(info.Status) == "" || strings.EqualFold(info.Status, "closed") {
		return fmt.Errorf("--doc-owner %s is not open", idrender.Bead(owner))
	}
	epic, _, err := findEpicForBeadFn(beadID)
	if err != nil {
		return err
	}
	ownerEpic, _, err := findEpicForBeadFn(owner)
	if err != nil {
		return err
	}
	if epic == "" || epic != ownerEpic {
		return fmt.Errorf("--doc-owner %s must belong to the same epic", idrender.Bead(owner))
	}
	return nil
}

func validateDocFlags(beadID string, opts CompleteOpts) error {
	if opts.AllowDocSkew != "" && (strings.TrimSpace(opts.AllowDocSkew) == "" || opts.DocOwner == "") {
		return guard.NewFailure("doc-sync: --allow-doc-skew needs a reason and --doc-owner", completionCommand(beadID, opts)+" --doc-owner <open-bead>")
	}
	if opts.DocOwner != "" {
		if opts.AllowDocSkew == "" {
			return guard.NewFailure("doc-sync: --doc-owner requires --allow-doc-skew", "supply --allow-doc-skew with a reason and an open doc owner")
		}
		if err := docOwnerScopeFn(beadID, opts.DocOwner); err != nil {
			return completionFailure(beadID, opts, "doc-sync owner", err)
		}
	}
	return nil
}

func docSyncHint(beadID string, opts CompleteOpts) string {
	if opts.AllowDocSkew == "" {
		opts.AllowDocSkew = "<reason>"
	}
	if opts.DocOwner == "" {
		opts.DocOwner = "<open-bead>"
	}
	return completionCommand(beadID, opts)
}

// prepareDocDeclarations uses the content gate's attribution, including its
// landing-tree ownership input. It performs no writes, also in check mode.
func prepareDocDeclarations(beadID string, opts CompleteOpts, docs *validate.Result, out io.Writer) (*validate.Result, error) {
	if err := validateDocFlags(beadID, opts); err != nil {
		return docs, err
	}
	exempt := map[string]bool{}
	audit := &docCompletionAudit{}
	now, actor := time.Now().UTC().Format(time.RFC3339), ""
	if !opts.Check && (len(opts.NoDocImpact) > 0 || opts.AllowDocSkew != "") {
		actor = gitUserEmailFn()
	}
	for _, value := range opts.NoDocImpact {
		domain, reason, ok := strings.Cut(value, ":")
		domain, reason = strings.TrimSpace(domain), strings.TrimSpace(reason)
		if !ok || domain == "" || reason == "" {
			return docs, completionFailure(beadID, opts, "doc-sync declaration", fmt.Errorf("--no-doc-impact requires <domain>: <reason> with a non-empty reason"))
		}
		touched, err := docs.DocImpact.TouchesDomain(domain)
		if err != nil {
			return docs, completionFailure(beadID, opts, "doc-sync declaration ownership", err)
		}
		if !touched {
			return docs, completionFailure(beadID, opts, "doc-sync declaration", fmt.Errorf("--no-doc-impact names untouched domain %q", termsafe.Escape(domain)))
		}
		exempt[domain] = true
		audit.declarations = append(audit.declarations, validate.DocDeclaration{Domain: domain, Reason: reason, By: actor, At: now})
		fmt.Fprintf(out, "doc-sync: no doc impact for %s: %s\n", termsafe.Escape(domain), termsafe.Escape(reason))
	}
	filtered := validate.ApplyDocExemptions(docs, exempt)
	meta, err := completeGetMetadataFn(beadID)
	if err != nil {
		return docs, completionFailure(beadID, opts, "doc-sync obligations", err)
	}
	pending, err := validate.ReadDocRecords[validate.DocDeferral](meta, validate.DocDeferralsKey)
	if err != nil {
		return docs, completionFailure(beadID, opts, "doc-sync obligations", err)
	}
	for _, d := range pending {
		if d.ID == "" || d.Source == "" || d.Owner == "" || len(d.Domains) == 0 {
			return docs, completionFailure(beadID, opts, "doc-sync obligations", fmt.Errorf("invalid doc deferral record"))
		}
		if d.Owner != beadID {
			continue
		}
		missing := []string{}
		for _, domain := range d.Domains {
			if docs.DocImpact == nil || !docs.DocImpact.Docs[domain] {
				missing = append(missing, domain)
			}
		}
		if len(missing) == 0 {
			continue
		}
		if opts.AllowDocSkew == "" {
			return filtered, guard.NewFailure(fmt.Sprintf("doc-sync: owner %s must update docs for deferral %s: %s", idrender.Bead(beadID), termsafe.Escape(d.ID), strings.Join(termsafeEscapeEach(missing), ", ")), "update the deferred domain docs, then run "+completionCommand(beadID, opts), docSyncHint(beadID, opts))
		}
		d.Domains = missing
		d.Owner, d.Reason, d.By, d.At = opts.DocOwner, opts.AllowDocSkew, actor, now
		audit.deferrals = append(audit.deferrals, d)
	}
	if opts.AllowDocSkew != "" {
		for _, domain := range filtered.DocImpact.MissingDomains() {
			sum := sha256.Sum256([]byte(beadID + "\x00" + domain))
			audit.deferrals = append(audit.deferrals, validate.DocDeferral{ID: fmt.Sprintf("doc-%x", sum[:8]), Domains: []string{domain}, Source: beadID, Owner: opts.DocOwner, Reason: opts.AllowDocSkew, By: actor, At: now})
		}
		for _, d := range audit.deferrals {
			fmt.Fprintf(out, "doc-sync: deferral %s to %s covers %s\n", termsafe.Escape(d.ID), idrender.Bead(d.Owner), strings.Join(termsafeEscapeEach(d.Domains), ", "))
		}
	}
	if opts.docAudit != nil {
		*opts.docAudit = *audit
	}
	return filtered, nil
}

// recordDocDeclarations runs only after the change lands. Every copy is merged
// by identity, so a retry repairs a partial multi-bead write without duplicates.
func recordDocDeclarations(beadID string, opts CompleteOpts) error {
	if opts.docAudit == nil {
		return nil
	}
	if len(opts.docAudit.declarations) > 0 {
		meta, err := completeGetMetadataFn(beadID)
		if err != nil {
			return err
		}
		rows, err := validate.ReadDocRecords[validate.DocDeclaration](meta, validate.NoDocImpactKey)
		if err != nil {
			return err
		}
		for _, entry := range opts.docAudit.declarations {
			found := false
			for i := range rows {
				if rows[i].Domain == entry.Domain {
					found = true
					if rows[i].Reason != entry.Reason {
						rows[i] = entry
					}
					break
				}
			}
			if !found {
				rows = append(rows, entry)
			}
		}
		if err := completeMergeMetadataFn(beadID, map[string]interface{}{validate.NoDocImpactKey: rows}); err != nil {
			return err
		}
	}
	for _, d := range opts.docAudit.deferrals {
		// Write the closing bead last. Its old owner record drives replay
		// until the source and new owner have the new record.
		ids := []string{d.Owner, d.Source, beadID}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			meta, err := completeGetMetadataFn(id)
			if err != nil {
				return err
			}
			rows, err := validate.ReadDocRecords[validate.DocDeferral](meta, validate.DocDeferralsKey)
			if err != nil {
				return err
			}
			found := false
			for i := range rows {
				if rows[i].ID == d.ID {
					rows[i] = d
					found = true
					break
				}
			}
			if !found {
				rows = append(rows, d)
			}
			if err := completeMergeMetadataFn(id, map[string]interface{}{validate.DocDeferralsKey: rows}); err != nil {
				return err
			}
		}
	}
	return nil
}
