# Context-System Domain — Interfaces

## Provided Interfaces

### Glossary Parsing

```python
DocParser.parse_glossary() -> dict[str, str]
# Returns: {term: target_path_with_anchor}
```

### Glossary Matching (Planned — Spec 002)

```python
GlossaryMatcher.match(text: str) -> list[GlossaryMatch]
# Returns matched terms with their targets
```

### Context Pack Generation (Planned — Spec 003)

```python
ContextPackBuilder.build(spec_id: str, mode: str) -> ContextPack
# Assembles a context pack for the given spec and mode
```

### Section Extraction (Planned)

```python
DocParser.extract_section(file_path: Path, anchor: str) -> str
# Extracts a specific section from a markdown file
```

## Consumed Interfaces

- **core**: `Workspace.find_project_root()`, `Workspace.get_glossary_path()`, `Workspace.get_docs_dir()`
- **workflow**: Spec bead metadata (impacted domains, ADR citations) for context pack routing

## Events

None defined yet. Future: context pack generation events for observability (tokens injected, cache hits).
