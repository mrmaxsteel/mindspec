# Context-System Domain — Runbook

## Common Operations

### List Glossary Terms

```bash
mindspec glossary list
```

### Match Terms in Text

```bash
mindspec glossary match "spec mode approval"
```

### Show Documentation for a Term

```bash
mindspec glossary show "Context Pack"
```

### Generate Context Pack

```bash
# Planned (Spec 003)
mindspec context pack <spec-id>
mindspec context pack 001 --mode plan
```

### Add a Glossary Entry

Edit `GLOSSARY.md` and add a row to the table:
```
| **New Term** | [label](relative/path#anchor) |
```

Use relative paths from the project root. Ensure the target anchor exists.

### Validate Glossary Links

```bash
mindspec doctor
```

The doctor command checks for broken glossary links as part of health validation.

## Troubleshooting

### Broken Glossary Link

If `mindspec doctor` reports a broken link:
1. Check that the target file exists at the relative path
2. Check that the anchor (after `#`) matches a heading in the target file
3. Update the glossary entry or fix the target document
