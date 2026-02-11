# Context-System Domain — Runbook

## Common Operations

### List Glossary Terms

```bash
python -m mindspec glossary list
```

### Match Terms in Text

```bash
python -m mindspec glossary match "spec mode approval"
```

### Generate Context Pack

```bash
python -m mindspec context pack <spec-id>
python -m mindspec context pack 001 --mode plan
```

### Add a Glossary Entry

Edit `GLOSSARY.md` and add a row to the table:
```
| **New Term** | [label](relative/path#anchor) |
```

Use relative paths from the project root. Ensure the target anchor exists.

### Validate Glossary Links

```bash
python -m mindspec doctor
```

The doctor command checks for broken glossary links as part of health validation.

## Troubleshooting

### Broken Glossary Link

If `mindspec doctor` reports a broken link:
1. Check that the target file exists at the relative path
2. Check that the anchor (after `#`) matches a heading in the target file
3. Update the glossary entry or fix the target document
