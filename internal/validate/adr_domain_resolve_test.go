package validate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// TestADRDomainResolve_DirectoryNameStaysVerbatim — Rule 1: an entry
// that already names a domain dir is returned unchanged (it is already
// the canonical owner name; no glob match is attempted).
func TestADRDomainResolve_DirectoryNameStaysVerbatim(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"orders"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	a, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"orders"}) {
		t.Errorf("expected Domains [orders] unchanged, got %v", a.Domains)
	}
}

// TestADRDomainResolve_PathResolvesToOwnerBothSlashForms — the AC-5 /
// spec 122 R2 directory-shape completeness core: an ADR-side directory
// label resolves to its owning domain's dir-name identically whether or
// not it carries a trailing slash.
func TestADRDomainResolve_PathResolvesToOwnerBothSlashForms(t *testing.T) {
	cases := []struct {
		name  string
		label string
	}{
		{"with trailing slash", "src/orders/"},
		{"without trailing slash", "src/orders"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
			writeADR(t, root, "ADR-0090", "Accepted", []string{tc.label})

			store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
			a, err := store.Get("ADR-0090")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if !reflect.DeepEqual(a.Domains, []string{"orders"}) {
				t.Errorf("label %q: expected Domains [orders], got %v", tc.label, a.Domains)
			}
		})
	}
}

// TestADRDomainResolve_BareFilePathResolves — a bare file path (not a
// directory label) resolves the same way a directory label does,
// matching the spec's "a bare file path resolves the same way" clause.
func TestADRDomainResolve_BareFilePathResolves(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/orders/api.py"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	a, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"orders"}) {
		t.Errorf("expected Domains [orders], got %v", a.Domains)
	}
}

// TestADRDomainResolve_AmbiguousStaysVerbatim — AC-6-adjacent: when a
// path-shaped entry is claimed by MORE than one domain's OWNERSHIP
// paths:, it is left EXACTLY as authored — no error, no partial
// resolution (the ADR-side no-new-error doctrine: ambiguity is not
// itself a failure here, unlike the spec-side Rule 3 which DOES error
// on ambiguity).
func TestADRDomainResolve_AmbiguousStaysVerbatim(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "alpha", "paths:\n  - src/shared/**\n")
	writeManifest(t, root, "beta", "paths:\n  - src/shared/**\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/shared/"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	a, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"src/shared/"}) {
		t.Errorf("expected ambiguous entry to stay verbatim [src/shared/], got %v", a.Domains)
	}
}

// TestADRDomainResolve_ProseTupleStaysVerbatim — AC-6(i): an Accepted
// ADR's Domain(s) line that is only legacy prose/tuple tokens (not
// path-shaped) is left untouched — the resolver never parses or
// guesses a free-form token. Mirrors this repo's own ADR-0032 line
// (`validation, adr, lifecycle, workflow`), cited in the ADR-0032
// amendment §(b) as the concrete evidence for this doctrine.
func TestADRDomainResolve_ProseTupleStaysVerbatim(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
	writeADR(t, root, "ADR-0031", "Accepted", []string{"validation", "lifecycle"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	a, err := store.Get("ADR-0031")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"validation", "lifecycle"}) {
		t.Errorf("expected prose tokens to stay verbatim, got %v", a.Domains)
	}
}

// TestADRDomainResolve_UnclaimedPathStaysVerbatim — AC-6(ii): an
// ADR-side path claimed by NO domain resolves to nothing (stays
// literal) and produces no resolve-style error on the ADR side; it
// simply compares literally downstream, exactly as before.
func TestADRDomainResolve_UnclaimedPathStaysVerbatim(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/unclaimed/"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	a, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"src/unclaimed/"}) {
		t.Errorf("expected unclaimed path to stay verbatim [src/unclaimed/], got %v", a.Domains)
	}
}

// TestADRDomainResolve_ListAlsoResolves — List() must apply the same
// resolution as Get(), since checkADRCitations/checkADRCoverage read
// through Get(cite.ID) but other consumers (e.g. Bead 3's uncited-
// covering-ADR scan) read through List().
func TestADRDomainResolve_ListAlsoResolves(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/orders"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	list, err := store.List(adr.ListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var found bool
	for _, a := range list {
		if a.ID == "ADR-0090" {
			found = true
			if !reflect.DeepEqual(a.Domains, []string{"orders"}) {
				t.Errorf("expected Domains [orders] via List, got %v", a.Domains)
			}
		}
	}
	if !found {
		t.Fatal("ADR-0090 not found in List() results")
	}
}

