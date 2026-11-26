package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
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
	// A hand edit of panel.json (focus included) must not reach a reviewer prompt.
	if err := checkHandEdits(reg); err != nil {
		return in, err
	}
	found := false
	for _, slot := range p.Roster {
		if slot.Slot == slotID {
			in.Slot, in.Family, in.Model, in.Lens, in.Focus = slot.Slot, slot.Family, slot.Model, slot.Lens, slot.Focus
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
	in.Kind, in.Output = runner.Kind, runner.Output
	if in.Focus == "" || in.Lens == "adversarial" {
		in.Focus = cfg.Panel.Lenses[in.Lens]
	}
	layout, err := panel.NewSlotLayout(root, cfg.Panel.IsolationRoot, p.Spec, reg.Slug(), p.Round, slotID)
	if err != nil {
		return in, err
	}
	in.SlotDir, in.Tree = layout.Dir, layout.Tree
	brief, err := panel.ReadPanelBrief(reg.Dir, reg.Panel)
	if err != nil {
		return in, guard.NewFailure(err.Error(), fmt.Sprintf("fill %s, then run mindspec panel prompt %s --slot %s --spec %s", shellQuoteTarget(filepath.Join(reg.Dir, "BRIEF.md")), shellQuoteTarget(reg.Slug()), shellQuoteTarget(slotID), shellQuoteTarget(reg.Panel.Spec)))
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
	if p.Gate == "bead" || p.Gate == "final_review" {
		// A prompt is read-only. Without a kernel state it keeps the panel command.
		if state, err := panelKernelStateFn(root, p.Spec); err == nil && state != nil {
			if step, err := kernelSlotStep(*state, p, slotID); err == nil {
				in.LeaseStep = step
				spec := ""
				if p.Spec != "" {
					spec = " --spec " + shellQuoteTarget(p.Spec)
				}
				target := shellQuoteTarget(reg.Slug()) + spec + " --slot " + shellQuoteTarget(slotID)
				in.Checkout = "mindspec panel checkout " + target + " --dispatch"
				in.Prompt = "mindspec panel prompt " + target
			}
		}
	}
	if p.Orchestrated {
		in.DurablePrior = true
		return in, nil
	}
	in.PriorItems, err = panel.PriorRoundItems(reg.Dir, slotID, p.Round)
	return in, err
}

func init() {
	panelPromptCmd.Flags().String("slot", "", "Roster slot to review")
	panelPromptCmd.Flags().String("spec", "", "Select the owning spec when the slug is ambiguous")
	panelCmd.AddCommand(panelPromptCmd)
}

// kernelSlotStep returns the current kernel step of a roster slot.
func kernelSlotStep(state kernel.State, p panel.Panel, slot string) (string, error) {
	index := -1
	for i, reviewer := range p.Roster {
		if reviewer.Slot == slot {
			index = i
			break
		}
	}
	if index < 0 || slot != fmt.Sprintf("R%d", index+1) {
		return "", guard.NewFailure("slot identity differs from its roster position", "select a slot from the panel roster")
	}
	gate := kernelPanelGate(state, p)
	head, err := kernel.SlotLineageHead(state, fmt.Sprintf("%s.r%d.R%d", gateBase(state, gate), p.Round, index+1))
	if err != nil {
		return "", err
	}
	return head.Key(), nil
}
