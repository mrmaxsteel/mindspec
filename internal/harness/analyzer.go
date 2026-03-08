package harness

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// TurnClass categorizes an agent turn.
type TurnClass string

const (
	ClassForward     TurnClass = "forward"      // Productive work advancing the task
	ClassRetry       TurnClass = "retry"        // Re-running a previously failed command
	ClassCorrection  TurnClass = "correction"   // Fixing a recent mistake (same file within 2 turns)
	ClassRecovery    TurnClass = "recovery"     // Recovering from a hook block
	ClassWrongAction TurnClass = "wrong_action" // Violated a workflow rule and was NOT blocked
	ClassOverhead    TurnClass = "overhead"     // Read/search without subsequent write
)

// TurnSummary describes a single agent turn.
type TurnSummary struct {
	Turn   int
	Class  TurnClass
	Events []ActionEvent
	Reason string // explanation for classification
}

// WrongActionRule checks if an event violates a workflow rule.
// Returns (violated, reason).
type WrongActionRule struct {
	Name  string
	Check func(ActionEvent) (bool, string)
}

// DefaultWrongActionRules returns the standard set of wrong-action detectors.
func DefaultWrongActionRules() []WrongActionRule {
	return []WrongActionRule{
		{
			Name: "code_in_spec_mode",
			Check: func(e ActionEvent) (bool, string) {
				if e.Phase != "spec" {
					return false, ""
				}
				if e.ActionType != "command" && e.ActionType != "tool_invoke" {
					return false, ""
				}
				if e.ToolName == "Write" || e.ToolName == "Edit" {
					path := e.Args["file_path"]
					if path != "" && isCodePath(path) {
						return true, "code edit in spec mode: " + path
					}
				}
				return false, ""
			},
		},
		{
			Name: "code_in_plan_mode",
			Check: func(e ActionEvent) (bool, string) {
				if e.Phase != "plan" {
					return false, ""
				}
				if e.ToolName == "Write" || e.ToolName == "Edit" {
					path := e.Args["file_path"]
					if path != "" && isCodePath(path) {
						return true, "code edit in plan mode: " + path
					}
				}
				return false, ""
			},
		},
		{
			Name: "commit_to_main",
			Check: func(e ActionEvent) (bool, string) {
				if e.Command != "git" {
					return false, ""
				}
				args := flatArgs(e.Args)
				if containsAll(args, "commit") && containsCWDMain(e.CWD) {
					return true, "git commit on main branch"
				}
				return false, ""
			},
		},
		// skip_next and skip_complete are session-level checks, not per-event.
		// They are implemented in detectSkipNext() and detectSkipComplete()
		// and called from DetectWrongActions().
		{
			Name: "wrong_cwd",
			Check: func(e ActionEvent) (bool, string) {
				if e.Command == "" || e.CWD == "" {
					return false, ""
				}
				// If there's an active worktree but CWD is the main repo
				if e.Phase == "implement" && containsCWDMain(e.CWD) {
					// Only flag for code-modifying commands
					if e.Command == "git" && containsAll(flatArgs(e.Args), "commit") {
						return true, "command executed from main repo instead of worktree"
					}
				}
				return false, ""
			},
		},
		{
			Name: "force_bypass",
			Check: func(e ActionEvent) (bool, string) {
				if e.Command != "mindspec" {
					return false, ""
				}
				args := flatArgs(e.Args)
				if containsAll(args, "--force") || containsAll(args, "--no-verify") {
					return true, "force bypass used: " + strings.Join(args, " ")
				}
				return false, ""
			},
		},
		// bd_close_shortcut is a session-level check (see detectBdCloseShortcut),
		// not per-event, because the agent may use bd close during exploration
		// before ultimately running mindspec complete.
	}
}

// Analyzer classifies agent turns and detects wrong actions.
type Analyzer struct {
	Rules []WrongActionRule
}

// NewAnalyzer creates an analyzer with default rules.
func NewAnalyzer() *Analyzer {
	return &Analyzer{Rules: DefaultWrongActionRules()}
}

