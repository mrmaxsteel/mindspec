package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var adrCmd = &cobra.Command{
	Use:   "adr",
	Short: "Manage Architecture Decision Records",
}

var adrCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new ADR from template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		// WRITE target is the worktree-LOCAL root (FindLocalRoot does NOT
		// resolve a worktree back to main, unlike FindRoot), so a new ADR
		// authored from a bead/spec worktree lands in THAT worktree's
		// .mindspec/docs/adr/, not the main checkout (mindspec-8lzq).
		root, err := workspace.FindLocalRoot(cwd)
		if err != nil {
			return err
		}
		// Number the new ADR over the BRANCH+MAIN union: the write store is
		// worktree-local, but reads/validation union the worktree branch ADRs
		// with the main-checkout ADRs (OverlayStore). Allocating NextID over
		// only the local root could collide with a main-only ADR added after
		// the branch diverged, so we also number against the main root. In the
		// main checkout FindRoot == FindLocalRoot and this is a no-op. FindRoot
		// is best-effort: if it can't resolve, number against the local root
		// alone.
		var numberingRoots []string
		if mainRoot, mErr := workspace.FindRoot(cwd); mErr == nil && mainRoot != root {
			numberingRoots = append(numberingRoots, mainRoot)
		}

		domain, _ := cmd.Flags().GetString("domain")
		supersedes, _ := cmd.Flags().GetString("supersedes")

		// SEC-1 (mindspec-x1qr): validate --supersedes at the CLI boundary
		// BEFORE it reaches filepath.Join in internal/adr/create.go. Defense
		// in depth — internal/adr/create.go also validates.
		if supersedes != "" {
			if err := validate.ADRID(supersedes); err != nil {
				return fmt.Errorf("invalid --supersedes value: %w", err)
			}
		}

		var domains []string
		if domain != "" {
			for _, d := range strings.Split(domain, ",") {
				d = strings.TrimSpace(d)
				if d != "" {
					domains = append(domains, d)
				}
			}
		}

		// R5(a) (spec 123): --slug overrides the derived-from-title
		// filename slug. Only pass a non-nil override when the flag was
		// actually set, so an unset --slug falls through to title
		// derivation (adr.CreateOpts.SlugOverride nil vs "" distinction).
		var slugOverride *string
		if cmd.Flags().Changed("slug") {
			s, _ := cmd.Flags().GetString("slug")
			slugOverride = &s
		}

		path, err := adr.CreateUnion(root, numberingRoots, args[0], adr.CreateOpts{
			Domains:      domains,
			Supersedes:   supersedes,
			SlugOverride: slugOverride,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Created: %s\n", path)
		fmt.Println("Fill in the Context and Decision sections, then update Status to Accepted when ready.")
		if supersedes != "" {
			fmt.Printf("Updated %s with Superseded-by reference.\n", supersedes)
		}

		return nil
	},
}

// adrReadStore builds the worktree-aware ADR read store for show/list, so an
// ADR present only in the ACTIVE bead/spec worktree is visible. workspace.
// FindRoot resolves a worktree back to the MAIN checkout (where a
// worktree-local ADR is absent), so reading through a plain FileStore(FindRoot)
// misses it (mindspec-3cfr). Instead it overlays the worktree-LOCAL ADR dir
// (FindLocalRoot, the same root `adr create` writes into) over the main
// checkout's — mirroring the validator's adrStoreForSpec read semantics: branch
// ADRs win on ID conflicts, main-only ADRs stay visible. In a plain checkout
// FindLocalRoot == FindRoot, so the overlay collapses to the prior single
// FileStore behavior.
func adrReadStore(cwd string) (adr.Store, error) {
	localRoot, err := workspace.FindLocalRoot(cwd)
	if err != nil {
		return nil, err
	}
	// Best-effort main root: when distinct from localRoot we're in a
	// worktree and overlay branch-over-main; otherwise (main checkout, or
	// FindRoot can't resolve) read the local root alone.
	if mainRoot, mErr := workspace.FindRoot(cwd); mErr == nil && mainRoot != localRoot {
		return adr.NewOverlayStore(adr.NewFileStore(localRoot), adr.NewFileStore(mainRoot)), nil
	}
	return adr.NewFileStore(localRoot), nil
}

var adrListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ADRs with optional filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		store, err := adrReadStore(cwd)
		if err != nil {
			return err
		}

		status, _ := cmd.Flags().GetString("status")
		domain, _ := cmd.Flags().GetString("domain")

		adrs, err := store.List(adr.ListOpts{
			Status: status,
			Domain: domain,
		})
		if err != nil {
			return err
		}

		if len(adrs) == 0 {
			fmt.Println("No ADRs found.")
			return nil
		}

		fmt.Print(adr.FormatTable(adrs))
		fmt.Printf("\n%d ADR(s)\n", len(adrs))
		return nil
	},
}

var adrShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a single ADR summary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// SEC-1: validate at CLI boundary before reaching filepath.Glob.
		if err := validate.ADRID(args[0]); err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		store, err := adrReadStore(cwd)
		if err != nil {
			return err
		}

		jsonFlag, _ := cmd.Flags().GetBool("json")

		a, err := store.Get(args[0])
		if err != nil {
			return err
		}

		if jsonFlag {
			out, err := adr.FormatJSON(a)
			if err != nil {
				return err
			}
			fmt.Println(out)
		} else {
			fmt.Print(adr.FormatSummary(a))
		}
		return nil
	},
}

func init() {
	adrCreateCmd.Flags().String("domain", "", "Domain(s) for the ADR (comma-separated)")
	adrCreateCmd.Flags().String("supersedes", "", "ADR ID to supersede (e.g., ADR-0001)")
	adrCreateCmd.Flags().String("slug", "", "Override the derived filename slug (lowercase kebab-case); empty writes the bare ADR-NNNN.md form")

	adrListCmd.Flags().String("status", "", "Filter by status (e.g., accepted, proposed, superseded)")
	adrListCmd.Flags().String("domain", "", "Filter by domain")

	adrShowCmd.Flags().Bool("json", false, "Output as JSON")

	adrCmd.AddCommand(adrCreateCmd)
	adrCmd.AddCommand(adrListCmd)
	adrCmd.AddCommand(adrShowCmd)
}
