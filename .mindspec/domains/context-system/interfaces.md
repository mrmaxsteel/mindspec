# Context-System Domain — Interfaces

## Provided Interfaces

### Glossary Parsing

```go
// internal/glossary/glossary.go
glossary.Parse(root string) ([]glossary.Entry, error)
// Returns all glossary entries with Term, Label, Target, FilePath, Anchor
```

### Glossary Matching

```go
// internal/glossary/match.go
glossary.Match(entries []glossary.Entry, text string) []glossary.Entry
// Returns matched terms, longest-match-first, case-insensitive
```

### Section Extraction

```go
// internal/glossary/section.go
glossary.ExtractSection(root, filePath, anchor string) (string, error)
// Extracts a specific section from a markdown file by anchor
```

### Context Pack Generation (Spec 003)

```go
// internal/contextpack/builder.go
contextpack.Build(root, specID, mode string) (*ContextPack, error)
// Assembles a context pack for the given spec and mode

// internal/contextpack/spec.go
contextpack.ParseSpec(specDir string) (*SpecMeta, error)
// Parses spec.md to extract goal and impacted domains

// internal/contextpack/domaindoc.go
contextpack.ReadDomainDocs(root, domain string) (*DomainDoc, error)
// Reads 4 standard domain doc files (overview, architecture, interfaces, runbook)

// internal/contextpack/contextmap.go
contextpack.ParseContextMap(path string) ([]Relationship, error)
contextpack.ResolveNeighbors(rels []Relationship, impactedDomains []string) []string
// Parses Context Map relationships and resolves 1-hop neighbors

// internal/contextpack/adr.go
contextpack.ScanADRs(root string) ([]ADR, error)
contextpack.FilterADRs(adrs []ADR, domains []string) []ADR
// Scans and filters ADRs by status and domain

// internal/contextpack/policy.go
contextpack.ParsePolicies(path string) ([]Policy, error)
contextpack.FilterPolicies(policies []Policy, mode string) []Policy
// Parses policies.yml and filters by mode
```

## Consumed Interfaces

- **core**: `workspace.FindRoot()`, `workspace.GlossaryPath()`, `workspace.DocsDir()`, `workspace.SpecDir()`, `workspace.ContextMapPath()`, `workspace.ADRDir()`, `workspace.PoliciesPath()`, `workspace.DomainDir()`
- **workflow**: Spec bead metadata (impacted domains, ADR citations) for context pack routing

## Events

None defined yet. Future: context pack generation events for observability (tokens injected, cache hits).