// Classify groups events by turn and classifies each turn.
// If events have Turn=0 (shims don't know API turns), turns are estimated
// from timestamp gaps: a gap > 2s between consecutive events signals a new turn.
func (a *Analyzer) Classify(events []ActionEvent) []TurnSummary {
	// Assign turn numbers if not already set by the event source.
	assignTurns(events)

	log := NewEventLog(events)
	byTurn := log.ByTurn()

	// Track files written per turn for correction detection
	writtenFiles := make(map[int]map[string]bool) // turn -> set of file paths
	// Track failed commands for retry detection: cmdKey -> true
	failedCmds := make(map[string]bool)

	var summaries []TurnSummary
	maxTurn := log.MaxTurn()

	for turn := 0; turn <= maxTurn; turn++ {
		turnEvents, ok := byTurn[turn]
		if !ok {
			continue
		}

		writtenFiles[turn] = make(map[string]bool)
		for _, e := range turnEvents {
			if e.ToolName == "Write" || e.ToolName == "Edit" {
				if path, ok := e.Args["file_path"]; ok {
					writtenFiles[turn][path] = true
				}
			}
		}

		class, reason := a.classifyTurn(turn, turnEvents, writtenFiles, failedCmds)
		summaries = append(summaries, TurnSummary{
			Turn:   turn,
			Class:  class,
			Events: turnEvents,
			Reason: reason,
		})

		// Record failed commands from this turn for future retry detection
		for _, e := range turnEvents {
			if e.Command != "" && e.ExitCode != 0 && !isInfrastructureCmd(e) {
				failedCmds[cmdKey(e)] = true
			}
		}
	}

	return summaries
}

// assignTurns estimates API turn numbers from event timestamps when the Turn
// field is unset (all zeros). Commands executed within 2 seconds of each other
// are grouped into the same turn (the LLM typically takes 2+ seconds between
// turns for thinking).
func assignTurns(events []ActionEvent) {
	if len(events) == 0 {
		return
	}
	// If any event already has a non-zero turn, assume turns are pre-assigned.
	for _, e := range events {
		if e.Turn != 0 {
			return
		}
	}

	const turnGap = 2 * time.Second
	turn := 1
	events[0].Turn = turn
	for i := 1; i < len(events); i++ {
		gap := events[i].Timestamp.Sub(events[i-1].Timestamp)
		if gap > turnGap {
			turn++
		}
		events[i].Turn = turn
	}
}

func (a *Analyzer) classifyTurn(turn int, events []ActionEvent, writtenFiles map[int]map[string]bool, failedCmds map[string]bool) (TurnClass, string) {
	// Check for recovery: any blocked event means the agent was recovering
	for _, e := range events {
		if e.Blocked {
			return ClassRecovery, "hook blocked: " + e.BlockReason
		}
	}

	// Check for wrong actions (rules that fire and event was NOT blocked)
	for _, e := range events {
		for _, rule := range a.Rules {
			if violated, reason := rule.Check(e); violated {
				return ClassWrongAction, reason
			}
		}
	}

	// Check for retry: re-running a command that previously failed
	for _, e := range events {
		if e.Command != "" && !isInfrastructureCmd(e) && failedCmds[cmdKey(e)] {
			return ClassRetry, "retrying previously failed: " + e.Command + " " + strings.Join(e.ArgsList, " ")
		}
	}

	// Check for correction: writing to a file that was written in the last 2 turns
	for _, e := range events {
		if e.ToolName == "Write" || e.ToolName == "Edit" {
			path := e.Args["file_path"]
			if path == "" {
				continue
			}
			for lookback := 1; lookback <= 2; lookback++ {
				prevTurn := turn - lookback
				if prevTurn < 0 {
					continue
				}
				if prevFiles, ok := writtenFiles[prevTurn]; ok && prevFiles[path] {
					return ClassCorrection, "re-editing " + filepath.Base(path) + " within 2 turns"
				}
			}
		}
	}

	// Check for overhead: only read/search operations, no writes
	hasWrite := false
	hasCommand := false
	for _, e := range events {
		switch e.ActionType {
		case "command":
			hasCommand = true
		case "tool_invoke":
			if e.ToolName == "Write" || e.ToolName == "Edit" || e.ToolName == "NotebookEdit" {
				hasWrite = true
			}
		}
	}
	if !hasWrite && !hasCommand {
		return ClassOverhead, "read-only operations"
	}

	return ClassForward, ""
}

