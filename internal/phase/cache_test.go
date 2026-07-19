package phase

import (
	"errors"
	"strings"
	"testing"
)

// epicsListPayload is a canned bd list --type=epic response covering all three
// statuses, used by the memoization tests below.
const epicsListPayload = `[
  {"id":"epic-1","title":"[SPEC 001-alpha] Alpha","status":"open","issue_type":"epic","metadata":{"spec_num":1,"spec_title":"alpha"}},
  {"id":"epic-2","title":"[SPEC 002-beta] Beta","status":"in_progress","issue_type":"epic","metadata":{"spec_num":2,"spec_title":"beta"}},
  {"id":"epic-3","title":"[SPEC 003-gamma] Gamma","status":"closed","issue_type":"epic","metadata":{"spec_num":3,"spec_title":"gamma"}}
]`

func TestCache_AllEpics_MemoizesAcrossCalls(t *testing.T) {
	calls := 0
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		calls++
		return []byte(epicsListPayload), nil
	})
	defer restore()

	c := NewCache()
	for i := 0; i < 3; i++ {
		if _, err := c.AllEpics(); err != nil {
			t.Fatalf("AllEpics call %d: %v", i, err)
		}
	}
	if calls != 1 {
		t.Errorf("listJSON called %d times, want 1", calls)
	}
}

func TestCache_AllEpics_DoesNotMemoizeError(t *testing.T) {
	calls := 0
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		calls++
		return nil, errors.New("transient bd failure")
	})
	defer restore()

	c := NewCache()
	for i := 0; i < 3; i++ {
		if _, err := c.AllEpics(); err == nil {
			t.Fatalf("AllEpics call %d: expected error", i)
		}
	}
	if calls != 3 {
		t.Errorf("listJSON called %d times, want 3 (errors must not be memoized)", calls)
	}
}

func TestCache_GetChildren_MemoizesPerEpic(t *testing.T) {
	calls := 0
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		calls++
		return []byte(`[{"id":"b1","title":"b","status":"open","issue_type":"task"}]`), nil
	})
	defer restore()

	c := NewCache()
	for i := 0; i < 2; i++ {
		if _, err := c.GetChildren("epic-1"); err != nil {
			t.Fatalf("GetChildren(epic-1) call %d: %v", i, err)
		}
	}
	if _, err := c.GetChildren("epic-2"); err != nil {
		t.Fatalf("GetChildren(epic-2): %v", err)
	}
	if calls != 2 {
		t.Errorf("listJSON called %d times, want 2 (one per distinct epic ID)", calls)
	}
}

// TestCache_GetChildren_IncludesBlockedChild pins R4 (mindspec-7rih): the
// child-fetch must query the full bead.AllStatuses breadth (built-ins
// open/in_progress/blocked/closed + customs) in ONE comma-joined bd list call,
// so a blocked child is not dropped from the cache before
// DerivePhaseFromChildren sees it. The genuinely-RED assertion (against the old
// hardcoded "open,in_progress,closed") is that the captured --status arg
// contains "blocked"; the stub returns the blocked child regardless of breadth,
// so the inclusion check guards the parse/return path.
func TestCache_GetChildren_IncludesBlockedChild(t *testing.T) {
	var capturedStatus string
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if strings.HasPrefix(a, "--status=") {
				capturedStatus = strings.TrimPrefix(a, "--status=")
			}
		}
		return []byte(`[{"id":"b-blocked","title":"blocked child","status":"blocked","issue_type":"task"}]`), nil
	})
	defer restore()

	c := NewCache()
	kids, err := c.GetChildren("epic-1")
	if err != nil {
		t.Fatalf("GetChildren(epic-1): %v", err)
	}

	// The blocked child must survive into the returned set.
	found := false
	for _, k := range kids {
		if k.ID == "b-blocked" {
			found = true
		}
	}
	if !found {
		t.Errorf("GetChildren dropped the blocked child; got %+v", kids)
	}

	// The genuinely-RED assertion: the captured --status arg must cover the
	// AllStatuses breadth (must include blocked), not the legacy
	// open,in_progress,closed.
	if capturedStatus == "" {
		t.Fatalf("no --status= arg captured")
	}
	statuses := strings.Split(capturedStatus, ",")
	hasBlocked := false
	for _, s := range statuses {
		if strings.TrimSpace(s) == "blocked" {
			hasBlocked = true
		}
	}
	if !hasBlocked {
		t.Errorf("--status arg %q does not include blocked (AllStatuses breadth)", capturedStatus)
	}
}

