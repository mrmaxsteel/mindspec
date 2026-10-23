package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var panelRebindCmd = &cobra.Command{
	Use:   "rebind <slug> --reason <text>",
	Short: "Refresh a passing panel to its target's live tip without a new round",
	Long: `Refresh a passing panel's reviewed commit and subject digest without changing
its round. --reason is required and records the reviewer's judgement about the
diff. Every changed path is accepted except .mindspec/config.yaml. The audit
entry records the diff paths, excluding review artifacts. The reviewed commit
must be an ancestor of the live target. An already fresh panel writes nothing.
A spec or plan panel pinned to a commit tracks its spec branch. A hex-looking
name is read as a commit, not as a branch or tag name. Only confirmed absence
of refs/heads/<spec branch> permits fallback to the recorded target.
Confirmation uses git show-ref --exists (git 2.43 or later). An unreadable
spec branch, lookup errors and older Git versions refuse rebind.`,
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE:         panelRebindRunE,
}

func init() {
	panelRebindCmd.Flags().String("reason", "", "Who accepts this diff and why")
	panelRebindCmd.Flags().String("spec", "", "Select the owning spec when the slug is ambiguous")
	panelCmd.AddCommand(panelRebindCmd)
}

func panelRebindRunE(cmd *cobra.Command, args []string) error {
	slug := args[0]
	if err := validatePanelSlug(slug); err != nil {
		return err
	}
	reason, _ := cmd.Flags().GetString("reason")
	if strings.TrimSpace(reason) == "" {
		return guard.NewFailure("panel rebind requires a non-empty reason", "supply --reason '<who>: <why>'")
	}
	root, err := findPanelRoot(config.IsolationRootFor)
	if err != nil {
		return err
	}
	specID, _ := cmd.Flags().GetString("spec")
	reg, err := findPanelRegistration(root, slug, specID)
	if err != nil {
		return err
	}
	refuse := func(err error) error { return panelRebindFailure(reg, err) }
	before, err := os.ReadFile(filepath.Join(reg.Dir, panel.FileName))
	if err != nil {
		return refuse(err)
	}
	res, err := panel.Tally(reg.Dir)
	if err != nil {
		return refuse(err)
	}
	if d := panel.RebindDecision(reg, res); d.Action != panel.Allow {
		return refuse(fmt.Errorf("%s", d.Message))
	}
	p := *res.Panel
	// Resolve in the panel's checkout: a co-located spec can be outside the
	// invoking checkout, and the target is the ref the panel actually reviewed.
	scanRoot := panel.PanelDirScanRoot(reg.Dir)
	exec := newExecutor(scanRoot)
	live := resolvePanelGateFacts(root, reg)
	if live.GitErr != nil {
		return refuse(live.GitErr)
	}
	if live.MissingRef || live.HeadSHA == "" {
		return refuse(fmt.Errorf("cannot resolve the panel's live target"))
	}
	tip := live.HeadSHA
	if p.ReviewedHeadSHA == tip {
		fmt.Fprintln(cmd.OutOrStdout(), "already fresh")
		return nil
	}
	if p.ReviewedHeadSHA == "" {
		return refuse(fmt.Errorf("panel has no reviewed commit"))
	}
	reviewed, err := exec.RevParseRef(scanRoot, p.ReviewedHeadSHA)
	if err != nil {
		return refuse(err)
	}
	base, err := exec.MergeBase(reviewed, tip)
	if err != nil {
		return refuse(fmt.Errorf("cannot verify reviewed commit ancestry: %w", err))
	}
	if base != reviewed {
		return refuse(fmt.Errorf("the reviewed commit is not an ancestor of the live tip"))
	}
	paths, err := exec.ChangedFiles(p.ReviewedHeadSHA, tip, executor.ChangedFilesOptions{NoRenames: true})
	if err != nil {
		return refuse(err)
	}
	for i, name := range paths {
		// ChangedFiles returns git's quoted name-only syntax; decode it before
		// comparing exclusions or recording repository-relative paths.
		if strings.HasPrefix(name, "\"") {
			name, err = strconv.Unquote(name)
			if err != nil {
				return refuse(err)
			}
			paths[i] = name
		}
		if name == ".mindspec/config.yaml" {
			return refuse(fmt.Errorf("the diff changes .mindspec/config.yaml; a new review round is required"))
		}
	}
	subject := panel.Subject(p.Gate, panel.PanelSpecDir(reg.Dir, filepath.Join(workspace.SpecsDir(root), p.Spec)))
	digest, err := panel.SubjectTreeReader(exec.LsTree)(scanRoot, tip, subject)
	if err != nil {
		return refuse(err)
	}
	entry := panel.Rebind{Round: p.Round, FromSHA: p.ReviewedHeadSHA, ToSHA: tip,
		Paths: panel.RebindPaths(paths, subject), Reason: reason, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	spec := p.Spec
	if p.Gate == "adhoc" {
		spec = "adhoc"
	}
	event, err := json.Marshal(panel.RebindRecord{Record: panel.RecordRebind, Spec: spec, Panel: slug, SubjectDigest: digest, Rebind: entry})
	if err != nil {
		return refuse(err)
	}
	if err := panel.AppendRecordsWithCheck(reg.Dir, func() error {
		current, err := os.ReadFile(filepath.Join(reg.Dir, panel.FileName))
		if err != nil {
			return err
		}
		if !bytes.Equal(current, before) {
			return fmt.Errorf("panel registration changed during rebind; retry")
		}
		return nil
	}, event); err != nil {
		return refuse(err)
	}
	p.ReviewedHeadSHA = tip
	p.SubjectDigest = digest
	p.Rebinds = append(p.Rebinds, entry)
	if err := panel.WriteSnapshot(reg.Dir, p); err != nil {
		return refuse(err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "panel %s round %d rebound to %s\n", termsafe.Escape(slug), p.Round, termsafe.Escape(tip))
	return nil
}

func panelRebindFailure(reg panel.Registration, err error) error {
	return panelCreateFailure(fmt.Errorf("%s", termsafe.Escape(err.Error())), reg.Slug(), panel.CreateInput{
		Spec: reg.Panel.Spec, Target: reg.Panel.Target, Gate: reg.Panel.Gate, Round: reg.Panel.Round + 1,
	})
}
