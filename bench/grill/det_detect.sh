#!/usr/bin/env bash
# Deterministic detector: faithfully encodes the CEILING of schema-driven heuristics
# the Go interview engine can do — empty-detection, exact bad-phrase, domain-set
# membership, <3 ACs. (Synonym, semantic-thinness, and cross-requirement
# contradiction are out of reach for any deterministic rule set.)
set -u
REAL_DOMAINS="workflow execution core context-system"
for f in "$@"; do
  name=$(basename "$f" .md)
  echo "### $name"
  # 1. empty / placeholder Out of Scope
  if awk '/^### Out of Scope/{g=1;next} /^##/{g=0} g&&NF{ if($0 ~ /\(none\)|^- *$|^-? *TBD/) e=1; else if($0 ~ /^- /) c++} END{exit !(e || c==0)}' "$f"; then
    echo "FLAG STRUCTURAL: Out-of-Scope empty/placeholder"
  fi
  # 2. exact bad-phrase in Requirements: support X / handle Y
  grep -nEi '^[0-9]+\. .*\b(support|handle)s?\b' "$f" | while read -r l; do echo "FLAG EXACT_PHRASE(req): $l"; done
  # 3. exact bad-phrase in ACs: works correctly / it works / is correct / function(s) properly? — engine's known list
  grep -nEi '^- \[ \] .*(works correctly|it works|is correct|works\.|functions? properly|handles .* correctly)' "$f" | while read -r l; do echo "FLAG EXACT_PHRASE(ac): $l"; done
  # 4. domain-set membership
  awk '/^## Impacted Domains/{g=1;next} /^## /{g=0} g && /^- /{ sub(/^- */,""); split($0,a,":"); d=a[1]; gsub(/ /,"",d); print d }' "$f" | while read -r d; do
    case " $REAL_DOMAINS " in *" $d "*) : ;; *) echo "FLAG GROUNDING: unknown domain '$d'";; esac
  done
  # 5. <3 acceptance criteria
  n=$(grep -cE '^- \[[ x]\] ' "$f"); [ "$n" -lt 3 ] && echo "FLAG STRUCTURAL: only $n acceptance criteria (<3)"
  echo
done
