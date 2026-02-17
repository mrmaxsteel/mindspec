# Migration Plan

- Run ID: `20260217T234919Z`
- Generated At: `2026-02-17T23:49:19Z`
- LLM Provider: `claude-cli` (available=true)
- Operations: `3`

## 1. `update` -> `.mindspec/docs/user/AGENTS.md`

- Operation ID: `op-001`
- Confidence: `0.85`
- LLM Used: `false`
- Rationale: AGENTS.md maps to .mindspec/docs/user/AGENTS.md via rule "path-user-docs-operational"; rationale: Deterministic classification rule matched source path/content.
- Sources:
  - `AGENTS.md` (sha256=6f37b4fee553670e7fd0e6eec8c20fd7592e133a89cdaae25de1545e03cfc1fa, category=user-docs, rule=path-user-docs-operational)
- Archive Targets:
  - `docs_archive/20260217T234919Z/AGENTS.md`

## 2. `update` -> `.mindspec/docs/user/CLAUDE.md`

- Operation ID: `op-002`
- Confidence: `0.85`
- LLM Used: `false`
- Rationale: CLAUDE.md maps to .mindspec/docs/user/CLAUDE.md via rule "path-user-docs-operational"; rationale: Deterministic classification rule matched source path/content.
- Sources:
  - `CLAUDE.md` (sha256=5c0e62d318d2a8cecb801c675cef8b7abca29d22fa035c82f385f2e3b77ca806, category=user-docs, rule=path-user-docs-operational)
- Archive Targets:
  - `docs_archive/20260217T234919Z/CLAUDE.md`

## 3. `update` -> `.mindspec/docs/user/README.md`

- Operation ID: `op-003`
- Confidence: `0.80`
- LLM Used: `false`
- Rationale: README.md maps to .mindspec/docs/user/README.md via rule "path-user-docs-heuristic"; rationale: Deterministic classification rule matched source path/content.
- Sources:
  - `README.md` (sha256=2d133b83ee7e3aae56ef57923130421ee9e2cf0943e8b9bb50b7b31c62ad8901, category=user-docs, rule=path-user-docs-heuristic)
- Archive Targets:
  - `docs_archive/20260217T234919Z/README.md`

