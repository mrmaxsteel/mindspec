# Spec: Add background job notifications

## Goal
Notify users about their background jobs so they stay in the loop.

## Impacted Domains
- notifications: the new alerting layer
- workflow: command wiring

## Requirements
1. Enable delivery of a notification when a background job finishes.
2. Every notification is delivered exactly once per job, with no duplicates.
3. Notifications are sent at-least-once and may arrive more than once for a single job.
4. A user with notifications disabled receives no messages, verified by a test that disables the preference, runs a job to completion, and asserts zero messages were enqueued.
5. Each notification is dispatched before the job has finished running, so the user is alerted while the work is still in progress.

## Scope
### In Scope
- the alerting layer
- the per-user preference flag
### Out of Scope
- (none)

## Acceptance Criteria
- [ ] Notifications behave the way users expect.
- [ ] A completed job for a user with notifications enabled enqueues exactly one message, asserted by a test that runs one job and counts enqueued messages == 1.
- [ ] Disabling the preference suppresses all messages, asserted by the disabled-preference test above showing zero enqueued messages.
- [ ] A notification is enqueued only after the job has fully completed, asserted by a test that fails if any message is observed while the job is still running.
