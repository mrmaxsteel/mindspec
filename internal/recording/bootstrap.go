package recording

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const defaultRecordingPort = 4319

// EnsureOTLP checks .claude/settings.local.json for OTLP env vars.
// If not present, adds them with the recording collector port (4319).
// If already present with a different endpoint, warns and does not override.
// Returns true if new config was written (first-run), false otherwise.
func EnsureOTLP(root string) (bool, error) {
	settingsPath := filepath.Join(root, ".claude", "settings.local.json")

	// Read existing settings
	var settings map[string]any
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("reading settings.local.json: %w", err)
		}
		settings = make(map[string]any)
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, fmt.Errorf("parsing settings.local.json: %w", err)
		}
	}

	// Check existing env block
	envRaw, hasEnv := settings["env"]
	var env map[string]any
	if hasEnv {
		env, _ = envRaw.(map[string]any)
	}
	if env == nil {
		env = make(map[string]any)
	}

	// If OTLP endpoint is already set, check if it's ours
	expectedEndpoint := fmt.Sprintf("http://localhost:%d", defaultRecordingPort)
	if existing, ok := env["OTEL_EXPORTER_OTLP_ENDPOINT"].(string); ok {
		if existing == expectedEndpoint {
			return false, nil // already configured correctly
		}
		fmt.Fprintf(os.Stderr, "warning: OTLP endpoint already set to %q (expected %q) — not overriding\n",
			existing, expectedEndpoint)
		return false, nil
	}

	// Write OTLP config
	env["CLAUDE_CODE_ENABLE_TELEMETRY"] = "1"
	env["OTEL_METRICS_EXPORTER"] = "otlp"
	env["OTEL_LOGS_EXPORTER"] = "otlp"
	env["OTEL_EXPORTER_OTLP_PROTOCOL"] = "http/json"
	env["OTEL_EXPORTER_OTLP_ENDPOINT"] = expectedEndpoint
	settings["env"] = env

	// Write back
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return false, fmt.Errorf("creating .claude dir: %w", err)
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshaling settings: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		return false, fmt.Errorf("writing settings.local.json: %w", err)
	}

	return true, nil
}
