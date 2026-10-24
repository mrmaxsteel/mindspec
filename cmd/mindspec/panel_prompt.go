package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/rules"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

var fileAtRefForPanelBriefFn = func(root, sha, path string) ([]byte, bool, error) {
	return newExecutor(root).FileAtRefOrAbsent(sha, path)
}

func resolvePanelBriefRegions(root, specDir, specID, sha, gate string, cfg *config.Config) (panel.BriefRegions, error) {
	acceptance := cfg.GateAcceptance(gate)
	regions := panel.BriefRegions{Question: acceptance.Question, NotBlocking: acceptance.NotBlocking, SpecACs: "none (adhoc panel)"}
	conduct, err := rules.Filter("reviewer", gate)
	if err != nil {
		return regions, err
	}
	for _, rule := range conduct {
		regions.Conduct = append(regions.Conduct, panel.BriefRule{ID: rule.ID, Prohibition: rule.Prohibition, Replacement: rule.Replacement})
	}
	read := func(path string) ([]byte, bool, error) { return fileAtRefForPanelBriefFn(root, sha, path) }
	if specID != "" {
		specPath, err := filepath.Rel(root, filepath.Join(specDir, "spec.md"))
		if err != nil {
			return regions, err
		}
		data, _, err := read(filepath.ToSlash(specPath))
		if err != nil {
			return regions, guard.NewFailure("cannot read reviewed acceptance criteria: "+termsafe.Escape(err.Error()), "check the reviewed commit and run panel create again")
		}
		regions.SpecACs = panel.AcceptanceCriteriaSection(string(data))
	}
	if err := panel.ValidateBriefRegions(regions, read); err != nil {
		return regions, err
	}
	return regions, nil
}

var panelPromptCmd = &cobra.Command{
	Use:          "prompt <slug> --slot S",
	Short:        "Print a slot review prompt without writing files",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validatePanelSlug(args[0]); err != nil {
			return err
		}
		root, err := findPanelRoot(config.IsolationRootFor)
		if err != nil {
			return err
		}
		specID, _ := cmd.Flags().GetString("spec")
		slotID, _ := cmd.Flags().GetString("slot")
		reg, err := findPanelRegistration(root, args[0], specID)
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		in, err := resolvePanelPrompt(root, reg, slotID, cfg)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), panel.RenderSlotPrompt(in))
		return err
	},
}

// resolvePanelPrompt reads the effective runner and model from the same round
// as the roster. The launcher can replace the nonce and attempt before render.
func resolvePanelPrompt(root string, reg panel.Registration, slotID string, cfg *config.Config) (panel.PromptInput, error) {
	var in panel.PromptInput
	if reg.Err != nil {
		return in, reg.Err
	}
	p := reg.Panel
	found := false
	for _, slot := range p.Roster {
		if slot.Slot == slotID {
			in.Slot, in.Family, in.Model, in.Lens = slot.Slot, slot.Family, slot.Model, slot.Lens
			found = true
			break
		}
	}
	if !found {
		return in, guard.NewFailure("slot is not on the roster", "select a slot from the panel roster")
	}
	if in.Family == "" {
		in.Family = "harness"
	}
	lines, err := panel.ReadLedger(reg.Dir)
	if err != nil {
		return in, err
	}
	for _, line := range lines {
		var sub panel.SubstitutionRecord
		if err := json.Unmarshal(line, &sub); err != nil {
			return in, err
		}
		if sub.Record == panel.RecordSubstitution && sub.Round == p.Round && sub.Slot == slotID {
			in.Family, in.Model = sub.ToFamily, sub.Model
		}
	}
	runner, ok := cfg.RunnerFor(in.Family)
	if !ok {
		return in, guard.NewFailure("slot family has no runner: "+termsafe.Escape(in.Family), "add a runner for the family to panel.runners")
	}
	in.Kind, in.Output, in.Focus = runner.Kind, runner.Output, cfg.Panel.Lenses[in.Lens]
	layout, err := panel.NewSlotLayout(root, cfg.Panel.IsolationRoot, p.Spec, reg.Slug(), p.Round, slotID)
	if err != nil {
		return in, err
	}
	in.SlotDir, in.Tree = layout.Dir, layout.Tree
	brief, err := panel.ReadFilledBrief(filepath.Join(reg.Dir, "BRIEF.md"))
	if err != nil {
		return in, guard.NewFailure(err.Error(), "run panel create to render the brief")
	}
	in.Brief = string(brief)
	if runner.Kind == "process" {
		in.Attempt, err = panel.NextAttemptNumber(reg.Dir, p.Round, slotID, layout.Dir)
		if err != nil {
			return in, err
		}
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return in, err
		}
		in.Nonce = hex.EncodeToString(nonce[:])
	} else {
		attempts, err := panel.SlotAttempts(reg.Dir, p.Round)
		if err != nil {
			return in, err
		}
		for _, attempt := range attempts[slotID] {
			if attempt.Status == "dispatched" && attempt.Attempt > in.Attempt {
				in.Attempt = attempt.Attempt
			}
		}
	}
	in.PriorItems, err = panel.PriorRoundItems(reg.Dir, slotID, p.Round)
	return in, err
}

func init() {
	panelPromptCmd.Flags().String("slot", "", "Roster slot to review")
	panelPromptCmd.Flags().String("spec", "", "Select the owning spec when the slug is ambiguous")
	panelCmd.AddCommand(panelPromptCmd)
}
