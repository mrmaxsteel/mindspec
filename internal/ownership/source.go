package ownership

// sourcePopulatePrompt is the spec 091 Requirement 12 agent prompt,
// verbatim. Repo-wide (no domain parameter, hence no substitution).
// ZFC: no pre-filled glob values — the FULL-REPLACE coverage warning
// is part of the prompt because a non-empty source_globs list FULLY
// REPLACES the built-in classifier (Requirement 16 override
// semantics).
const sourcePopulatePrompt = `Populate the ` + "`source_globs:`" + ` field in
.mindspec/config.yaml.

Inspect THIS repo's directory layout — ` + "`ls -R`, `find`" + `,
` + "`go list ./...`" + `, or whatever discovery commands fit your
tools — and identify the path globs that match all
hand-authored source code (any language), excluding
documentation, generated artifacts, vendored
dependencies, and test fixtures.

The framework deliberately provides no pattern hints —
that classification depends on THIS repo's layout and
conventions, not a template. Reach your own
determination by reading the tree.

IMPORTANT: a non-empty ` + "`source_globs:`" + ` list FULLY
REPLACES mindspec's built-in default classifier (it is
never merged with it). Your list must therefore cover
EVERYTHING the doc-sync gate should treat as source —
a too-narrow list narrows the gate.

The resulting ` + "`source_globs:`" + ` determines which file
changes the doc-sync gate considers "source": files
that match ` + "`source_globs`" + ` but do not match any domain's
OWNERSHIP.yaml ` + "`paths`" + ` fire the ` + "`unclaimed-source`" + ` Warn
(Requirement 16).

When done, edit .mindspec/config.yaml's ` + "`source_globs:`" + `
list. Run ` + "`mindspec doctor`" + ` to confirm
` + "`missing-source-globs`" + ` no longer Warns.
`

// BuildSourcePopulatePrompt returns the Requirement 12 agent prompt
// instructing the resident coding agent to propose source_globs
// entries. Unlike BuildPopulatePrompt (per-domain), this is repo-wide
// and takes no argument.
func BuildSourcePopulatePrompt() string {
	return sourcePopulatePrompt
}
