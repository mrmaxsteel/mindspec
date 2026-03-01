---
approved_at: "2026-03-01T08:48:20Z"
approved_by: user
molecule_id: mindspec-mol-1hh
status: Approved
step_mapping:
    implement: mindspec-mol-kp5
    plan: mindspec-mol-c8v
    plan-approve: mindspec-mol-9qj
    review: mindspec-mol-22f
    spec: mindspec-mol-x83
    spec-approve: mindspec-mol-y9w
    spec-lifecycle: mindspec-mol-1hh
---

# Spec 044-launch-website: MindSpec Launch Website

## Goal

Ship a landing page at mindspec.ai with an embedded AgentMind recording so visitors immediately see what MindSpec does.

## Background

For sharing minsepc, the GitHub repo will be the primary artifact, but a dedicated website at mindspec.ai serves two purposes:

1. **First impression**: A curated AgentMind recording showing a real spec-to-implementation flow is more compelling than a README.
2. **Shareability**: A clean URL (mindspec.ai) is easier to share on social media than a GitHub link, and the embedded player gives visitors an interactive experience.

AgentMind already renders in the browser, so the technical lift is embedding an existing component rather than building a player from scratch.

## Impacted Domains

- None (new standalone project, not part of the MindSpec CLI codebase)

## ADR Touchpoints

- None currently applicable

## Requirements

1. **Landing page** at mindspec.ai with:
   - Project tagline and brief description (what MindSpec is, who it's for)
   - Embedded AgentMind recording showing a real workflow
   - "Star on GitHub" CTA linking to the repo
   - "Get Started" link to repo README or docs
2. **AgentMind embed**: At least one curated recording demonstrating a spec-to-implementation flow, playable inline on the page
3. **Mobile-responsive**: Page must be readable and recording must be viewable on mobile
4. **Fast load**: Page should load in under 3 seconds on a typical connection (no heavy frameworks if avoidable)
5. **Deployment**: Hosted on a platform with zero ongoing maintenance (e.g., Vercel, Netlify, GitHub Pages)
6. **Domain**: mindspec.ai pointed at the hosting provider

## Scope

### In Scope
- Single-page landing site (could be a separate repo or a `/site` directory)
- Embedding the existing AgentMind web component/player
- One curated recording (content selection + capture)
- DNS configuration for mindspec.ai
- Deployment pipeline (push-to-deploy)

### Out of Scope
- User accounts or authentication
- Recording upload by visitors
- Community sharing / share-link generation
- Blog or documentation hosting (docs stay in the repo)
- Analytics beyond basic page views (can add later)
- Multiple pages or routing

## Non-Goals

- This is not a product marketing site with pricing, features grid, testimonials, etc.
- No user-generated content flow in v1 — upload/share is a future feature
- No custom video player — we use AgentMind's existing rendering

## Acceptance Criteria

- [ ] mindspec.ai resolves and serves the landing page over HTTPS
- [ ] Page contains project name, tagline, and a one-paragraph description
- [ ] An AgentMind recording plays inline on the page without requiring user install
- [ ] "Star on GitHub" button links to the correct repo
- [ ] Page is responsive (renders correctly on viewport widths 375px–1440px)
- [ ] Lighthouse performance score >= 90
- [ ] Deployment is automated (git push triggers deploy)

## Validation Proofs

- `curl -sI https://mindspec.ai | head -5`: Returns HTTP 200 with valid TLS
- Lighthouse audit: Performance >= 90, Accessibility >= 90
- Manual check: AgentMind recording plays to completion on desktop and mobile

## Open Questions

- [x] Separate repo or subdirectory of mindspec? — **Separate repo** (keeps the CLI repo focused; site has different build/deploy tooling)
- [x] Which recording to feature? — **A spec-to-implementation flow** showing MindSpec + AgentMind in action (exact recording TBD during implementation)
- [x] Framework choice? — **Decide during planning** (could be as simple as static HTML + embedded AgentMind, or Next.js if we want easy iteration later)

## Approval

- **Status**: APPROVED
- **Approved By**: user
- **Approval Date**: 2026-03-01
- **Notes**: Approved via mindspec approve spec