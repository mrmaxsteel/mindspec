# Core Domain — Runbook

## Common Operations

### Run Health Check

```bash
python -m mindspec doctor
```

Validates project structure and reports issues. Exit code 0 = healthy, 1 = errors found.

### Verify Workspace Detection

If `mindspec doctor` can't find the project root, ensure one of these exists in an ancestor directory:
- `.mindspec/` directory (preferred)
- `.git` directory

### Add a New CLI Command

1. Define the command in `src/mindspec/cli.py`
2. Implement logic in a dedicated module (e.g., `src/mindspec/doctor.py`)
3. Register in the CLI command group

### Update Policies

Edit `architecture/policies.yml`. Each policy needs: `id`, `description`, `severity`, and optionally `scope`, `mode`, `reference`.

### A lifecycle verb refuses with "timed out waiting for the phase write lock" (spec 128 R3.3)

Every `mindspec_phase` writer (`plan approve`, `spec approve`, `complete`,
`release`, `impl approve`/`adopt`, `repair phase`) now serializes through a
kernel-held lock at `.mindspec/phase.lock`. A timeout means the lock is
genuinely contended by another live process (a crashed holder's kernel-held
lock has already released on its own — never delete the lock file as a
"fix"). Re-run the same command once the concurrent writer finishes; the
lock's bounded wait (10s default) is a poll-with-deadline, not a deadlock.
