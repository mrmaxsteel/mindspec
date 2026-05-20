package recording

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/agentmind/client"
)

// StartCollector launches AgentMind as a detached background process
// to collect OTLP telemetry and write NDJSON to the spec's recording directory.
//
// Spec 083 Bead 3a: rewired to call `client.AutoStart` (from the
// extracted agentmind module). Per Hard Constraint #4
// (telemetry-as-output class), an absent binary is a hard error here:
// the recording IS the deliverable. The error wraps
// `client.ErrBinaryNotFound` so upstream commands (e.g.
// `mindspec record start`) can detect the condition via
// `errors.Is(err, client.ErrBinaryNotFound)` and exit non-zero with
// the canonical warn line.
func StartCollector(root, specID string) error {
	eventsPath, err := EventsPath(root, specID)
	if err != nil {
		return err
	}

	handle, err := client.AutoStart(root, client.DefaultOTLPPort, client.DefaultUIPort, eventsPath)
	if err != nil {
		// Telemetry-as-output class: every AutoStart failure (including
		// the typed client.ErrBinaryNotFound sentinel) is propagated to
		// the command-level handler. The %w wrapping preserves
		// errors.Is(err, client.ErrBinaryNotFound) detection upstream so
		// the handler can call client.EmitWarnOnce alongside the non-zero
		// exit. No branching needed here — both arms previously returned
		// the same wrapped error, which was structurally dead code (panel
		// bead-3a-v1, REV-3).
		return fmt.Errorf("starting AgentMind collector: %w", err)
	}

	pid := 0
	if handle != nil {
		pid = handle.PID
	}

	// Update manifest with PID, port, and process name for later verification
	m, err := ReadManifest(root, specID)
	if err != nil {
		return fmt.Errorf("reading manifest for PID update: %w", err)
	}
	m.CollectorPID = pid
	m.CollectorPort = client.DefaultOTLPPort
	m.Status = "recording"

	// Record the expected process name for PID verification on stop
	binPath, _ := os.Executable()
	m.ProcessName = filepath.Base(binPath)

	if err := WriteManifest(root, specID, m); err != nil {
		return fmt.Errorf("writing manifest with PID: %w", err)
	}

	return nil
}

// StopCollector sends SIGTERM to the AgentMind process after verifying its identity,
// then updates the manifest. Fails closed if the process cannot be verified.
func StopCollector(root, specID string) error {
	m, err := ReadManifest(root, specID)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	if m.CollectorPID > 0 {
		if m.CollectorPID <= 0 {
			return fmt.Errorf("invalid collector PID: %d", m.CollectorPID)
		}

		// Check process is still alive
		if !isProcessAlive(m.CollectorPID) {
			// Process already gone — mark as complete without signaling
			m.Status = "complete"
			m.CollectorPID = 0
			return closePhases(m, root, specID)
		}

		// Verify process identity if we have an expected name
		if m.ProcessName != "" {
			name, err := processName(m.CollectorPID)
			if err != nil {
				m.Status = "stale"
				_ = WriteManifest(root, specID, m)
				return fmt.Errorf("cannot verify PID %d identity: %w (manifest marked stale)", m.CollectorPID, err)
			}
			if !strings.Contains(name, "mindspec") {
				m.Status = "stale"
				_ = WriteManifest(root, specID, m)
				return fmt.Errorf("PID %d is %q, expected %q (manifest marked stale, refusing to signal)", m.CollectorPID, name, m.ProcessName)
			}
		}

		proc, err := os.FindProcess(m.CollectorPID)
		if err != nil {
			return fmt.Errorf("finding process %d: %w", m.CollectorPID, err)
		}
		if err := signalTerminate(proc); err != nil {
			return fmt.Errorf("signaling process %d: %w", m.CollectorPID, err)
		}
		// Give it a moment to shut down gracefully
		time.Sleep(500 * time.Millisecond)
	}

	m.Status = "complete"
	m.CollectorPID = 0
	return closePhases(m, root, specID)
}

func closePhases(m *Manifest, root, specID string) error {
	if len(m.Phases) > 0 {
		last := &m.Phases[len(m.Phases)-1]
		if last.EndedAt == "" {
			last.EndedAt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	return WriteManifest(root, specID, m)
}
