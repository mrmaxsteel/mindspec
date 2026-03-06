package bench

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// QualitativeResult holds the outputs of qualitative analysis.
type QualitativeResult struct {
	Analysis     string
	Improvements string
}

// RunQualitative runs both qualitative analysis and improvements extraction
// via claude -p --no-session-persistence.
func RunQualitative(cfg *RunConfig, quantReport string, plans, diffs map[string]string) (*QualitativeResult, error) {
	qualPrompt := buildQualPrompt(cfg.Prompt, quantReport, plans, diffs)
	fmt.Fprintln(cfg.Stdout, "Running qualitative analysis...")
	analysis, err := runClaudeAnalysis(qualPrompt)
	if err != nil {
		return &QualitativeResult{Analysis: "(qualitative analysis failed)"}, nil
	}

	improvPrompt := buildImprovPrompt(cfg.Prompt, analysis, plans, diffs)
	fmt.Fprintln(cfg.Stdout, "Running improvements analysis...")
	improvements, err := runClaudeAnalysis(improvPrompt)
	if err != nil {
		return &QualitativeResult{
			Analysis:     analysis,
			Improvements: "(improvements analysis failed)",
		}, nil
	}

	return &QualitativeResult{
		Analysis:     analysis,
		Improvements: improvements,
	}, nil
}

// CollectPlans gathers plan artifacts from each session's worktree.
func CollectPlans(cfg *RunConfig, specID string) map[string]string {
	plans := make(map[string]string)

	// Session C (mindspec): plan under the active docs root.
	planC := filepath.Join(workspace.SpecDir(filepath.Join(cfg.WorkDir, "wt-c"), specID), "plan.md")
	if data, err := os.ReadFile(planC); err == nil {
		plans["c"] = string(data)
	}

	// Sessions A and B: check .claude/plans/ for any plan files
	for _, label := range []string{"a", "b"} {
		planDir := filepath.Join(cfg.WorkDir, "wt-"+label, ".claude", "plans")
		entries, err := os.ReadDir(planDir)
		if err != nil {
			plans[label] = fmt.Sprintf("(No plan artifact found for Session %s)", strings.ToUpper(label))
			continue
		}
		found := false
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				data, err := os.ReadFile(filepath.Join(planDir, e.Name()))
				if err == nil {
					plans[label] = string(data)
					found = true
					break
				}
			}
		}
		if !found {
			plans[label] = fmt.Sprintf("(No plan artifact found for Session %s)", strings.ToUpper(label))
		}
	}

	return plans
}

// GenerateDiffs generates git diffs for each session, excluding benchmark artifacts.
func GenerateDiffs(cfg *RunConfig, benchCommit string) map[string]string {
	diffs := make(map[string]string)
	const maxDiffChars = 100000

	for _, label := range []string{"a", "b", "c"} {
		wtPath := filepath.Join(cfg.WorkDir, "wt-"+label)
		cmd := exec.Command("git", "-C", wtPath, "diff", benchCommit, "HEAD",
			"--",
			":(exclude).beads",
			":(exclude).mindspec",
			":(exclude)docs/specs",
			":(exclude).mindspec/docs/specs")
		out, err := cmd.Output()
		if err != nil {
			diffs[label] = "(diff generation failed)"
			continue
		}
		diff := string(out)
		if len(diff) > maxDiffChars {
			diff = diff[:maxDiffChars] + fmt.Sprintf("\n\n[... truncated at %d chars ...]", maxDiffChars)
		}
		diffs[label] = diff
	}

	return diffs
}

