package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage MindSpec workflow state",
	Long:  `Read and write the focus cursor that tracks current mode and active work.`,
}

var stateSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the current mode and active work (deprecated)",
	Long:  `Deprecated: lifecycle state is now derived from beads (ADR-0023). This command is a no-op.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("state set is deprecated: lifecycle state is now derived from beads (ADR-0023)")
		return nil
	},
}

// stateOutput is the JSON output format for state show.
type stateOutput struct {
	Mode       string `json:"mode"`
	ActiveSpec string `json:"activeSpec,omitempty"`
	ActiveBead string `json:"activeBead,omitempty"`
	SpecBranch string `json:"specBranch,omitempty"`
}

// stateDiscoveryFailure formats an R1.4/ADR-0035 refusal for `state show`
// naming the failed lifecycle-discovery query. `state show` is a read-only
// query: a discovery failure fails loudly rather than emitting a second
// "indeterminate" success shape (that form is reserved for the instruct
// tail, which must not fail the verb it trails — OQ-2's resolution).
//
// Final-review round 1 (R5/G3): err chains bd-derived text (AllEpics /
// ResolveMode -> wrapBDError's raw stderr excerpt) — escape via
// next.RenderBDError at this render sink.
func stateDiscoveryFailure(query string, err error) error {
	return guard.NewFailure(
		fmt.Sprintf("state show: %s failed: %v", query, next.RenderBDError(err)),
		"mindspec doctor",
	)
}

var stateShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current MindSpec state",
	Long: `Show the current MindSpec state derived from beads.