// DetectWrongActions scans all events for wrong-action violations.
// Includes both per-event rule checks and session-level pattern detection
// (skip_next, skip_complete).
func (a *Analyzer) DetectWrongActions(events []ActionEvent) []WrongActionResult {
	var results []WrongActionResult
	for _, e := range events {
		if e.Blocked {
			continue // blocked events are handled by hooks, not wrong actions
		}
		for _, rule := range a.Rules {
			if violated, reason := rule.Check(e); violated {
				results = append(results, WrongActionResult{
					Rule:   rule.Name,
					Event:  e,
					Reason: reason,
				})
			}
		}
	}

	// Session-level checks that require scanning the full event stream.
	results = append(results, detectSkipNext(events)...)
	results = append(results, detectSkipComplete(events)...)
	results = append(results, detectBdCloseShortcut(events)...)

	return results
}

// detectSkipNext checks if the agent wrote code before running `mindspec next`.
// Returns a wrong action if code-modifying events appear before any `mindspec next`
// AND the phase is not already `implement` (which implies a bead was pre-claimed).
func detectSkipNext(events []ActionEvent) []WrongActionResult {
	// Early bail-out: if this session never enters implement phase and never
	// runs `mindspec next`, then skip_next is irrelevant (e.g. spec-only or
	// plan-only sessions where commits are legitimate lifecycle artifacts).
	hasImplement := false
	hasNextOrApprove := false
	for _, e := range events {
		if e.Phase == "implement" {
			hasImplement = true
		}
		if e.Command == "mindspec" {
			args := eventArgsList(e)
			if containsAll(args, "next") || containsAll(args, "approve") {
				hasNextOrApprove = true
			}
		}
	}
	if !hasImplement && !hasNextOrApprove {
		return nil
	}

	// Build a set of turns that contain lifecycle commands. Git commits in
	// these turns are side-effects of the lifecycle operation (e.g. approve
	// auto-commits state changes) and should not count as "agent wrote code."
	lifecycleTurns := lifecycleTurnSet(events)

	// Find the last approve command index. Code modifications before an
	// approve are part of the approval flow (e.g. updating a spec file
	// before approve) and don't require `mindspec next`. But code
	// modifications AFTER the last approve still need `next`.
	lastApproveIdx := -1
	for i, e := range events {
		if e.Command == "mindspec" && containsAll(eventArgsList(e), "approve") {
			lastApproveIdx = i
		}
	}

	for i, e := range events {
		if e.Blocked {
			continue
		}
		// If we see `mindspec next` first, no violation.
		if e.Command == "mindspec" && containsAll(eventArgsList(e), "next") {
			return nil
		}
		// Code modification before next — but only if not already in implement
		// phase (setup may have pre-claimed a bead) and not before an approve
		// command (approval flows don't require next).
		if isCodeModifyingEvent(e) && e.Phase != "implement" {
			if lastApproveIdx >= 0 && i <= lastApproveIdx {
				continue // code edit is part of the approval flow
			}
			// Git commits in the same turn as a lifecycle command are
			// side-effects, not agent implementation code.
			if e.Command == "git" && lifecycleTurns[e.Turn] {
				continue
			}
			return []WrongActionResult{{
				Rule:   "skip_next",
				Event:  e,
				Reason: "code modification before mindspec next: " + describeEvent(e),
			}}
		}
	}
	return nil
}

// lifecycleTurnSet returns the set of turn numbers that contain a mindspec
// lifecycle command (spec-init, approve, complete). Commits in these turns
// are side-effects of the lifecycle operation, not agent code.
func lifecycleTurnSet(events []ActionEvent) map[int]bool {
	turns := make(map[int]bool)
	lifecycleVerbs := []string{"approve", "spec-init", "complete"}
	for _, e := range events {
		if e.Command != "mindspec" {
			continue
		}
		args := eventArgsList(e)
		for _, verb := range lifecycleVerbs {
			if containsAll(args, verb) {
				turns[e.Turn] = true
				break
			}
		}
	}
	return turns
}