// TestADRDomainResolve_MemoizedPerID — the resolution is memoized per
// ADR ID: repeat Get() calls for the same ID resolve the underlying
// path-shaped entry at most once. Demonstrated by mutating the
// manifest between calls — a memoized resolver returns the FIRST
// resolution both times, proving it did not re-resolve.
func TestADRDomainResolve_MemoizedPerID(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/orders"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	first, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get (1st): %v", err)
	}
	if !reflect.DeepEqual(first.Domains, []string{"orders"}) {
		t.Fatalf("expected [orders] on first Get, got %v", first.Domains)
	}

	// Rewrite the manifest so a fresh resolution would now find NO
	// owner (moving the claim to a different domain) — but the
	// decorator's per-ID cache must still return the memoized result.
	writeManifest(t, root, "orders", "paths:\n  - src/nothing/**\n")

	second, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get (2nd): %v", err)
	}
	if !reflect.DeepEqual(second.Domains, []string{"orders"}) {
		t.Errorf("expected memoized [orders] on second Get (per-run cache), got %v", second.Domains)
	}
}

// TestADRDomainResolve_GetDoesNotMutateInnerStore — the decorator
// returns a COPY with Domains replaced; a fresh Get() through the raw
// inner store must still see the ADR's ORIGINAL, unresolved Domains.
// This is the load-bearing guarantee behind the cmd-side adrReadStore
// staying literal: `adr show`/`adr list` build their OWN unwrapped
// store, but if this decorator mutated shared state (e.g. a package-
// level parse cache) that guarantee would silently break.
func TestADRDomainResolve_GetDoesNotMutateInnerStore(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/orders/"})

	inner := adr.NewFileStore(root)
	resolving := newDomainResolvingStore(inner, nil, root, "")

	if _, err := resolving.Get("ADR-0090"); err != nil {
		t.Fatalf("Get via decorator: %v", err)
	}

	raw, err := inner.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get via raw inner store: %v", err)
	}
	if !reflect.DeepEqual(raw.Domains, []string{"src/orders/"}) {
		t.Errorf("expected raw store to still read literal [src/orders/], got %v", raw.Domains)
	}
}

// TestADRDomainResolve_ReadsOwnerRefAnchoredTree — the divergence lane
// passes its own executor.Executor + a ref-anchored ownerRef so
// ADR-side resolution reads the SAME ref-anchored tree the spec side's
// normalizeImpactedDomains call reads (both built from one
// newOwnershipCache per the plan). This drives newDomainResolvingStore
// with a MockExecutor + ownerRef exactly as ValidateDivergence does,
// pinning that the ownerRef parameter is actually threaded through to
// domain enumeration / the ownership cache rather than silently
// falling back to a working-tree read.
func TestADRDomainResolve_ReadsOwnerRefAnchoredTree(t *testing.T) {
	root := t.TempDir()
	// The ADR itself is read from the on-disk root (adr.NewFileStore is
	// not ref-aware) — only domain enumeration + manifest loading route
	// through the mock's ref-anchored methods, mirroring exactly what
	// ValidateDivergence's own store construction feeds this decorator.
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/orders"})

	manifestYAML := []byte("paths:\n  - src/orders/**\n")
	mock := &executor.MockExecutor{
		TreeDirsAtRefResult: []string{"orders"},
		FileAtRefOrAbsentFn: func(ref, path string) ([]byte, bool, error) {
			if strings.Contains(path, "orders") && strings.HasSuffix(path, "OWNERSHIP.yaml") {
				return manifestYAML, true, nil
			}
			return nil, false, nil
		},
	}
	store := newDomainResolvingStore(adr.NewFileStore(root), mock, root, "some-ref")
	a, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"orders"}) {
		t.Errorf("expected Domains [orders] resolved via the ref-anchored tree, got %v", a.Domains)
	}
}

// TestADRDomainResolve_EmptyDomainsUnaffected — an ADR with no
// Domain(s) line at all (nil Domains) resolves to nil/empty without
// panicking, and Get()/List() both handle it.
func TestADRDomainResolve_EmptyDomainsUnaffected(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "ADR-0001", "Accepted", nil)

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	a, err := store.Get("ADR-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(a.Domains) != 0 {
		t.Errorf("expected empty Domains, got %v", a.Domains)
	}
}

