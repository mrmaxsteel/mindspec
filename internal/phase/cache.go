package phase

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Cache memoizes bd subprocess results within a single CLI invocation.
//
// Lifetime: ONE Cache MUST be created per cobra RunE (or top-level entrypoint)
// and threaded through the resolve / phase / instruct / state call stack. Never
// store a Cache in a package-level variable — bd state can be mutated by other
// processes and the cache has no invalidation hook.
//
// Safety: methods are safe for sequential use; the embedded sync.Mutex allows
// concurrent reads from goroutines if the CLI ever fans out work.
//
// Error policy: errors are NOT memoized. A transient bd failure on one call
// inside an invocation does not poison subsequent attempts. "Not found"
// (successful query, zero matches) IS memoized as a sentinel — a present
// map entry with an empty/nil value.
//
// Nil receivers: every method accepts a nil *Cache and falls back to a direct
// bd call without memoization. This lets call sites be cache-aware without
// nil checks; the legacy non-cache wrappers also exploit this.
type Cache struct {
	mu sync.Mutex

	// Whole-list memoization.
	epicsAll       []EpicInfo
	epicsAllLoaded bool

	// Per-key memoization. Presence of the map key means "looked up successfully".
	// Errors are never stored.
	epicByID   map[string]*EpicInfo   // epicID → bd show result; nil pointer = "looked up, not found"
	children   map[string][]ChildInfo // epicID → fetchChildren result
	specToEpic map[string]string      // specID → epicID; "" = looked up, not found
}

// NewCache constructs an empty Cache. Callers should create one per cobra RunE
// invocation and thread it through the resolve/phase/instruct/state stack.
func NewCache() *Cache {
	return &Cache{
		epicByID:   make(map[string]*EpicInfo),
		children:   make(map[string][]ChildInfo),
		specToEpic: make(map[string]string),
	}
}

