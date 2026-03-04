package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// shimScript is the template for a recording shim. It logs the invocation
// to a JSONL file, then delegates to the real binary.
// Placeholders: %s = log path, %s = real binary path, %s = command name,
//
//	%s = mindspec binary path, %s = original PATH (without shim dir).
const shimScript = `#!/bin/sh
# Recording shim for %[3]s — logs invocations to JSONL before delegating.
LOG_PATH="%[1]s"
REAL_BIN="%[2]s"
CMD_NAME="%[3]s"
MINDSPEC_BIN="%[4]s"
ORIG_PATH="%[5]s"
START_NS=$(date +%%s%%N 2>/dev/null || python3 -c "import time; print(int(time.time()*1e9))" 2>/dev/null || echo 0)
"$REAL_BIN" "$@"
EXIT_CODE=$?
END_NS=$(date +%%s%%N 2>/dev/null || python3 -c "import time; print(int(time.time()*1e9))" 2>/dev/null || echo 0)
if [ "$START_NS" != "0" ] && [ "$END_NS" != "0" ]; then
  DURATION_MS=$(( (END_NS - START_NS) / 1000000 ))
else
  DURATION_MS=0
fi
CWD_PATH=$(pwd)
ARGS=$(printf '%%s\n' "$@" | python3 -c "
import sys, json
args = [line.rstrip() for line in sys.stdin]
print(json.dumps(args))
" 2>/dev/null || echo '[]')
# Phase cache: avoid expensive mindspec state show on every shim call.
# Cache is valid for 5 seconds — phase changes are infrequent.
PHASE_CACHE="${LOG_PATH}.phase-cache"
PHASE=""
if [ -f "$PHASE_CACHE" ]; then
  CACHE_LINE=$(cat "$PHASE_CACHE" 2>/dev/null)
  CACHE_TS=$(echo "$CACHE_LINE" | cut -d' ' -f1)
  NOW=$(date +%%s)
  if [ -n "$CACHE_TS" ] && [ $((NOW - CACHE_TS)) -lt 5 ]; then
    PHASE=$(echo "$CACHE_LINE" | cut -d' ' -f2-)
  fi
fi
if [ -z "$PHASE" ]; then
  PHASE=$(PATH="$ORIG_PATH" "$MINDSPEC_BIN" state show 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('mode',''))" 2>/dev/null || echo "")
  echo "$(date +%%s) $PHASE" > "$PHASE_CACHE" 2>/dev/null
fi
printf '{"timestamp":"%%s","action_type":"command","command":"%[3]s","args_list":%%s,"cwd":"%%s","exit_code":%%d,"duration_ms":%%d,"phase":"%%s"}\n' \
  "$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ 2>/dev/null || echo unknown)" \
  "$ARGS" \
  "$CWD_PATH" \
  "$EXIT_CODE" \
  "$DURATION_MS" \
  "$PHASE" \
  >> "$LOG_PATH"
exit $EXIT_CODE
`

// shimScriptPinCWD is like shimScript but forces CWD to a fixed directory
// before executing the real binary. Used for bd to ensure .beads/ resolution
// always finds the sandbox's database, not the host project's.
// Placeholders: %s = log path, %s = real binary path, %s = command name,
//
//	%s = pinned CWD, %s = mindspec binary path, %s = original PATH.
const shimScriptPinCWD = `#!/bin/sh
# Recording shim for %[3]s — CWD-pinned to sandbox root for .beads/ isolation.
LOG_PATH="%[1]s"
REAL_BIN="%[2]s"
CMD_NAME="%[3]s"
PIN_CWD="%[4]s"
MINDSPEC_BIN="%[5]s"
ORIG_PATH="%[6]s"
ORIG_CWD=$(pwd)
cd "$PIN_CWD"
START_NS=$(date +%%s%%N 2>/dev/null || python3 -c "import time; print(int(time.time()*1e9))" 2>/dev/null || echo 0)
"$REAL_BIN" "$@"
EXIT_CODE=$?
END_NS=$(date +%%s%%N 2>/dev/null || python3 -c "import time; print(int(time.time()*1e9))" 2>/dev/null || echo 0)
if [ "$START_NS" != "0" ] && [ "$END_NS" != "0" ]; then
  DURATION_MS=$(( (END_NS - START_NS) / 1000000 ))
else
  DURATION_MS=0
fi
ARGS=$(printf '%%s\n' "$@" | python3 -c "
import sys, json
args = [line.rstrip() for line in sys.stdin]
print(json.dumps(args))
" 2>/dev/null || echo '[]')
# Phase cache: avoid expensive mindspec state show on every shim call.
# Cache is valid for 5 seconds — phase changes are infrequent.
PHASE_CACHE="${LOG_PATH}.phase-cache"
PHASE=""
if [ -f "$PHASE_CACHE" ]; then
  CACHE_LINE=$(cat "$PHASE_CACHE" 2>/dev/null)
  CACHE_TS=$(echo "$CACHE_LINE" | cut -d' ' -f1)
  NOW=$(date +%%s)
  if [ -n "$CACHE_TS" ] && [ $((NOW - CACHE_TS)) -lt 5 ]; then
    PHASE=$(echo "$CACHE_LINE" | cut -d' ' -f2-)
  fi
fi
if [ -z "$PHASE" ]; then
  PHASE=$(PATH="$ORIG_PATH" "$MINDSPEC_BIN" state show 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('mode',''))" 2>/dev/null || echo "")
  echo "$(date +%%s) $PHASE" > "$PHASE_CACHE" 2>/dev/null
fi
printf '{"timestamp":"%%s","action_type":"command","command":"%[3]s","args_list":%%s,"cwd":"%%s","exit_code":%%d,"duration_ms":%%d,"phase":"%%s"}\n' \
  "$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ 2>/dev/null || echo unknown)" \
  "$ARGS" \
  "$ORIG_CWD" \
  "$EXIT_CODE" \
  "$DURATION_MS" \
  "$PHASE" \
  >> "$LOG_PATH"
exit $EXIT_CODE
`

