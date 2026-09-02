package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
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
func stateDiscoveryFailure(query string, err error) error {
	return guard.NewFailure(
		fmt.Sprintf("state show: %s failed: %v", query, err),
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

			// No bd-active spec resolved. ResolveTargetWithCache's error
			// text does not yet distinguish "bd is healthy, nothing
			// active" from "bd/discovery itself failed" — probe bd
			// health directly, up front, before drawing any conclusion.
			// A discovery failure is never read as "no drafts, therefore
			// idle" (the named hazard: never fabricate a mode from a
			// failed read).
			if _, epicsErr := c.AllEpics(); epicsErr != nil {
				return stateDiscoveryFailure("querying beads for active specs", epicsErr)
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
