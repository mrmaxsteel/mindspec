# mindspec

**Spec-Driven Development + Memory + Self-Documentation System for Antigravity.**

Mindspec is a tool designed to turn system intent into structured specifications, actionable task graphs, and persistent project memory. It ensures that documentation is a first-class outcome of the development process rather than an afterthought.

## Core Principles

1.  **Spec-Driven**: Every feature starts with a formal specification.
2.  **Self-Documenting**: Implementation automatically updates and refactors documentation.
3.  **Deterministic Context**: Progressive disclosure of architectural context via keyword-anchored docs.
4.  **Durable Memory**: Locally-persisted "Project Brain" for cross-session rationale and decisions.

## Getting Started

Review the [Architecture Documentation](file:///Users/Max/Documents/mindspec/docs/core/ARCHITECTURE.md) to understand the system design.

The project is currently in the **Skeleton Initialization** phase. See [Spec 001: Skeleton](file:///Users/Max/Documents/mindspec/docs/specs/001-skeleton/spec.md) for current implementation goals.

## Project Structure

*   `docs/core/`: Permanent architectural context.
*   `docs/specs/`: Versioned feature specifications.
*   `GLOSSARY.md`: Keyword-to-anchor mapping.
*   `architecture/`: Machine-readable policies.

## Usage (Future)

```bash
# Verify project health
python -m mindspec doctor

# Generate context pack for a task
python -m mindspec context pack 001

# Run validation gates
python -m mindspec validate run 001
```