// DefaultShimCommands is the list of commands that get recording shims.
var DefaultShimCommands = []string{"mindspec", "git", "bd", "gh"}

// InstallShims creates recording shim scripts in binDir for each command.
// Each shim logs to logPath and delegates to the real binary found via PATH
// (excluding binDir itself). Each shim also queries the current lifecycle
// phase via the real mindspec binary and records it in the JSONL output.
func InstallShims(binDir, logPath string) error {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("creating shim dir: %w", err)
	}

	// Find mindspec binary for phase queries. If not found (e.g. CI without
	// ./bin/mindspec), shims still work but phase will be empty.
	mindspecPath, _ := findRealBinary("mindspec", binDir)

	// Compute PATH without shim dir — phase queries use this to bypass shims
	// so mindspec's internal bd/git calls go directly to real binaries.
	origPath := pathWithout(binDir)

	for _, cmd := range DefaultShimCommands {
		realPath, err := findRealBinary(cmd, binDir)
		if err != nil {
			// If the real binary isn't found, skip (e.g. bd not installed in test env)
			continue
		}
		if err := writeShim(binDir, logPath, cmd, realPath, mindspecPath, origPath); err != nil {
			return fmt.Errorf("writing shim for %s: %w", cmd, err)
		}
	}
	return nil
}

// writeShim creates a single shim script.
func writeShim(binDir, logPath, cmdName, realPath, mindspecPath, origPath string) error {
	script := fmt.Sprintf(shimScript, logPath, realPath, cmdName, mindspecPath, origPath)
	shimPath := filepath.Join(binDir, cmdName)
	if err := os.WriteFile(shimPath, []byte(script), 0o755); err != nil {
		return err
	}
	return nil
}

// WritePinnedShim creates a CWD-pinned recording shim for a command.
// The shim changes to pinCWD before executing the real binary, ensuring
// .beads/ resolution stays within the sandbox regardless of agent CWD drift.
func WritePinnedShim(binDir, logPath, cmdName, pinCWD string) error {
	realPath, err := findRealBinary(cmdName, binDir)
	if err != nil {
		return err
	}
	mindspecPath, _ := findRealBinary("mindspec", binDir)
	origPath := pathWithout(binDir)
	script := fmt.Sprintf(shimScriptPinCWD, logPath, realPath, cmdName, pinCWD, mindspecPath, origPath)
	shimPath := filepath.Join(binDir, cmdName)
	if err := os.WriteFile(shimPath, []byte(script), 0o755); err != nil {
		return err
	}
	return nil
}

// findRealBinary locates the real binary for cmdName, excluding excludeDir.
func findRealBinary(cmdName, excludeDir string) (string, error) {
	pathEnv := os.Getenv("PATH")
	absExclude, _ := filepath.Abs(excludeDir)

	for _, dir := range filepath.SplitList(pathEnv) {
		absDir, _ := filepath.Abs(dir)
		if absDir == absExclude {
			continue
		}
		candidate := filepath.Join(dir, cmdName)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found in PATH", cmdName)
}

// pathWithout returns the current PATH with binDir removed.
// Used to give phase queries a clean PATH that bypasses shims.
func pathWithout(binDir string) string {
	absExclude, _ := filepath.Abs(binDir)
	var dirs []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		absDir, _ := filepath.Abs(dir)
		if absDir != absExclude {
			dirs = append(dirs, dir)
		}
	}
	return strings.Join(dirs, string(filepath.ListSeparator))
}

// ShimEnv returns environment variables that prepend binDir to PATH.
func ShimEnv(binDir string) []string {
	abs, _ := filepath.Abs(binDir)
	return []string{
		fmt.Sprintf("PATH=%s:%s", abs, os.Getenv("PATH")),
	}
}