Use --spec to target a specific spec.
If multiple active specs exist and no --spec is given, shows the ambiguity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		specFlag, _ := cmd.Flags().GetString("spec")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}

		root, err := workspace.FindLocalRoot(cwd)
		if err != nil {
			return err
		}

		// R3.5: one phase.Cache threaded through the whole invocation
		// instead of a fresh cache per call site.
		c := phase.NewCache()

		// ADR-0023 + OQ-2: derive state from beads, plus the filesystem-
		// scoped pre-epic draft carve-out — never from focus files.
		specID, resolveErr := resolve.ResolveTargetWithCache(c, root, specFlag)
		if resolveErr != nil {
			if ambErr, ok := resolveErr.(*resolve.ErrAmbiguousTarget); ok {
				return ambErr
			}

			// Round-1 panel G2/F2b: phase.Cache does not memoize errors,
			// so this failure may be a transient bd flake rather than a
			// genuine "nothing active" result — probe bd health directly,
			// and if it recovers, RE-RUN target resolution against the
			// now-warm cache before drawing any conclusion. Without this,
			// a resolution that would have succeeded on retry was
			// silently read as "bd healthy, nothing active" instead
			// (the same discipline resolve.ResolveModeWithCache applies
			// internally for its own epic lookup, G2/F2a).
			if _, epicsErr := c.AllEpics(); epicsErr != nil {
				return stateDiscoveryFailure("querying beads for active specs", epicsErr)
			}
			specID, resolveErr = resolve.ResolveTargetWithCache(c, root, specFlag)
		}

		if resolveErr != nil {
			if ambErr, ok := resolveErr.(*resolve.ErrAmbiguousTarget); ok {
				return ambErr
			}

			// Round-1 panel G1 (R1/R4/R5/R6 four-slot convergence): an
			// explicit --spec must never be answered with a DIFFERENT
			// spec's ambient state or with bare idle — bd is confirmed
			// healthy (probed above), so this is a genuine resolution
			// failure for the requested target and must be reported as
			// such. Only the no-target (ambient) case below may fall
			// through to draft/idle discovery. Final-review round 1
			// (R5/G3): the returned error's rendered text can chain bd
			// stderr — wrap for the terminal boundary (Unwrap preserved).
			if specFlag != "" {
				return next.RenderBDError(resolveErr)
			}

			// bd is healthy and reports nothing active. Check the OQ-2
			// on-disk draft carve-out before concluding idle: a pre-epic
			// draft has no bd trace and is invisible to bd-based
			// discovery.
			drafts, draftErr := resolve.DiscoverDraftSpecsWithCache(c, root)
			if draftErr != nil {
				return stateDiscoveryFailure("scanning for draft specs", draftErr)
			}

			switch len(drafts) {
			case 1:
				out := &stateOutput{
					Mode:       state.ModeSpec,
					ActiveSpec: drafts[0].SpecID,
					SpecBranch: drafts[0].SpecBranch,
				}
				data, err := json.MarshalIndent(out, "", "  ")
				if err != nil {
					return fmt.Errorf("formatting state: %w", err)
				}
				fmt.Println(string(data))
				return nil
			default:
				if len(drafts) > 1 {
					active := make([]resolve.SpecStatus, len(drafts))
					for i, d := range drafts {
						active[i] = resolve.SpecStatus{SpecID: d.SpecID, Mode: state.ModeSpec, Active: true}
					}
					return &resolve.ErrAmbiguousTarget{Active: active}
				}
			}

			// No active bd spec, no draft: genuinely idle.
			out := &stateOutput{Mode: state.ModeIdle}
			data, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		// R1.2: one derivation is authoritative for both mode and bead —
		// drop the ResolveMode-for-mode / ambient-context-for-bead split.
		// ResolveModeWithCache is itself draft-aware (OQ-2) and propagates
		// discovery errors instead of collapsing them to idle.
		mode, modeErr := resolve.ResolveModeWithCache(c, root, specID)
		if modeErr != nil {
			return stateDiscoveryFailure(fmt.Sprintf("deriving mode for spec %s", idrender.Spec(specID)), modeErr)
		}

		// Round-1 panel G1: idle is reachable on this success path ONLY
		// via an explicit --spec pass-through to a spec ID with no epic
		// and no on-disk draft (an auto-selected spec always has an
		// epic; a worktree-derived spec ID always probes to at least
		// `spec` mode) — never render the R1.1-banned idle-alongside-
		// non-empty-activeSpec shape for a flagged spec that simply does
		// not exist.
		if specFlag != "" && mode == state.ModeIdle {
			return fmt.Errorf("no active spec or draft found for %s; run `mindspec spec list` and re-run with a listed ID", idrender.Spec(specID))
		}

		// ActiveBead is a secondary, best-effort field (ADR-0042 degrade-
		// vs-error policy for never-block consumers, matching SpecBranch
		// below): an epic-lookup hiccup here does not un-derive the
		// mode already established above.
		activeBead := ""
		if epicID, epicErr := phase.FindEpicBySpecIDWithCache(c, specID); epicErr == nil && epicID != "" {
			activeBead = phase.FindActiveBeadForEpicWithCache(c, epicID)
		}

		// Ambient display command: a malformed specID (shouldn't occur —
		// resolve.ResolveTarget's result is R3-gated — but this degrades
		// safely regardless) simply omits SpecBranch (ADR-0042 degrade-
		// vs-error policy for never-block consumers).
		specBranch, _ := workspace.SpecBranch(specID)
		out := &stateOutput{
			Mode:       mode,
			ActiveSpec: specID,
			ActiveBead: activeBead,
			SpecBranch: specBranch,
		}

		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("formatting state: %w", err)
		}
		fmt.Println(string(data))
		return nil
	},
}

var stateWriteSessionCmd = &cobra.Command{
	Use:   "write-session",
	Short: "Record session freshness metadata from SessionStart hook",
	Long:  `Writes sessionSource and sessionStartedAt to session.json. Called by the SessionStart hook with the source field from stdin JSON.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		source, _ := cmd.Flags().GetString("source")
		if source == "" {
			source = "unknown"
		}

		root, err := findRoot()
		if err != nil {
			return err
		}

		sess := &state.Session{
			SessionSource:    source,
			SessionStartedAt: time.Now().UTC().Format(time.RFC3339),
		}

		// On compact, preserve BeadClaimedAt so the freshness gate
		// still blocks mindspec next — compact is NOT a fresh context.
		if source == "compact" {
			if prev, readErr := state.ReadSession(root); readErr == nil {
				sess.BeadClaimedAt = prev.BeadClaimedAt
			}
		}

		if err := state.WriteSessionFile(root, sess); err != nil {
			return fmt.Errorf("writing session metadata: %w", err)
		}

		fmt.Printf("Session recorded: source=%s\n", source)
		return nil
	},
}

func init() {
	stateShowCmd.Flags().String("spec", "", "Target spec ID (auto-detected if exactly one active spec)")

	stateWriteSessionCmd.Flags().String("source", "", "Session source from SessionStart hook (startup, clear, resume, compact)")

	stateCmd.AddCommand(stateSetCmd)
	stateCmd.AddCommand(stateShowCmd)
	stateCmd.AddCommand(stateWriteSessionCmd)
}
