# Core Domain — Interfaces

## Provided Interfaces

### Workspace

```go
package workspace

// FindRoot walks up from startDir looking for .mindspec/ or .git.
func FindRoot(startDir string) (string, error)

// DocsDir returns the canonical-or-legacy docs root (no flat tier). Retained
// for consumers not yet migrated to the per-artifact accessors.
func DocsDir(root string) string

// Per-artifact, three-tier flat-first resolvers (spec 106 Req 1):
// flat (.mindspec/<artifact>) → canonical (.mindspec/docs/<artifact>) →
// legacy (docs/<artifact>), first-exists-wins.
func SpecDir(root, specID string) (string, error)
func ADRDir(root string) string
func CoreDir(root string) string
func DomainDir(root, domain string) (string, error)
func ContextMapPath(root string) string
func RecordingDir(root, specID string) (string, error)

// Flat-aware ENUMERATION roots: the parent dirs SpecDir/DomainDir resolve
// an <id>/<domain> under (same three-tier flat-first precedence). For
// filesystem enumerators that list all specs/domains without re-deriving
// the layout. Byte-identical to filepath.Join(DocsDir(root), "specs"|
// "domains") on canonical/legacy/greenfield trees.
func SpecsDir(root string) string
func DomainsDir(root string) string

// TreeRootForSpecDir resolves the checkout tree root from a spec dir in any
// of the flat / canonical / legacy shapes (preserves mindspec-ew79).
func TreeRootForSpecDir(specDir string) string

// Whole-tree layout classification (spec 106 Req 2).
type Layout string // flat | canonical | legacy | greenfield | mixed

// DetectLayout classifies the tree; mixed is a hard error (ErrMixedLayout)
// except under an IN-PROGRESS (non-terminal) .mindspec/migrations/<run-id>/
// run — a completed/"applied" record does not tolerate a mixed tree.
func DetectLayout(root string) (Layout, error)

// ClassifyLayout is the pure layout-signature classifier shared by
// DetectLayout (filesystem) and the cross-layout merge guard (git refs).
type LayoutMarkers struct{ Flat, Canonical, Legacy bool }
func ClassifyLayout(m LayoutMarkers) Layout
func LayoutMarkersFromMindspecChildren(children []string) LayoutMarkers
```

Used by context-system (for glossary location) and workflow (for spec/bead resolution).

### Health Check Report

```go
package doctor

type Status int // OK, Missing, Error, Warn

type Check struct {
    Name    string
    Status  Status
    Message string
}

type Report struct {
    Checks []Check
}

func (r *Report) HasFailures() bool  // true if any Error or Missing
func Run(root string) *Report        // execute all checks
```

### CLI Command Registration

Other domains register subcommands via cobra in `cmd/mindspec/`. Core owns the top-level `mindspec` command group.

## Consumed Interfaces

- **context-system**: Glossary parsing (for broken-link validation in doctor)
- **workflow**: None currently

## Events

None defined yet. Future: health check completion events for observability.
