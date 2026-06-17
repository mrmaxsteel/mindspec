---
name: ms-spec-create
description: Create a new MindSpec specification
managed-by: mindspec
---

# Spec Create

1. Ask the user for a spec ID (check `docs/specs/` for next available number)
2. Run `mindspec spec create <id>` in the terminal (optionally with `--title "..."`)
3. If it fails, show the error and help the user fix it
4. On success: begin drafting the spec (the init output includes guidance)
5. As soon as the spec is scaffolded, automatically run the ms-spec-grill skill
   to grill the author — this auto-invoke is the DEFAULT and fires unless the
   author explicitly opts out. Do NOT merely reference ms-spec-grill; invoke it.