// TestFetchChildren covers the exported uncached wrapper (spec 107 wave 1,
// mindspec-oexu.3). It must (1) issue exactly one comma-joined
// `bd list --parent <epic> --status=<AllStatuses breadth>` call carrying the
// epic ID and the status breadth, and (2) NOT memoize — a second call re-issues
// the query and reflects bd state mutated between calls. This is the freshness
// contract complete.Run's post-close children read depends on (Cache.GetChildren
// is the memoizing counterpart, guarded by TestCache_GetChildren_MemoizesPerEpic).
func TestFetchChildren(t *testing.T) {
	calls := 0
	current := `[{"id":"b1","title":"bead1","status":"in_progress","issue_type":"task"}]`
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		calls++
		var sawEpic, sawStatus bool
		for i, a := range args {
			if a == "--parent" && i+1 < len(args) && args[i+1] == "epic-x" {
				sawEpic = true
			}
			if strings.HasPrefix(a, "--status=") {
				sawStatus = true
				if !strings.Contains(a, "blocked") {
					t.Errorf("--status arg %q must cover the AllStatuses breadth (blocked)", a)
				}
			}
		}
		if !sawEpic {
			t.Errorf("FetchChildren must issue `bd list --parent epic-x`, got args %v", args)
		}
		if !sawStatus {
			t.Errorf("FetchChildren must pass a --status= breadth arg, got args %v", args)
		}
		return []byte(current), nil
	})
	defer restore()

	kids, err := FetchChildren("epic-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kids) != 1 || kids[0].ID != "b1" || kids[0].Status != "in_progress" {
		t.Fatalf("unexpected children: %+v", kids)
	}
	if calls != 1 {
		t.Errorf("FetchChildren must issue exactly one bd list --parent call, got %d", calls)
	}

	// Uncached: after bd state changes, a second call re-reads it fresh.
	current = `[{"id":"b1","title":"bead1","status":"closed","issue_type":"task"}]`
	kids2, err := FetchChildren("epic-x")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if len(kids2) != 1 || kids2[0].Status != "closed" {
		t.Errorf("FetchChildren must re-read fresh (uncached), got %+v", kids2)
	}
	if calls != 2 {
		t.Errorf("second FetchChildren must re-issue the query (uncached), total calls = %d, want 2", calls)
	}
}

func TestCache_FindEpic_MemoizesPerID(t *testing.T) {
	calls := 0
	restore := SetRunBDForTest(func(args ...string) ([]byte, error) {
		calls++
		return []byte(`[{"id":"epic-1","title":"X","status":"open","issue_type":"epic"}]`), nil
	})
	defer restore()

	c := NewCache()
	for i := 0; i < 2; i++ {
		if _, err := c.FindEpic("epic-1"); err != nil {
			t.Fatalf("FindEpic(epic-1) call %d: %v", i, err)
		}
	}
	if _, err := c.FindEpic("epic-2"); err != nil {
		t.Fatalf("FindEpic(epic-2): %v", err)
	}
	if calls != 2 {
		t.Errorf("runBD called %d times, want 2 (one per distinct epic ID)", calls)
	}
}

func TestCache_FindEpicBySpecID_UsesAllEpicsOnce(t *testing.T) {
	calls := 0
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		calls++
		return []byte(epicsListPayload), nil
	})
	defer restore()

	c := NewCache()
	// Existing spec + non-existing spec — both go through cache.AllEpics.
	if id, err := c.FindEpicBySpecID("001-alpha"); err != nil || id != "epic-1" {
		t.Fatalf("FindEpicBySpecID(001-alpha) = (%q, %v); want (epic-1, nil)", id, err)
	}
	if _, err := c.FindEpicBySpecID("999-missing"); err == nil {
		t.Fatalf("FindEpicBySpecID(999-missing) expected not-found error")
	}
	if id, err := c.FindEpicBySpecID("001-alpha"); err != nil || id != "epic-1" {
		t.Fatalf("FindEpicBySpecID(001-alpha) repeat = (%q, %v)", id, err)
	}
	if calls != 1 {
		t.Errorf("listJSON called %d times, want 1", calls)
	}
}

func TestCache_NilReceiver_PassesThrough(t *testing.T) {
	listCalls := 0
	restoreList := SetListJSONForTest(func(args ...string) ([]byte, error) {
		listCalls++
		return []byte(epicsListPayload), nil
	})
	defer restoreList()
	showCalls := 0
	restoreRun := SetRunBDForTest(func(args ...string) ([]byte, error) {
		showCalls++
		return []byte(`[{"id":"epic-1","title":"X","status":"open","issue_type":"epic"}]`), nil
	})
	defer restoreRun()

	var c *Cache
	// Each call must reach the underlying fetch (no memoization on nil receiver).
	for i := 0; i < 2; i++ {
		if _, err := c.AllEpics(); err != nil {
			t.Fatalf("nil AllEpics call %d: %v", i, err)
		}
		if _, err := c.GetChildren("epic-1"); err != nil {
			t.Fatalf("nil GetChildren call %d: %v", i, err)
		}
		if _, err := c.FindEpic("epic-1"); err != nil {
			t.Fatalf("nil FindEpic call %d: %v", i, err)
		}
	}
	if listCalls != 4 {
		t.Errorf("listJSON called %d times, want 4 (no memoization on nil receiver)", listCalls)
	}
	if showCalls != 2 {
		t.Errorf("runBD called %d times, want 2 (no memoization on nil receiver)", showCalls)
	}
}

