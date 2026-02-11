# Mindspec Core Architecture

Mindspec is a workspace-aware workflow and memory layer designed to facilitate spec-driven development and self-documentation for Antigravity-assisted projects.

## System Components

### 1. Workflow Service
Manages the lifecycle of a specification:
*   **Spec Generation**: Converting intent into a structured spec.
*   **Task Graph Construction**: Breaking down specs into actionable tasks with dependencies.
*   **Execution Protocol**: Orchestrating task execution and ensuring validation gates are met.

### 2. Memory Service
A local persistent store (SQLite) that maintains the "Project Brain":
*   **Storage**: Decisions, gotchas, rationale, and debugging outcomes.
*   **Recall**: Hybrid retrieval (keyword + vector) for progressive disclosure in context packs.

### 3. Workspace Provider
Abstracts the underlying repository structure:
*   Supports multi-repo setups (e.g., frontend + backend).
*   Resolves repo roots and SHAs deterministically.
*   Facilitates cross-repo task dispatch.

#### Provider Implementations

| Provider | Use Case |
| :------- | :------- |
| **Config-only** | Reads `workspace.yml` for explicit repo roots |
| **Meta-repo + Submodules** | Resolves `.gitmodules`, pins SHAs deterministically |
| **CI Provider** | Reads environment variables/paths in CI context |

The **meta-repo + submodules** strategy is the recommended default for projects requiring version-coupled multi-repo setups.

#### Commit Tuple

Every context pack and validation report records a **commit tuple**:
*   `repo_alias → commit SHA` for each repository
*   Injected doc sections (file + anchor)
*   Memory fragments used (IDs)

This ensures reproducibility across machines and time.

## Operational Modes {#operational-modes}

Mindspec enforces a **gated lifecycle** with two distinct operational modes:

1.  **Spec Mode**: Only markdown artifacts may be created/modified. Focus on intent capture, requirements, and acceptance criteria.
2.  **Implementation Mode**: Code changes permitted. Requires an approved spec with all acceptance criteria met.

The transition between modes is an explicit **Approval Gate** requiring human sign-off.

For full details, see [MODES.md](MODES.md).

## Core Invariants

1.  **Docs-First**: Every non-trivial change must update corresponding documentation.
2.  **Spec-Anchored**: All code changes originate from a versioned specification.
3.  **Human Gate for Divergence**: Architecture deviations require an Architecture Change Proposal (ACP) and human approval.
4.  **Proof of Done**: Tasks are only complete when automated proofs and manual validation gates pass.

