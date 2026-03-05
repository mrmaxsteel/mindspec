// Package hook implements hook logic for Claude Code, Copilot, and git hooks.
// Each hook reads context from stdin or environment, checks workflow state,
// and emits a response in the caller's protocol format.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mindspec/mindspec/internal/phase"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
)

// Protocol identifies the calling agent platform.
type Protocol int

const (
	ProtocolAuto    Protocol = iota
	ProtocolClaude           // Claude Code: tool_input.file_path, stderr+exit2 for block
	ProtocolCopilot          // Copilot: toolArgs.file_path, permissionDecision JSON
)

// Action is the hook's decision.
type Action int

const (
	Pass  Action = iota // silent exit 0
	Block               // hard deny
	Warn                // advisory context (non-blocking)
)

// Result is the outcome of a hook evaluation.
type Result struct {
	Action  Action
	Message string
}

// Input holds normalized tool invocation fields parsed from stdin.
type Input struct {
	FilePath string // from tool_input.file_path or toolArgs.file_path/path
	Command  string // from tool_input.command or toolArgs.command
	Raw      map[string]any
}

// Names lists all registered hook names.
var Names = []string{
	"pre-commit",
	"session-start",
}

// ParseInput reads stdin JSON and auto-detects the protocol.
func ParseInput(r io.Reader) (*Input, Protocol, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return &Input{}, ProtocolClaude, fmt.Errorf("reading stdin: %w", err)
	}
	if len(data) == 0 {
		return &Input{}, ProtocolClaude, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return &Input{}, ProtocolClaude, nil // graceful: treat as empty
	}

	inp := &Input{Raw: raw}
	proto := detectProtocol(raw)

	switch proto {
	case ProtocolClaude:
		if ti, ok := raw["tool_input"].(map[string]any); ok {
			inp.FilePath, _ = ti["file_path"].(string)
			inp.Command, _ = ti["command"].(string)
		}
	case ProtocolCopilot:
		if ta, ok := raw["toolArgs"].(map[string]any); ok {
			fp, _ := ta["file_path"].(string)
			if fp == "" {
				fp, _ = ta["path"].(string)
			}
			inp.FilePath = fp
			inp.Command, _ = ta["command"].(string)
		}
	}

	return inp, proto, nil
}

func detectProtocol(raw map[string]any) Protocol {
	if _, ok := raw["tool_input"]; ok {
		return ProtocolClaude
	}
	if _, ok := raw["toolName"]; ok {
		return ProtocolCopilot
	}
	if _, ok := raw["toolArgs"]; ok {
		return ProtocolCopilot
	}
	return ProtocolClaude // default
}

// Emit writes the result to stdout/stderr and returns the exit code.
func Emit(r Result, p Protocol) int {
	if r.Action == Pass {
		return 0
	}

	switch p {
	case ProtocolCopilot:
		return emitCopilot(r)
	default:
		return emitClaude(r)
	}
}

func emitClaude(r Result) int {
	switch r.Action {
	case Block:
		fmt.Fprintln(os.Stderr, r.Message)
		return 2
	case Warn:
		out, _ := json.Marshal(map[string]string{"additionalContext": r.Message})
		fmt.Println(string(out))
		return 0
	default:
		return 0
	}
}

func emitCopilot(r Result) int {
	var resp map[string]string
	switch r.Action {
	case Block:
		resp = map[string]string{
			"permissionDecision":       "deny",
			"permissionDecisionReason": r.Message,
		}
	case Warn:
		resp = map[string]string{
			"permissionDecision":       "allow",
			"permissionDecisionReason": r.Message,
		}
	default:
		return 0
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
	return 0
}

// ReadState constructs a HookState from beads phase context and session.json.
// Returns nil (not error) if state cannot be determined.
func ReadState() *HookState {
	root, err := workspace.FindLocalRoot(".")
	if err != nil {
		return nil
	}

	hs := &HookState{}

	ctx, ctxErr := phase.ResolveContext(root)
	if ctxErr == nil && ctx != nil {
		hs.Mode = ctx.Phase
		hs.ActiveSpec = ctx.SpecID
		if ctx.BeadID != "" && ctx.SpecID != "" {
			specWt := state.SpecWorktreePath(root, ctx.SpecID)
			wt := state.BeadWorktreePath(specWt, ctx.BeadID)
			if dirExists(wt) {
				hs.ActiveWorktree = wt
			}
		} else if ctx.SpecID != "" {
			wt := state.SpecWorktreePath(root, ctx.SpecID)
			if dirExists(wt) {
				hs.ActiveWorktree = wt
			}
		}
	}

	sess, err := state.ReadSession(root)
	if err == nil {
		hs.SessionSource = sess.SessionSource
		hs.SessionStartedAt = sess.SessionStartedAt
		hs.BeadClaimedAt = sess.BeadClaimedAt
	}

	if hs.Mode == "" && hs.SessionStartedAt == "" {
		return nil
	}

	return hs
}
