# Core Domain — Interfaces

## Provided Interfaces

### Workspace

```python
class Workspace:
    find_project_root() -> Path
    get_docs_dir() -> Path
    get_glossary_path() -> Path
```

Used by context-system (for glossary location) and workflow (for spec/bead resolution).

### Health Check Report

```python
DocParser.check_health() -> dict
# Returns: docs_dir_exists, glossary_exists, term_count, broken_links, warnings
```

### CLI Command Registration

Other domains register subcommands via the CLI module. Core owns the top-level `mindspec` command group.

## Consumed Interfaces

- **context-system**: Glossary parsing (for broken-link validation in doctor)
- **workflow**: None currently

## Events

None defined yet. Future: health check completion events for observability.