// detectSkipComplete checks if the agent ran `mindspec next` and wrote code
// but never ran `mindspec complete` before the session ended or the next
// lifecycle command ran.
func detectSkipComplete(events []ActionEvent) []WrongActionResult {
	nextSeen := false
	codeAfterNext := false
	var firstCodeEvent ActionEvent

	for _, e := range events {
		if e.Blocked {
			continue
		}
		args := eventArgsList(e)

		if e.Command == "mindspec" && containsAll(args, "next") {
			nextSeen = true
			codeAfterNext = false
			continue
		}

		if nextSeen && !codeAfterNext && isCodeModifyingEvent(e) {
			codeAfterNext = true
			firstCodeEvent = e
		}

		// `mindspec complete` resets the cycle.
		if e.Command == "mindspec" && containsAll(args, "complete") {
			nextSeen = false
			codeAfterNext = false
			continue
		}

		// A lifecycle command after code without complete is a violation.
		if codeAfterNext && e.Command == "mindspec" {
			if containsAll(args, "approve") || containsAll(args, "next") {
				return []WrongActionResult{{
					Rule:   "skip_complete",
					Event:  firstCodeEvent,
					Reason: "code written after mindspec next but mindspec complete not run before " + e.Command + " " + strings.Join(args, " "),
				}}
			}
		}
	}

	// If session ended with code after next but no complete, that's a violation.
	if codeAfterNext {
		return []WrongActionResult{{
			Rule:   "skip_complete",
			Event:  firstCodeEvent,
			Reason: "code written after mindspec next but mindspec complete never ran",
		}}
	}
	return nil
}

// detectBdCloseShortcut checks if the agent used `bd close` to close a bead
// without ever running `mindspec complete`. If `mindspec complete` ran
// successfully at any point, `bd close` calls are tolerated (the agent
// explored before finding the right command).
func detectBdCloseShortcut(events []ActionEvent) []WrongActionResult {
	var bdCloseEvents []ActionEvent
	completeSeen := false

	for _, e := range events {
		if e.Blocked {
			continue
		}
		if e.Command == "bd" && e.ExitCode == 0 && containsAll(eventArgsList(e), "close") {
			bdCloseEvents = append(bdCloseEvents, e)
		}
		if e.Command == "mindspec" && e.ExitCode == 0 && containsAll(eventArgsList(e), "complete") {
			completeSeen = true
		}
	}

	if completeSeen || len(bdCloseEvents) == 0 {
		return nil
	}

	return []WrongActionResult{{
		Rule:   "bd_close_shortcut",
		Event:  bdCloseEvents[0],
		Reason: "used bd close instead of mindspec complete — skips merge, cleanup, and state transitions",
	}}
}

// isCodeModifyingEvent returns true if the event represents a code modification.
func isCodeModifyingEvent(e ActionEvent) bool {
	if e.ToolName == "Write" || e.ToolName == "Edit" {
		path := e.Args["file_path"]
		if path != "" && isCodePath(path) {
			return true
		}
	}
	if e.Command == "git" && e.ExitCode == 0 {
		args := eventArgsList(e)
		if containsAll(args, "commit") {
			return true
		}
	}
	return false
}

// eventArgsList returns args as a list, using ArgsList if available, else flatArgs.
func eventArgsList(e ActionEvent) []string {
	if len(e.ArgsList) > 0 {
		return e.ArgsList
	}
	return flatArgs(e.Args)
}

// describeEvent returns a short description of an event for error messages.
func describeEvent(e ActionEvent) string {
	if e.ToolName != "" {
		path := e.Args["file_path"]
		if path != "" {
			return e.ToolName + " " + path
		}
		return e.ToolName
	}
	if e.Command != "" {
		return e.Command + " " + strings.Join(eventArgsList(e), " ")
	}
	return "unknown event"
}

// WrongActionResult captures a specific rule violation.
type WrongActionResult struct {
	Rule   string
	Event  ActionEvent
	Reason string
}