func buildQualPrompt(prompt, quantReport string, plans, diffs map[string]string) string {
	var b strings.Builder

	b.WriteString(`You are a senior software engineer reviewing three implementations of the same feature,
produced by Claude Code under different conditions:

- **Session A (no-docs)**: No MindSpec tooling AND no docs/ directory — pure freestyle
  with no project documentation.
- **Session B (baseline)**: No MindSpec tooling (CLAUDE.md, .mindspec/ removed, hooks
  stripped from .claude/settings.json, MindSpec commands removed), but docs/ directory
  (domain docs, ADRs, context map) and .claude/ directory still present.
- **Session C (mindspec)**: Full MindSpec tooling — spec-driven workflow with CLAUDE.md,
  hooks, domain documentation, context map, and ADRs.

All three sessions started from the same git commit and received the same feature prompt:

> `)
	b.WriteString(prompt)
	b.WriteString("\n\n## Quantitative Report\n\n```\n")
	b.WriteString(quantReport)
	b.WriteString("\n```\n\n## Plans\n\n### Session A Plan (Claude /plan mode)\n```markdown\n")
	b.WriteString(plans["a"])
	b.WriteString("\n```\n\n### Session B Plan (Claude /plan mode)\n```markdown\n")
	b.WriteString(plans["b"])
	b.WriteString("\n```\n\n### Session C Plan (mindspec plan.md)\n```markdown\n")
	b.WriteString(plans["c"])
	b.WriteString("\n```\n\n## Implementation Diffs\n\n### Session A (no-docs)\n```diff\n")
	b.WriteString(diffs["a"])
	b.WriteString("\n```\n\n### Session B (baseline)\n```diff\n")
	b.WriteString(diffs["b"])
	b.WriteString("\n```\n\n### Session C (mindspec)\n```diff\n")
	b.WriteString(diffs["c"])
	b.WriteString(`
` + "```" + `

## Your Task

Analyze all three implementations and produce a structured comparison. Be completely unbiased.

### Dimensions

For each dimension, rate each session 1-5 and explain briefly:

1. **Planning Quality**: Clarity of the plan, scope decomposition, verification steps, architectural reasoning.
2. **Architecture**: Code organization, separation of concerns, package structure, interface design.
3. **Code Quality**: Readability, error handling, naming, idiomatic style, absence of code smells.
4. **Test Quality**: Coverage, edge cases, test isolation, meaningful assertions.
5. **Documentation**: Code comments, doc-sync, inline documentation quality.
6. **Functional Completeness**: Does the implementation satisfy the feature prompt fully?
7. **Consistency with Project Conventions**: Does the code follow patterns visible in the existing codebase?

### Output Format

Use this exact structure:

### Planning Quality
| Session | Rating | Assessment |
|---------|--------|------------|
| A (no-docs)  | X/5 | ... |
| B (baseline) | X/5 | ... |
| C (mindspec) | X/5 | ... |

[repeat for each dimension]

### Overall Verdict
[Which session produced the best overall result and why — 3-5 sentences]

### Key Differentiators
[What specific advantages did MindSpec provide, or fail to provide?]

### Surprising Findings
[Anything unexpected in the comparison]
`)

	return b.String()
}

func buildImprovPrompt(prompt, qualAnalysis string, plans, diffs map[string]string) string {
	var b strings.Builder

	b.WriteString(`You are analyzing three implementations of the same feature to identify what the
non-MindSpec sessions (A and B) did BETTER than the MindSpec session (C).

The feature prompt was:
> `)
	b.WriteString(prompt)
	b.WriteString("\n\n## Plans\n\n### Session A Plan (no-docs)\n```markdown\n")
	b.WriteString(plans["a"])
	b.WriteString("\n```\n\n### Session B Plan (baseline)\n```markdown\n")
	b.WriteString(plans["b"])
	b.WriteString("\n```\n\n### Session C Plan (mindspec)\n```markdown\n")
	b.WriteString(plans["c"])
	b.WriteString("\n```\n\n## Implementation Diffs\n\n### Session A (no-docs — no MindSpec, no docs/)\n```diff\n")
	b.WriteString(diffs["a"])
	b.WriteString("\n```\n\n### Session B (baseline — no MindSpec, but has docs/)\n```diff\n")
	b.WriteString(diffs["b"])
	b.WriteString("\n```\n\n### Session C (mindspec)\n```diff\n")
	b.WriteString(diffs["c"])
	b.WriteString("\n```\n\n## Qualitative Analysis (already completed)\n\n")
	b.WriteString(qualAnalysis)
	b.WriteString(`

## Your Task

Identify specific, actionable improvements from sessions A and B that session C should
adopt. Focus on:

1. **Code patterns** A/B used that are objectively better (simpler, more idiomatic, better error handling)
2. **Features or edge cases** A/B handled that C missed
3. **Architectural decisions** in A/B that are cleaner (even if session C was "correct by spec")
4. **Planning approaches** in A/B that produced better outcomes
5. **Test approaches** in A/B that are more thorough or practical
6. **Documentation or naming** that A/B got right where C did not

For each improvement, provide:
- Which session(s) it came from (A, B, or both)
- What specifically was better
- A concrete suggestion for how to incorporate it into the MindSpec implementation

If there are no meaningful improvements from A/B, say so explicitly.

Output format:

# Improvements from Non-MindSpec Sessions

## Summary
[1-2 sentences: overall, did A/B produce anything worth adopting?]

## Improvements

### 1. [Brief title]
**Source**: Session A / B / Both
**What was better**: ...
**Suggestion**: ...

### 2. [Brief title]
...

## Conclusion
[2-3 sentences: what does this tell us about MindSpec workflow gaps?]
`)

	return b.String()
}

func runClaudeAnalysis(prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", "--no-session-persistence")
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = append(os.Environ(), "CLAUDECODE=")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude analysis: %w", err)
	}
	return string(out), nil
}
