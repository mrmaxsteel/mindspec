package lifecycle

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// MergePresence records the changed paths and the subset restored to their
// pre-merge state. LaterChanges is newest first and contains only commits
// that changed a reverted path on the spec branch's first-parent chain.
type MergePresence struct {
	ChangedPaths  []string
	RevertedPaths []string
	LaterChanges  []MergePathChange
}

// MergePathChange identifies a later commit and the reverted paths it changed.
type MergePathChange struct {
	Commit gitutil.MergeCommit
	Paths  []string
}

// FindNamingMerges finds two-parent merges that name this bead on the spec
// branch's first-parent chain. It does not check the bead tip or content.
func FindNamingMerges(root, specBranch, beadID string) ([]gitutil.MergeCommit, error) {
	return gitutil.FindNamingMerges(root, specBranch, beadID)
}

// CheckMergePresence checks a naming merge independently of branch survival
// and tip equality. The caller supplies the owning spec directory for tool
// artifact filtering. A later naming merge for another bead exempts paths
// it changes. Other later edits pass unless they restore pre-merge content.
func CheckMergePresence(root, specBranch, specDir string, merge *LandedMerge) (MergePresence, error) {
	var result MergePresence
	tip, err := gitutil.RevParseRef(root, specBranch)
	if err != nil {
		return result, err
	}
	ancestor, err := gitutil.IsAncestor(root, merge.SHA, tip)
	if err != nil {
		return result, err
	}
	if !ancestor {
		return result, fmt.Errorf("merge %s is not an ancestor of %s", termsafe.Escape(merge.SHA), termsafe.Escape(tip))
	}
	paths, err := gitutil.DiffNameOnlyNoRenames(root, merge.FirstParent, merge.SHA)
	if err != nil {
		return result, err
	}
	later, err := gitutil.FirstParentCommitsAfter(root, merge.SHA, tip)
	if err != nil {
		return result, err
	}
	// A second naming merge for this bead is never an exemption.
	naming, err := gitutil.CommitMessageBody(root, merge.SHA)
	if err != nil {
		return result, err
	}
	mergeBranch, _ := parseMergeSubjectBeadBranch(strings.SplitN(naming, "\n", 2)[0])
	exempt := make(map[string]bool)
	var changes []MergePathChange
	for _, c := range later {
		if len(c.Parents) == 0 {
			return result, fmt.Errorf("later commit %s has no parent", termsafe.Escape(c.SHA))
		}
		changed, err := gitutil.DiffNameOnlyNoRenames(root, c.Parents[0], c.SHA)
		if err != nil {
			return result, err
		}
		changes = append(changes, MergePathChange{Commit: c, Paths: changed})
		branch, named := parseMergeSubjectBeadBranch(c.Subject)
		if len(c.Parents) == 2 && named && branch != mergeBranch {
			for _, p := range changed {
				exempt[p] = true
			}
		}
	}
	reverted := make(map[string]bool)
	mergePaths := make(map[string]bool)
	for _, p := range paths {
		if panel.IsToolArtifactPath(p, specDir) {
			continue
		}
		result.ChangedPaths = append(result.ChangedPaths, p)
		mergePaths[p] = true
		if exempt[p] {
			continue
		}
		before, err := mergePathContent(root, merge.FirstParent, p)
		if err != nil {
			return result, err
		}
		now, err := mergePathContent(root, tip, p)
		if err != nil {
			return result, err
		}
		if before == now {
			result.RevertedPaths = append(result.RevertedPaths, p)
			reverted[p] = true
		}
	}
	for _, c := range changes {
		var matched []string
		touchesReverted := false
		for _, p := range c.Paths {
			if reverted[p] {
				touchesReverted = true
			}
			if mergePaths[p] {
				matched = append(matched, p)
			}
		}
		if touchesReverted {
			c.Paths = matched
			result.LaterChanges = append(result.LaterChanges, c)
		}
	}
	return result, nil
}

// Compare content object IDs. A later mode change does not hide restored
// content. An empty ID denotes a path absent from this tree.
func mergePathContent(root, ref, path string) (string, error) {
	entry, err := gitutil.TreePathEntry(root, ref, path)
	if err != nil || len(entry) == 0 {
		return "", err
	}
	header, _, ok := strings.Cut(string(entry), "\t")
	fields := strings.Fields(header)
	if !ok || len(fields) != 3 {
		return "", fmt.Errorf("invalid tree path record")
	}
	return fields[2], nil
}

// FindValidMerge requires one naming merge, a surviving bead branch at its
// second parent, and no reverted path. Metadata markers do not decide this.
// A nil result with no error means that no valid merge exists.
func FindValidMerge(root, specBranch, beadID string) (*LandedMerge, error) {
	merges, err := FindNamingMerges(root, specBranch, beadID)
	if err != nil || len(merges) != 1 {
		return nil, err
	}
	branch, err := workspace.BeadBranch(beadID)
	if err != nil {
		return nil, err
	}
	tip, survives, err := resolveBranchTip(root, branch)
	if err != nil || !survives || tip != merges[0].Parents[1] {
		return nil, err
	}
	specDir, err := mergeSpecDir(root, specBranch)
	if err != nil {
		return nil, err
	}
	m := namingMergeIdentity(merges[0])
	presence, err := CheckMergePresence(root, specBranch, specDir, m)
	if err != nil || len(presence.RevertedPaths) != 0 {
		return nil, err
	}
	return m, nil
}

