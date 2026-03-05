package hook

import (
	"os"
	"strings"
)

// dirExists returns true if the path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// hasPathPrefix checks if path starts with prefix, handling trailing slashes.
func hasPathPrefix(path, prefix string) bool {
	if prefix == "" {
		return false
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return path == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(path, prefix)
}

// stripEnvPrefixes removes leading VAR=val prefixes from a command string.
func stripEnvPrefixes(cmd string) string {
	_, stripped := parseEnvPrefixes(cmd)
	return stripped
}

// parseEnvPrefixes parses and removes leading VAR=val prefixes from a command.
func parseEnvPrefixes(cmd string) (map[string]string, string) {
	env := map[string]string{}
	stripped := strings.TrimSpace(cmd)
	for {
		idx := strings.Index(stripped, " ")
		if idx < 0 {
			break
		}
		prefix := stripped[:idx]
		eqIdx := strings.Index(prefix, "=")
		if eqIdx < 0 {
			break
		}
		varName := prefix[:eqIdx]
		if !isEnvVarName(varName) {
			break
		}
		env[varName] = prefix[eqIdx+1:]
		stripped = strings.TrimSpace(stripped[idx+1:])
	}
	return env, stripped
}

func isEnvVarName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// getCwd returns the current working directory.
var getCwd = os.Getwd