func TestCache_ActiveEpics_FiltersInProcess(t *testing.T) {
	calls := 0
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		calls++
		return []byte(epicsListPayload), nil
	})
	defer restore()

	c := NewCache()
	active, err := c.ActiveEpics()
	if err != nil {
		t.Fatalf("ActiveEpics: %v", err)
	}
	if calls != 1 {
		t.Errorf("listJSON called %d times, want 1 (ActiveEpics filters in-process)", calls)
	}
	if len(active) != 2 {
		t.Fatalf("ActiveEpics returned %d epics, want 2 (open + in_progress)", len(active))
	}
	for _, e := range active {
		if e.Status == "closed" {
			t.Errorf("ActiveEpics returned closed epic %s", e.ID)
		}
	}
}

func TestCache_FindEpic_NotFoundMemoized(t *testing.T) {
	calls := 0
	restore := SetRunBDForTest(func(args ...string) ([]byte, error) {
		calls++
		return []byte("[]"), nil
	})
	defer restore()

	c := NewCache()
	for i := 0; i < 3; i++ {
		// A well-formed-but-nonexistent id: this test exercises not-found
		// memoization, not idvalidate.BeadID gating (which fetchEpic now
		// applies BEFORE any bd spawn — see TestFetchChildrenEpicIDGate for
		// that property).
		epic, err := c.FindEpic("mindspec-missing")
		if err != nil {
			t.Fatalf("FindEpic(mindspec-missing) call %d: %v", i, err)
		}
		if epic != nil {
			t.Fatalf("FindEpic(mindspec-missing) returned non-nil pointer; want nil sentinel")
		}
	}
	if calls != 1 {
		t.Errorf("runBD called %d times, want 1 (not-found should be memoized)", calls)
	}
}

func TestCache_FindEpicBySpecID_NotFoundMemoized(t *testing.T) {
	calls := 0
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		calls++
		return []byte("[]"), nil
	})
	defer restore()

	c := NewCache()
	for i := 0; i < 3; i++ {
		if _, err := c.FindEpicBySpecID("999-missing"); err == nil {
			t.Fatalf("FindEpicBySpecID(missing) call %d: expected error", i)
		}
	}
	if calls != 1 {
		t.Errorf("listJSON called %d times, want 1 (not-found should be memoized)", calls)
	}
}

// TestCache_CrossAPIMemoization verifies the cache is shared across the
// resolve/phase API surface: ResolveTargetWithCache → ResolveModeWithCache on
// the same cache should issue exactly one `bd list --type=epic` regardless of
// the two cache.FindEpicBySpecID lookups it triggers.
func TestCache_FindEpicBySpecIDWithCache_ShareAcrossCalls(t *testing.T) {
	calls := 0
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		calls++
		return []byte(epicsListPayload), nil
	})
	defer restore()

	c := NewCache()
	if _, err := FindEpicBySpecIDWithCache(c, "001-alpha"); err != nil {
		t.Fatalf("FindEpicBySpecIDWithCache(001-alpha): %v", err)
	}
	if _, err := FindEpicBySpecIDWithCache(c, "002-beta"); err != nil {
		t.Fatalf("FindEpicBySpecIDWithCache(002-beta): %v", err)
	}
	if calls != 1 {
		t.Errorf("listJSON called %d times, want 1 (shared cache should memoize)", calls)
	}
}

// TestFetchChildrenEpicIDGate is spec 120 AC-26 (round-8 epicID gate):
// fetchChildren (via the exported FetchChildren) with a malformed epicID
// records ZERO listJSONFn calls (the existing recording-stub seam,
// derive.go:48) and errors through the existing channel; a well-formed
// epicID (mindspec-s2mf) produces byte-identical argv to today (exactly
// one call).
func TestFetchChildrenEpicIDGate(t *testing.T) {
	hostileEpicIDs := []string{
		"--help",
		"x;evil",
		"x\x00\x1b[31m\nrecovery: forged",
	}

	for _, epicID := range hostileEpicIDs {
		var calls int
		restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
			calls++
			return []byte("[]"), nil
		})
		_, err := FetchChildren(epicID)
		restore()
		if err == nil {
			t.Errorf("FetchChildren(%q) accepted a malformed epic id", epicID)
		}
		if calls != 0 {
			t.Errorf("FetchChildren(%q): expected ZERO listJSONFn calls, got %d", epicID, calls)
		}
	}

	// Well-formed epicID: exactly one call, byte-identical argv to today.
	var gotArgs []string
	var calls int
	restore := SetListJSONForTest(func(args ...string) ([]byte, error) {
		calls++
		gotArgs = args
		return []byte("[]"), nil
	})
	defer restore()
	if _, err := FetchChildren("mindspec-s2mf"); err != nil {
		t.Fatalf("FetchChildren(mindspec-s2mf): %v", err)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 listJSONFn call, got %d", calls)
	}
	if len(gotArgs) < 2 || gotArgs[0] != "--parent" || gotArgs[1] != "mindspec-s2mf" {
		t.Errorf("argv = %v, want [--parent mindspec-s2mf ...]", gotArgs)
	}
}
