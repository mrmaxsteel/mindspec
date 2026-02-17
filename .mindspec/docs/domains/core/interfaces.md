# Core Domain — Interfaces

## Provided Interfaces

### Workspace

```go
package workspace

// FindRoot walks up from startDir looking for .mindspec/ or .git.
func FindRoot(startDir string) (string, error)

// DocsDir returns the docs directory path under root.
func DocsDir(root string) string

// GlossaryPath returns the GLOSSARY.md path under root.
func GlossaryPath(root string) string
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
