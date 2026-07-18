package complete

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

// beadScopeGetMetadataFn is bead_scope's OWN seam onto bd metadata reads
// (Spec 119 R11 / mindspec-jli8's advisory leg). Kept separate from
// completeGetMetadataFn (the Spec 114 durable-obligation seam) so a test
// pinning call ORDER/COUNT on the obligation-reconciliation path (step
// 3.75) is never perturbed by this independent, best-effort advisory
// check — the two seams default to the same underlying bead.GetMetadata
// but are swapped independently.
var beadScopeGetMetadataFn = bead.GetMetadata

// beadScopeChangedFilesFn is bead_scope's own seam onto the executor's
// diff read, mirroring the doc-sync/ADR-divergence gates' direct
// exec.ChangedFiles use (ADR-0030: git facts flow through the executor).
// A distinct package var (not reusing exec.ChangedFiles inline) keeps
// this file's single external dependency point obvious and swappable in
// isolation from MockExecutor's other configured behaviors.
var beadScopeChangedFilesFn = func(exec executor.Executor, base, head string) ([]string, error) {
	return exec.ChangedFiles(base, head)
}

// beadScopeWarnAdvisory is the Spec 119 R11 (mindspec-jli8) advisory
// per-bead scope check: emits a NON-FATAL WARN — the exit code is never
// affected — naming a file the bead actually changed that is owned by a
// domain OTHER than the bead's DECLARED domain set, while that file's
// domain is still one of the spec's impacted domains. A bead's declared
// domain set is derived from its own PLANNED scope: the `file_paths`
// bead-metadata Bead 4 populates from a plan's structured
// `work_chunks[].key_file_paths` (spec 097 R4), attributed to the
// domain(s) that own those planned files.
//
// This is deliberately advisory-only (never a gate, never blocking):
// legitimate seams routinely cross a domain boundary atomically — this
// spec's OWN plan puts Bead 3's `internal/phase` classifier export
// (core) inside a bead whose bulk of work is workflow/execution. The
// WARN surfaces sprawl for a human/panel to judge; it never refuses.
//
// Silent no-op (prints nothing) on any of:
//   - the bead carries no `file_paths` metadata at all (no declared
//     baseline to compare against — a plan without structured
//     `work_chunks`, or a bead created outside `plan approve`, carries
//     none; NOT an error condition);
//   - none of the declared file_paths attribute to a known domain (no
//     baseline domain to compare against);
//   - the spec's impacted-domain resolution fails or yields nothing
//     (best-effort: the gates this rides alongside already surface a
//     genuine spec.md/plan.md problem elsewhere; this check never
//     duplicates that as a second failure mode);
//   - the changed-file read or its attribution fails.
//
// out receives one `WARN bead-scope: ...` line per cross-domain file.
// The file path and every domain name are rendered through
// internal/termsafe (spec 116 / R11) — both flow from data this process
// does not fully control (a changed path from the diff; a domain name
// from OWNERSHIP.yaml/spec.md content) — so both are defensively escaped
// before reaching terminal-facing output.
func beadScopeWarnAdvisory(exec executor.Executor, root, specDir, beadID, ownerRef, base, head string, out io.Writer) {
	meta, err := beadScopeGetMetadataFn(beadID)
	if err != nil || meta == nil {
		return
	}
	declaredPaths := stringSliceFromMetadataValue(meta["file_paths"])
	if len(declaredPaths) == 0 {
		return
	}

	candidateDomains, err := validate.ResolveCandidateDomains(exec, root, specDir, ownerRef)
	if err != nil || len(candidateDomains) == 0 {
		return
	}

	declaredAttr, err := validate.AttributeChangedFileDomains(exec, root, ownerRef, declaredPaths, candidateDomains)
	if err != nil || len(declaredAttr) == 0 {
		// No declared file attributes to any candidate domain — no
		// baseline to compare the actual diff against.
		return
	}
	declaredDomains := make(map[string]bool, len(declaredAttr))
	for _, d := range declaredAttr {
		declaredDomains[d] = true
	}

	changed, err := beadScopeChangedFilesFn(exec, base, head)
	if err != nil || len(changed) == 0 {
		return
	}
	changedAttr, err := validate.AttributeChangedFileDomains(exec, root, ownerRef, changed, candidateDomains)
	if err != nil || len(changedAttr) == 0 {
		return
	}

	declaredList := make([]string, 0, len(declaredDomains))
	for d := range declaredDomains {
		declaredList = append(declaredList, d)
	}
	sort.Strings(declaredList)
	declaredJoined := strings.Join(declaredList, ", ")

	var crossDomainPaths []string
	for p, domain := range changedAttr {
		if declaredDomains[domain] {
			continue
		}
		crossDomainPaths = append(crossDomainPaths, p)
	}
	sort.Strings(crossDomainPaths)
	for _, p := range crossDomainPaths {
		domain := changedAttr[p]
		fmt.Fprintf(out, "WARN bead-scope: %s is owned by domain %s, outside bead %s's declared domain(s) [%s]\n",
			termsafe.Escape(p), termsafe.Escape(domain), termsafe.Escape(beadID), termsafe.Escape(declaredJoined))
	}
}

// stringSliceFromMetadataValue coerces a bd-metadata JSON value back into
// a []string. bd persists metadata as JSON, so a `file_paths` written as
// a Go []string round-trips through GetMetadata's json.Unmarshal into
// map[string]interface{} as []interface{} of strings — this normalizes
// either representation (and tolerates a hand-authored []string in
// tests) into a plain []string, dropping any non-string/empty element.
func stringSliceFromMetadataValue(v interface{}) []string {
	switch vv := v.(type) {
	case []string:
		return vv
	case []interface{}:
		out := make([]string, 0, len(vv))
		for _, e := range vv {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
