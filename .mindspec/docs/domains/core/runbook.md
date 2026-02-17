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
