package harness

import (
	"fmt"
	"os"
	"path/filepath"
)

// shimScript is the template for a recording shim. It logs the invocation
// to a JSONL file, then delegates to the real binary.
// Placeholders: %s = log path, %s = real binary path, %s = command name.
const shimScript = `#!/bin/sh
# Recording shim for %[3]s — logs invocations to JSONL before delegating.
LOG_PATH="%[1]s"
REAL_BIN="%[2]s"
CMD_NAME="%[3]s"
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
printf '{"timestamp":"%%s","action_type":"command","command":"%[3]s","args_list":%%s,"cwd":"%%s","exit_code":%%d,"duration_ms":%%d}\n' \
  "$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ 2>/dev/null || echo unknown)" \
  "$ARGS" \
  "$CWD_PATH" \
  "$EXIT_CODE" \
  "$DURATION_MS" \
  >> "$LOG_PATH"
exit $EXIT_CODE
`

// DefaultShimCommands is the list of commands that get recording shims.
var DefaultShimCommands = []string{"mindspec", "git", "bd", "gh"}

// InstallShims creates recording shim scripts in binDir for each command.
// Each shim logs to logPath and delegates to the real binary found via PATH
// (excluding binDir itself).
func InstallShims(binDir, logPath string) error {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("creating shim dir: %w", err)
	}

	for _, cmd := range DefaultShimCommands {
		realPath, err := findRealBinary(cmd, binDir)
		if err != nil {
			// If the real binary isn't found, skip (e.g. bd not installed in test env)
			continue
		}
		if err := writeShim(binDir, logPath, cmd, realPath); err != nil {
			return fmt.Errorf("writing shim for %s: %w", cmd, err)
		}
	}
	return nil
}

// writeShim creates a single shim script.
func writeShim(binDir, logPath, cmdName, realPath string) error {
	script := fmt.Sprintf(shimScript, logPath, realPath, cmdName)
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

// ShimEnv returns environment variables that prepend binDir to PATH.
func ShimEnv(binDir string) []string {
	abs, _ := filepath.Abs(binDir)
	return []string{
		fmt.Sprintf("PATH=%s:%s", abs, os.Getenv("PATH")),
	}
}
