package domain

import "strings"

// ContextMapSkeleton returns the skeleton content for a fresh
// context-map.md: a title, a "## Bounded Contexts" section, then a "---"
// separator — so appendContextMap's insertion scan (which looks for the
// end of the Bounded Contexts section) finds a place to insert new entries
// immediately, rather than falling back to appending after unrelated tail
// content (spec 123 R1). Both `mindspec init` (bootstrap manifest) and
// doctor's missing-context-map --fix scaffold this exact skeleton, so a
// freshly bootstrapped project and a doctor-repaired one converge on
// identical bytes.
func ContextMapSkeleton() string {
	return "# Context Map\n\n## Bounded Contexts\n\n---\n"
}

// EntryHeading returns the exact "### <Title>" heading appendContextMap
// emits for a bounded-context entry.
func EntryHeading(title string) string {
	return "### " + title
}

// EntryHeadingForDomain returns the entry heading for a domain by its
// directory name (title-cased the same way Add does), so callers that only
// have the raw domain name — like doctor's unmapped-domain scan — compute
// the SAME heading Add would emit, without re-implementing titleCase.
func EntryHeadingForDomain(name string) string {
	return EntryHeading(titleCase(name))
}

// HasEntry reports whether context-map content already contains the
// bounded-context entry heading for a domain name. This is the single "is
// this domain mapped" predicate: both the emission side (scaffold.Add's
// appendContextMap backfill) and the detection side (doctor's
// unmapped-domain check) consume this exact helper — never a private
// reimplementation — so the two sides cannot silently disagree about what
// "mapped" means (spec 123 R3/AC-4).
//
// Detection is SECTION-AWARE: a `### <Title>` heading counts only when it
// sits INSIDE the `## Bounded Contexts` section — the exact place
// appendContextMap emits it, before the section's `---` separator. A
// same-named heading elsewhere in the document (e.g. under a `## Notes` or
// `## Relationships` section) is NOT a mapping, matching the writer's own
// insertion scan. Without this, a fully-scaffolded domain whose only
// matching heading lives outside the section would be wrongly refused as
// "already exists" and stranded in a terminal unmapped state a re-run
// could never repair (FX-1).
func HasEntry(content, name string) bool {
	heading := EntryHeadingForDomain(name)
	inBoundedContexts := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			// A level-2 heading either opens the Bounded Contexts section or
			// (once we're inside it) closes it by starting the next section.
			if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), "Bounded Contexts") {
				inBoundedContexts = true
			} else if inBoundedContexts {
				return false
			}
			continue
		}

		if !inBoundedContexts {
			continue
		}

		// The section's `---` separator closes it (the same terminator
		// appendContextMap inserts entries in front of).
		if trimmed == "---" {
			return false
		}

		if trimmed == heading {
			return true
		}
	}
	return false
}