// AllEpics returns every epic (open + in_progress + closed) from bd in a single
// `bd list --type=epic --status=open,in_progress,closed -n 0` call. On error,
// nothing is memoized so the next call retries.
func (c *Cache) AllEpics() ([]EpicInfo, error) {
	if c == nil {
		return fetchAllEpics()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.epicsAllLoaded {
		return c.epicsAll, nil
	}
	epics, err := fetchAllEpics()
	if err != nil {
		return nil, err
	}
	c.epicsAll = epics
	c.epicsAllLoaded = true
	return epics, nil
}

// ActiveEpics filters AllEpics to open + in_progress in-process. No extra bd call.
func (c *Cache) ActiveEpics() ([]EpicInfo, error) {
	epics, err := c.AllEpics()
	if err != nil {
		return nil, err
	}
	out := make([]EpicInfo, 0, len(epics))
	for _, e := range epics {
		switch strings.ToLower(strings.TrimSpace(e.Status)) {
		case "open", "in_progress":
			out = append(out, e)
		}
	}
	return out, nil
}

// FindEpic returns the EpicInfo for an exact epic ID, fetching via `bd show`
// once per ID per cache. Returns (nil, nil) if successfully looked up but not
// found. On bd error, returns the error and does NOT memoize.
//
// This is the unification point for the three call sites that today each issue
// their own `bd show <epicID> --json`: readStoredPhase, hasDoneMarker,
// queryEpicStatus. Cache-aware variants of those helpers call FindEpic and
// read the relevant field off the returned *EpicInfo instead of re-fetching.
func (c *Cache) FindEpic(epicID string) (*EpicInfo, error) {
	if c == nil {
		return fetchEpic(epicID)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.epicByID[epicID]; ok {
		return e, nil
	}
	epic, err := fetchEpic(epicID)
	if err != nil {
		return nil, err
	}
	c.epicByID[epicID] = epic // nil pointer memoizes "looked up, not found"
	return epic, nil
}

// GetChildren returns all children (any status) of an epic via a single
// `bd list --parent <epicID> --status=<bead.AllStatuses> -n 0` call,
// memoized per epicID. The status set covers built-ins (open, in_progress,
// blocked, closed) plus project custom statuses, matching advanceState.
// Callers that only want a subset (e.g. in_progress for
// FindActiveBeadForEpic) filter the returned slice in-process.
func (c *Cache) GetChildren(epicID string) ([]ChildInfo, error) {
	if c == nil {
		return fetchChildren(epicID)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if kids, ok := c.children[epicID]; ok {
		return kids, nil
	}
	kids, err := fetchChildren(epicID)
	if err != nil {
		return nil, err
	}
	c.children[epicID] = kids
	return kids, nil
}

// FindEpicBySpecID resolves a spec ID to its epic ID using AllEpics + the
// existing ExtractSpecMetadata/SpecIDFromMetadata logic. Memoizes the mapping.
func (c *Cache) FindEpicBySpecID(specID string) (string, error) {
	if c != nil {
		c.mu.Lock()
		if eid, ok := c.specToEpic[specID]; ok {
			c.mu.Unlock()
			if eid == "" {
				return "", fmt.Errorf("no epic found for spec %s", specID)
			}
			return eid, nil
		}
		c.mu.Unlock()
	}
	epics, err := c.AllEpics()
	if err != nil {
		return "", err
	}
	for _, epic := range epics {
		num, title := ExtractSpecMetadata(epic)
		if num > 0 && title != "" && SpecIDFromMetadata(num, title) == specID {
			if c != nil {
				c.mu.Lock()
				c.specToEpic[specID] = epic.ID
				c.mu.Unlock()
			}
			return epic.ID, nil
		}
	}
	if c != nil {
		c.mu.Lock()
		c.specToEpic[specID] = "" // memoize the not-found
		c.mu.Unlock()
	}
	return "", fmt.Errorf("no epic found for spec %s", specID)
}

// FetchChildren returns all children (any status) of an epic via a single
// uncached `bd list --parent <epicID> --status=<AllStatuses> -n 0` call — the
// exact query Cache.GetChildren memoizes, but WITHOUT memoization so a caller
// that must observe bd state mutated mid-invocation always reads fresh. This is
// the exported seam complete.Run uses for its post-close children read (the
// state advance runs after `bd close` mutates the child set, so a memoized read
// would be stale). Callers that want per-invocation memoization must use
// Cache.GetChildren instead.
func FetchChildren(epicID string) ([]ChildInfo, error) {
	return fetchChildren(epicID)
}

// --- Package-private bd-touching helpers ---
//
// These are the single source of truth for bd invocations used by the cache
// and the wrapper functions. They go through listJSONFn / runBDFn so test
// stubs (SetListJSONForTest / SetRunBDForTest) continue to work.

// fetchAllEpics issues a single `bd list --type=epic --status=open,in_progress,closed -n 0 --json`.
// The status list is pinned explicitly because `bd list` defaults to open only
// and `-n 0` is required to avoid the default --limit=50 silently truncating.
func fetchAllEpics() ([]EpicInfo, error) {
	out, err := listJSONFn("--type=epic", "--status=open,in_progress,closed", "-n", "0")
	if err != nil {
		return nil, fmt.Errorf("bd list --type=epic failed: %w", err)
	}
	var epics []EpicInfo
	if err := json.Unmarshal(out, &epics); err != nil {
		return nil, fmt.Errorf("parse epics: %w", err)
	}
	return epics, nil
}

// fetchChildren issues a single `bd list --parent <id> --status=<AllStatuses> -n 0 --json`.
//
// The status set is the full bead.AllStatuses breadth — built-ins
// (open, in_progress, blocked, closed) plus every project custom status —
// matching the advanceState/queryAllChildren view in complete.go, so a
// `blocked` or custom-status child is not silently dropped from the cache
// before DerivePhaseFromChildren sees it (ADR-0023 counts blocked children).
//
// CRITICAL: this stays a SINGLE comma-joined `--status=` call (not one bd call
// per status) to preserve the GetChildren single-call contract and keep
// TestCache_GetChildren_MemoizesPerEpic green (exactly one listJSON call per
// epic). The root is resolved from cwd here without changing the public
// GetChildren(epicID) signature; if root resolution fails it degrades to ""
// and bead.AllStatuses("") still returns the built-ins (which include blocked).
func fetchChildren(epicID string) ([]ChildInfo, error) {
	root := ""
	if cwd, err := os.Getwd(); err == nil {
		if r, err := workspace.FindRoot(cwd); err == nil {
			root = r
		}
	}
	statusArg := "--status=" + strings.Join(bead.AllStatuses(root), ",")
	out, err := listJSONFn("--parent", epicID, statusArg, "-n", "0")
	if err != nil {
		return nil, fmt.Errorf("bd list --parent failed: %w", err)
	}
	var children []ChildInfo
	if err := json.Unmarshal(out, &children); err != nil {
		return nil, fmt.Errorf("parse children: %w", err)
	}
	return children, nil
}

// fetchEpic issues a `bd show <id> --json`. Returns (nil, nil) for "not found".
func fetchEpic(epicID string) (*EpicInfo, error) {
	out, err := runBDFn("show", epicID, "--json")
	if err != nil {
		return nil, err
	}
	var items []EpicInfo
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}