// PlanFidelity calculates how well the agent followed the plan.
// Returns a score between 0.0 and 1.0.
func PlanFidelity(planPath string, events []ActionEvent) (float64, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return 0, err
	}

	content := string(data)
	expectedPaths := extractMentionedPaths(content)
	expectedCommands := extractMentionedCommands(content)

	if len(expectedPaths)+len(expectedCommands) == 0 {
		return 1.0, nil // no expectations means perfect fidelity
	}

	total := len(expectedPaths) + len(expectedCommands)
	matched := 0

	// Check which expected paths were touched
	touchedPaths := make(map[string]bool)
	for _, e := range events {
		if e.ToolName == "Write" || e.ToolName == "Edit" || e.ToolName == "Read" {
			if path, ok := e.Args["file_path"]; ok {
				touchedPaths[path] = true
				// Also match by basename
				touchedPaths[filepath.Base(path)] = true
			}
		}
	}
	for _, p := range expectedPaths {
		if touchedPaths[p] || touchedPaths[filepath.Base(p)] {
			matched++
		}
	}

	// Check which expected commands were run
	ranCommands := make(map[string]bool)
	for _, e := range events {
		if e.Command != "" {
			ranCommands[e.Command] = true
			// Also match with first arg
			args := flatArgs(e.Args)
			if len(args) > 0 {
				ranCommands[e.Command+" "+args[0]] = true
			}
		}
	}
	for _, cmd := range expectedCommands {
		if ranCommands[cmd] {
			matched++
		}
	}

	return float64(matched) / float64(total), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func isCodePath(path string) bool {
	docPrefixes := []string{".mindspec/", "docs/", ".claude/", ".github/"}
	for _, prefix := range docPrefixes {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}
	if strings.HasSuffix(path, ".md") {
		return false
	}
	return true
}

func flatArgs(args map[string]string) []string {
	if args == nil {
		return nil
	}
	var result []string
	for i := 0; i < len(args); i++ {
		key := strconv.Itoa(i)
		if v, ok := args[key]; ok {
			result = append(result, v)
		}
	}
	// Also include named args
	for k, v := range args {
		if k == "file_path" || k == "command" {
			result = append(result, v)
		}
	}
	return result
}

func containsAll(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// cmdKey returns a stable key for a command+args pair, used to detect retries.
// Uses command name + first meaningful arg (e.g. "mindspec complete", "git commit").
func cmdKey(e ActionEvent) string {
	if len(e.ArgsList) > 0 {
		return e.Command + " " + e.ArgsList[0]
	}
	args := flatArgs(e.Args)
	if len(args) > 0 {
		return e.Command + " " + args[0]
	}
	return e.Command
}

// isInfrastructureCmd returns true for Claude Code internal git operations that
// run automatically every turn (upstream checks, sparse checkout probes, etc.).
// These should not count as agent-initiated retries when they fail.
func isInfrastructureCmd(e ActionEvent) bool {
	if e.Command != "git" && e.Command != "bd" {
		return false
	}
	args := strings.Join(e.ArgsList, " ")
	infraPatterns := []string{
		"config --get core.sparseCheckout",
		"rev-parse --abbrev-ref --symbolic-full-name @{u}",
		"--no-optional-locks status",
		"--no-optional-locks log",
		"-c credential.helper= pull",
		"rev-parse --git-dir",
		"rev-parse --git-common-dir",
	}
	for _, pat := range infraPatterns {
		if strings.Contains(args, pat) {
			return true
		}
	}
	// bd prime is infrastructure (context recovery)
	if e.Command == "bd" && containsAll(e.ArgsList, "prime") {
		return true
	}
	return false
}

func containsCWDMain(cwd string) bool {
	// Heuristic: if CWD does NOT contain ".worktrees/" it's likely the main repo
	return cwd != "" && !strings.Contains(cwd, ".worktrees/")
}

func extractMentionedPaths(content string) []string {
	var paths []string
	seen := make(map[string]bool)

	for _, line := range strings.Split(content, "\n") {
		// Look for file-like references (containing / and a file extension)
		words := strings.Fields(line)
		for _, w := range words {
			w = strings.Trim(w, "`\"'(),[]")
			if strings.Contains(w, "/") && (strings.Contains(w, ".go") ||
				strings.Contains(w, ".ts") || strings.Contains(w, ".js") ||
				strings.Contains(w, ".py") || strings.Contains(w, ".yaml") ||
				strings.Contains(w, ".yml") || strings.Contains(w, ".md") ||
				strings.Contains(w, ".json")) {
				if !seen[w] {
					paths = append(paths, w)
					seen[w] = true
				}
			}
		}
	}
	return paths
}

func extractMentionedCommands(content string) []string {
	var commands []string
	seen := make(map[string]bool)

	knownCmds := []string{"mindspec", "git", "go test", "make"}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, cmd := range knownCmds {
			if strings.Contains(trimmed, cmd) {
				// Extract the command reference
				if !seen[cmd] {
					commands = append(commands, cmd)
					seen[cmd] = true
				}
			}
		}
	}
	return commands
}
