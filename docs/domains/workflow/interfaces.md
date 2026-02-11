# Workflow Domain — Interfaces

## Provided Interfaces

### Mode State (Planned)

```python
ModeManager.current_mode() -> Mode  # spec | plan | implementation
ModeManager.can_transition(target: Mode) -> bool
ModeManager.transition(target: Mode) -> Result
```

### Spec Lifecycle (Planned)

```python
SpecManager.init(spec_id: str, title: str) -> Path
SpecManager.approve(spec_id: str) -> Result
SpecManager.status(spec_id: str) -> SpecStatus
```

### Worktree Lifecycle (Planned — Spec 005)

```python
WorktreeManager.create(bead_id: str) -> Path
WorktreeManager.list() -> list[Worktree]
WorktreeManager.cleanup(bead_id: str) -> Result
```

### Beads Adapter (Planned — Spec 004)

```python
BeadsAdapter.create_spec_bead(spec_id: str, summary: str) -> BeadId
BeadsAdapter.create_impl_bead(spec_bead: BeadId, scope: str) -> BeadId
BeadsAdapter.close_bead(bead_id: BeadId, evidence: Evidence) -> Result
```

## Consumed Interfaces

- **core**: `Workspace.find_project_root()` for locating specs and beads
- **context-system**: `ContextPackBuilder.build()` for loading mode-appropriate context

## Agent Commands

| Command | Purpose |
|:--------|:--------|
| `/spec-init` | Initialize a new specification |
| `/spec-approve` | Request Spec -> Plan transition |
| `/plan-approve` | Request Plan -> Implementation transition |
| `/spec-status` | Check current mode and state |