func namingMergeIdentity(m gitutil.MergeCommit) *LandedMerge {
	return &LandedMerge{SHA: m.SHA, FirstParent: m.Parents[0], SecondParent: m.Parents[1]}
}

func mergeSpecDir(root, specBranch string) (string, error) {
	specID := strings.TrimPrefix(specBranch, "spec/")
	if err := idvalidate.SpecID(specID); err != nil {
		return "", err
	}
	return workspace.SpecDir(root, specID)
}

// MergeRestoreOutcome reads user changes without writes. Only a clean
// checkout permits path restoration. All other outcomes omit that choice.
func MergeRestoreOutcome(specWorktree, specDir string) guard.DestructionOutcome {
	status, err := gitutil.StatusZWithStderr(specWorktree)
	if err != nil {
		return guard.DestructionEvidenceError
	}
	fields := strings.Split(status, "\x00")
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if entry == "" {
			continue
		}
		if len(entry) < 4 {
			return guard.DestructionEvidenceError
		}
		if !panel.IsToolArtifactPath(entry[3:], specDir) {
			return guard.DestructionStaleDeletion
		}
		if strings.ContainsAny(entry[:2], "RC") {
			i++
			if i >= len(fields) || fields[i] == "" {
				return guard.DestructionEvidenceError
			}
			if !panel.IsToolArtifactPath(fields[i], specDir) {
				return guard.DestructionStaleDeletion
			}
		}
	}
	return guard.DestructionClean
}

// RestoreMergeRecovery lists whole-commit reversals, then an optional
// path-scoped restore. The caller supplies its spec-checkout dirt outcome.
// Follow-up work starts only after the restored merge is verified and closed.
func RestoreMergeRecovery(specWorktree, beadID, epicID string, merge *LandedMerge, presence MergePresence, outcome guard.DestructionOutcome) (string, error) {
	var body strings.Builder
	fmt.Fprintf(&body, "bead %s has reverted paths from merge %s: %s.\nDo not revert a --merge-only merge by hand. Send a rejected change to a follow-up bead.\nA revert undoes the whole commit. Inspect each commit before you revert it.\n", idrender.Bead(beadID), termsafe.Escape(merge.SHA), mergeQuotedPaths(presence.RevertedPaths))
	var commands []string
	for _, c := range presence.LaterChanges {
		fmt.Fprintf(&body, "commit %s: %s; merge paths: %s\n", termsafe.Escape(c.Commit.SHA), termsafe.Escape(c.Commit.Subject), mergeQuotedPaths(c.Paths))
		mainline := ""
		if len(c.Commit.Parents) > 1 {
			mainline = " -m 1"
		}
		commands = append(commands, "git -C "+mergeWorktreeQuote(specWorktree)+" revert"+mainline+" "+mergeShellQuote(c.Commit.SHA))
	}
	if outcome == guard.DestructionClean && len(presence.RevertedPaths) > 0 {
		checkout, err := guard.NewDestructiveCommand("git -C "+mergeWorktreeQuote(specWorktree)+" checkout "+mergeShellQuote(merge.SHA)+" -- "+mergeQuotedPaths(presence.RevertedPaths), outcome)
		if err != nil {
			return "", err
		}
		body.WriteString("As a path-scoped choice, restore the listed paths and commit them. Use this choice instead of the whole-commit reverts.\n")
		commands = append(commands, checkout.String(), "git -C "+mergeWorktreeQuote(specWorktree)+" commit -m 'Restore merged paths'")
	}
	body.WriteString("If the latest panel round did not review the bead tip, run a new panel round at that tip first. Run complete until it exits 0.\nAfter complete exits 0, create the follow-up bead. Then edit plan.md on the spec branch. Append the new bead as the Nth `## Bead` heading with a concrete acceptance criterion. Add work_chunks id N and its key_file_paths. Commit plan.md before you claim the new bead.\nA panel round at the fix tip does not carry over. The new bead needs its own panel.")
	commands = append(commands,
		"mindspec complete "+idrender.Bead(beadID),
		"bd create \"<title>\" --parent "+idrender.Bead(epicID)+" --deps "+idrender.Bead(beadID),
		"mindspec next <new bead>")
	return guard.FormatFailure(body.String(), commands...), nil
}

func mergeShellQuote(s string) string {
	return "'" + strings.ReplaceAll(termsafe.Escape(s), "'", "'\"'\"'") + "'"
}

// Keep the global-option operand in one quote span for the destructive
// command classifier. Shell concatenation is valid but is outside its parser.
func mergeWorktreeQuote(worktree string) string {
	if !strings.Contains(worktree, "'") {
		return mergeShellQuote(worktree)
	}
	safe := termsafe.Escape(worktree)
	if strings.HasSuffix(safe, "\\") {
		// A directory suffix also keeps an escaped trailing backslash away
		// from the closing quote in the classifier's finite quote parser.
		safe += "/."
	}
	return "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "\\$", "`", "\\`").Replace(safe) + "\""
}

func mergeQuotedPaths(paths []string) string {
	quoted := make([]string, 0, len(paths))
	for _, p := range paths {
		// Literal pathspecs also protect filenames that start with a colon.
		quoted = append(quoted, mergeShellQuote(":(literal)"+p))
	}
	return strings.Join(quoted, " ")
}
