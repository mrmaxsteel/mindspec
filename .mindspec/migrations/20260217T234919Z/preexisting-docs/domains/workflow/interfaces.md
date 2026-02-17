# Workflow Domain — Interfaces

## Provided Interfaces

### State Management (Spec 004)

```go
package state

// State represents the MindSpec workflow state at .mindspec/state.json.
type State struct {
    Mode        string // idle | spec | plan | implement
    ActiveSpec  string // e.g. "004-instruct"
    ActiveBead  string // e.g. "beads-001"
    LastUpdated string // RFC3339 timestamp
}

// Read loads state from .mindspec/state.json.
func Read(root string) (*State, error)

// Write persists state to .mindspec/state.json.
func Write(root string, s *State) error

// SetMode validates and writes a new state.
func SetMode(root, mode, spec, bead string) error

// CrossValidate checks state against artifact state, returns warnings.
func CrossValidate(root string, s *State) []Warning
```

### Guidance Emission (Spec 004)

```go
package instruct

// BuildContext creates a rendering context from state and project root.
func BuildContext(root string, s *state.State) *Context

// Render produces markdown guidance for the given context.
func Render(ctx *Context) (string, error)

// RenderJSON produces structured JSON output.
func RenderJSON(ctx *Context) (string, error)

// CheckWorktree verifies the current worktree matches the active bead.
func CheckWorktree(activeBead string) string
```

### Worktree Lifecycle (Planned — Spec 008)

```go
// WorktreeManager (planned)
// Create(beadID string) (string, error)
// List() ([]Worktree, error)
// Cleanup(beadID string) error
```

### Beads Adapter (Planned — Spec 007)

```go
// BeadsAdapter (planned)
// CreateSpecBead(specID, summary string) (string, error)
// CreateImplBead(specBead, scope string) (string, error)
// CloseBead(beadID string, evidence Evidence) error
```

## Consumed Interfaces

- **core**: `workspace.FindRoot()`, `workspace.StatePath()`, `workspace.MindspecDir()` for locating state and specs
- **context-system**: `ContextPackBuilder.Build()` for loading mode-appropriate context

## CLI Commands

| Command | Purpose |
|:--------|:--------|
| `mindspec state set` | Set current mode and active work (ADR-0005) |
| `mindspec state show` | Display current state |
| `mindspec instruct` | Emit mode-appropriate operating guidance (ADR-0003) |

## Agent Commands

| Command | Purpose |
|:--------|:--------|
| `/spec-init` | Initialize a new specification |
| `/spec-approve` | Request Spec -> Plan transition |
| `/plan-approve` | Request Plan -> Implementation transition |
| `/spec-status` | Check current mode and state |