// TestADRShowListPathUnwrapped — cmd/mindspec/adr.go's adrReadStore
// (used by `adr show`/`adr list`) is deliberately NOT wrapped in
// newDomainResolvingStore (spec 122 R2's plan-level choice). This
// package-local smoke test asserts the raw adr.FileStore/OverlayStore
// path (the shape adrReadStore actually returns) keeps rendering a
// path-shaped Domain(s) line literally when read WITHOUT the resolving
// decorator — the counterpart proof to
// TestADRDomainResolve_PathResolvesToOwnerBothSlashForms above, which
// shows the SAME fixture resolves once wrapped.
func TestADRShowListPathUnwrapped(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders", "paths:\n  - src/orders/**\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/orders/"})

	// The unwrapped shape: a plain adr.FileStore, exactly what
	// cmd/mindspec/adr.go's adrReadStore builds (modulo the
	// OverlayStore layer it adds only inside a worktree, which does
	// not touch Domains either).
	raw := adr.NewFileStore(root)
	a, err := raw.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"src/orders/"}) {
		t.Errorf("expected literal Domain(s) [src/orders/] via the unwrapped store, got %v", a.Domains)
	}
}

// TestADRDomainResolve_G3_LiteralWinsOverChildProbe pins codex G-3
// (soundness): when the LITERAL token resolves to exactly one owner, the
// synthetic child probe is never consulted — a coincidental child match
// by a DIFFERENT domain must not turn a clean unique literal resolution
// into an ambiguous (verbatim-kept) one. Fixture: domain `file-owner`
// claims the exact file path `src/orders/api.py`; domain `child-owner`
// claims `src/orders/api.py/x` (a synthetic-child collision). The entry
// `src/orders/api.py` has exactly one LITERAL owner (`file-owner`), so it
// must resolve to `file-owner`. RED against the old code, which OR'd the
// literal and probe matches into one owners set — there both file-owner
// (literal) and child-owner (probe) matched, len==2, and the entry stayed
// verbatim.
func TestADRDomainResolve_G3_LiteralWinsOverChildProbe(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "file-owner", "paths:\n  - src/orders/api.py\n")
	writeManifest(t, root, "child-owner", "paths:\n  - src/orders/api.py/x\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/orders/api.py"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	a, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"file-owner"}) {
		t.Errorf("expected literal owner [file-owner] to win over child-probe collision, got %v", a.Domains)
	}
}

// TestADRDomainResolve_G1_ProbeHonorsExclusion pins codex G-1
// (soundness): the directory-completeness fallback probe must honor the
// domain's exclude: against the DECLARED label — a domain that EXCLUDES
// the literal entry must NOT be resurrected by a synthetic child match.
// Fixture: domain `orders` claims `src/orders/**` but EXCLUDES
// `src/orders/config.yaml`; the entry `src/orders/config.yaml` is
// explicitly excluded, so it must stay VERBATIM. RED against the old
// code, where the probe `src/orders/config.yaml/x` matched
// `src/orders/**` (the child is not itself excluded) and wrongly
// resolved the excluded file to `orders`.
func TestADRDomainResolve_G1_ProbeHonorsExclusion(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "orders",
		"paths:\n  - src/orders/**\nexclude:\n  - src/orders/config.yaml\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/orders/config.yaml"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	a, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"src/orders/config.yaml"}) {
		t.Errorf("expected excluded path to stay verbatim [src/orders/config.yaml], got %v", a.Domains)
	}
}

// TestADRDomainResolve_G2_LoadErrorStaysVerbatim pins codex G-2
// (soundness): when ANY enumerated domain's OWNERSHIP.yaml fails to load,
// resolution cardinality is UNKNOWABLE, so the entry stays literal (never
// promoted to a maybe-correct owner). Fixture: domain `orders` has a
// MALFORMED OWNERSHIP.yaml (a YAML syntax error → ownCache.get returns an
// error); domain `payments` cleanly claims `src/shared/**`. The entry
// `src/shared/api.go` matches payments literally, but because orders'
// manifest failed to load the result is indeterminate — it must stay
// VERBATIM, NOT resolve to `payments`. RED against the old code, which
// swallowed the load error (`if err != nil || o == nil { continue }`) and
// promoted the entry to `payments` on the single clean match.
func TestADRDomainResolve_G2_LoadErrorStaysVerbatim(t *testing.T) {
	root := t.TempDir()
	// Malformed YAML: an unterminated flow sequence → yaml.Unmarshal errors.
	writeManifest(t, root, "orders", "paths: [unterminated\n")
	writeManifest(t, root, "payments", "paths:\n  - src/shared/**\n")
	writeADR(t, root, "ADR-0090", "Accepted", []string{"src/shared/api.go"})

	store := newDomainResolvingStore(adr.NewFileStore(root), nil, root, "")
	a, err := store.Get("ADR-0090")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !reflect.DeepEqual(a.Domains, []string{"src/shared/api.go"}) {
		t.Errorf("expected indeterminate (load-error) entry to stay verbatim [src/shared/api.go], got %v", a.Domains)
	}
}
