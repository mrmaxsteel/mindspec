# Spec: Add response caching

## Goal
Add response caching to make things faster.

## Impacted Domains
- caching: the new cache layer
- workflow: command wiring

## Requirements
1. Support caching of responses.
2. The cache should be invalidated when data changes.
3. Once a response is cached, that entry never expires and is served unchanged for the lifetime of the process.
4. Add a flag to disable caching.

## Scope
### In Scope
- the cache layer
### Out of Scope
- (none)

## Acceptance Criteria
- [ ] Caching works correctly.
- [ ] The cache is fast.
- [ ] Disabling the cache via the flag returns uncached responses, verified by a test asserting two identical requests both hit the backend.
