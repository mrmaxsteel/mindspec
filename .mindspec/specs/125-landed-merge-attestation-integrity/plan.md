---
status: Draft
spec_id: 125-landed-merge-attestation-integrity
version: "1"
# adr_citations: cite the Accepted ADRs whose Domain(s) cover this plan's
# impacted domains (drop the leading "# " and list them); leave empty only when
# the ## ADR Fitness section explains why no ADR applies.
# adr_citations:
#   - ADR-XXXX
# work_chunks: the AUTHORITATIVE dependency-wiring source (spec 097 R3) — the
# ONLY thing bd dependency edges are wired from. Each chunk's "id" is 1-based
# and maps positionally to the Nth "## Bead N" section below (chunk id 1 -> the
# first "## Bead" section, id 2 -> the second, and so on); "depends_on" lists
# the chunk ids this chunk depends on (e.g. depends_on: [1] wires this bead to
# depend on Bead 1); "key_file_paths" declares the files that bead's context
# surface should carry. Add one chunk per "## Bead N" section, keeping ids
# contiguous (1..N) — the plan-approve preflight refuses a misaligned set
# before any mutation.
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - path/to/file.go
---
# Plan: 125-landed-merge-attestation-integrity

## ADR Fitness

No ADRs are relevant to this work. (Update this section if ADRs apply.)

## Testing Strategy

Unit tests will verify the implementation.

## Bead 1: <Title>

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] `make test` passes

**Acceptance Criteria**
- <Specific, measurable criterion for this bead>

**Depends on**
None (human-readable documentation only — NOT parsed; bd dependency edges
are wired exclusively from this bead's `work_chunks[].depends_on` entry
in the frontmatter above)

## Provenance

| Acceptance Criterion | Verified By |
|---------------------|-------------|
| (map spec criteria) | Bead 1 verification |
