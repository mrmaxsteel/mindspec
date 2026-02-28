package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const preCommitScript = `#!/usr/bin/env bash
# MindSpec pre-commit hook (Layer 1 enforcement — ADR-0019)
# Prevents commits on protected branches when mindspec is active.

# Escape hatch: MINDSPEC_ALLOW_MAIN=1 git commit
if [ "${MINDSPEC_ALLOW_MAIN:-}" = "1" ]; then
  exit 0
fi

# Read focus — if no cache file, allow commit
MODE_CACHE=".mindspec/focus"
if [ ! -f "$MODE_CACHE" ]; then
  exit 0
fi

MODE=$(cat "$MODE_CACHE" 2>/dev/null | grep -o '"mode"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"mode"[[:space:]]*:[[:space:]]*"//;s/"$//')
if [ -z "$MODE" ] || [ "$MODE" = "idle" ]; then
  exit 0
fi

# Check enforcement config
CONFIG_FILE=".mindspec/config.yaml"
if [ -f "$CONFIG_FILE" ]; then
  if grep -q 'pre_commit_hook.*false' "$CONFIG_FILE" 2>/dev/null; then
    exit 0
  fi
fi

# Get current branch
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
if [ -z "$BRANCH" ]; then
  exit 0
fi

# Read protected branches from config (default: main, master)
PROTECTED="main master"
if [ -f "$CONFIG_FILE" ]; then
  CUSTOM=$(grep -A5 'protected_branches' "$CONFIG_FILE" 2>/dev/null | grep '^\s*-' | sed 's/^[[:space:]]*-[[:space:]]*//' | tr '\n' ' ')
  if [ -n "$CUSTOM" ]; then
    PROTECTED="$CUSTOM"
  fi
fi

# Check if current branch is protected
for p in $PROTECTED; do
  if [ "$BRANCH" = "$p" ]; then
    WORKTREE=$(cat "$MODE_CACHE" 2>/dev/null | grep -o '"activeWorktree"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"activeWorktree"[[:space:]]*:[[:space:]]*"//;s/"$//')
    echo "mindspec: commits on '$BRANCH' are blocked while mindspec is active (mode: $MODE)." >&2
    if [ -n "$WORKTREE" ]; then
      echo "  Switch to your worktree: cd $WORKTREE" >&2
    fi
    echo "  Escape hatch: MINDSPEC_ALLOW_MAIN=1 git commit ..." >&2
    exit 1
  fi
done

exit 0
`

// InstallPreCommit installs the MindSpec pre-commit hook.
// It uses the git hooks path and chains with any existing pre-commit hook.
func InstallPreCommit(root string) error {
	hooksDir := filepath.Join(root, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		// Not a git repo or bare repo — skip
		return nil
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	marker := "# MindSpec pre-commit hook"

	// Check if already installed
	if data, err := os.ReadFile(hookPath); err == nil {
		if strings.Contains(string(data), marker) {
			return nil // already installed
		}
		// Existing hook — chain by renaming and calling it
		backupPath := hookPath + ".pre-mindspec"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			if err := os.Rename(hookPath, backupPath); err != nil {
				return fmt.Errorf("backing up existing pre-commit hook: %w", err)
			}
		}
		// Write chained hook
		chained := preCommitScript + "\n# Chain to previous hook\nif [ -x .git/hooks/pre-commit.pre-mindspec ]; then\n  .git/hooks/pre-commit.pre-mindspec\nfi\n"
		return os.WriteFile(hookPath, []byte(chained), 0755)
	}

	// No existing hook — write directly
	return os.WriteFile(hookPath, []byte(preCommitScript), 0755)
}
