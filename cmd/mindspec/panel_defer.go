package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

var panelDeferCmd = &cobra.Command{Use: "defer <slug>", Short: "Defer a latest-round finding to an open follow-up bead", Args: cobra.ExactArgs(1), SilenceUsage: true, RunE: runPanelDefer}

func runPanelDefer(cmd *cobra.Command, args []string) error {
	_, reg, err := resolvePanelVerb(cmd, args[0])
	if err != nil {
		return panelVerbFailure(err)
	}
	slot, _ := cmd.Flags().GetString("slot")
	round, _ := cmd.Flags().GetInt("round")
	finding, _ := cmd.Flags().GetString("finding")
	beadID, _ := cmd.Flags().GetString("bead")
	by, _ := cmd.Flags().GetString("by")
	if strings.TrimSpace(by) == "" {
		return panelVerbFailure(fmt.Errorf("--by must not be empty"))
	}
	if reg.Panel.Gate == "spec_approve" || reg.Panel.Gate == "adhoc" || reg.Panel.Gate == "" {
		return guard.NewFailure("this panel has no owning epic for a deferral", "fix the finding or use mindspec panel refute")
	}
	epic, err := phase.FindEpicBySpecID(reg.Panel.Spec)
	if err != nil {
		return panelVerbFailure(err)
	}
	res, err := panelVerbSnapshot(reg.Dir)
	if err != nil {
		return panelVerbFailure(err)
	}
	if res.Panel.Abandoned {
		return panelVerbFailure(fmt.Errorf("panel is abandoned"))
	}
	f, err := refuteFinding(reg.Dir, slot, round, finding)
	if err != nil {
		return panelVerbFailure(err)
	}
	model := ""
	for _, s := range res.Panel.Roster {
		if s.Slot == slot {
			model = s.Model
		}
	}
	if model == "" {
		return panelVerbFailure(fmt.Errorf("slot must belong to the recorded roster"))
	}
	check := func() error {
		texts := []string{f.Text}
		if f.ID == "slot" {
			v, err := panel.ReadFindingVerdict(reg.Dir, slot, round)
			if err != nil {
				return err
			}
			findings, err := panel.Findings(v)
			if err != nil {
				return err
			}
			texts = nil
			for _, finding := range findings {
				texts = append(texts, finding.Text)
			}
		}
		reviewedBead := ""
		if res.Panel.BeadID != nil {
			reviewedBead = *res.Panel.BeadID
		}
		return bead.CheckDeferral(beadID, epic, reviewedBead, texts, panel.RedactFindingPaths)
	}
	if err := check(); err != nil {
		return panelVerbFailure(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	ref := panel.Deferral{Slot: slot, Round: round, Finding: f.ID, FindingSHA256: f.SHA256,
		FindingText: f.Text, Bead: beadID, By: by, CreatedAt: now}
	event, err := json.Marshal(panel.DeferralRecord{Record: panel.RecordDeferral, Spec: panelRecordSpec(*res.Panel), Panel: args[0], Deferral: ref})
	if err != nil {
		return panelVerbFailure(err)
	}
	gate := res.Panel.Gate
	if gate == "" {
		gate = "adhoc"
	}
	row, err := json.Marshal(panel.DispositionRow{Record: "disposition", ID: fmt.Sprintf("%s:%s:r%d:%s:%s:deferral", args[0], slot, round, f.ID, f.SHA256[:12]),
		Spec: panelRecordSpec(*res.Panel), Gate: gate, Panel: args[0], Reviewer: slot, Model: model, Severity: "blocking",
		Summary: termsafe.Escape(f.Text), ConvergentWith: []string{}, Disposition: "confirmed-deferred", EvidenceRef: beadID, CreatedAt: now, Round: &round})
	if err != nil {
		return panelVerbFailure(err)
	}
	err = panel.AppendRecordsWithCheck(reg.Dir, func() error {
		current, err := panelVerbSnapshot(reg.Dir)
		if err != nil {
			return err
		}
		if current.Panel.Abandoned {
			return fmt.Errorf("panel is abandoned")
		}
		latest, err := refuteFinding(reg.Dir, slot, round, finding)
		if err != nil {
			return err
		}
		if latest != f {
			return fmt.Errorf("finding changed; retry against the current verdict")
		}
		res = current
		return check()
	}, event, row)
	if err != nil {
		return panelVerbFailure(err)
	}
	found := false
	for _, old := range res.Panel.Deferrals {
		if old.Slot == slot && old.Round == round && old.Finding == f.ID && old.FindingSHA256 == f.SHA256 {
			found = true
		}
	}
	if !found {
		res.Panel.Deferrals = append(res.Panel.Deferrals, ref)
	}
	if err := panel.WriteSnapshot(reg.Dir, *res.Panel); err != nil {
		return panelVerbFailure(err)
	}
	updated, err := panel.Tally(reg.Dir)
	if err != nil {
		return panelVerbFailure(err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), panel.RuleLine(updated))
	return nil
}

func init() {
	panelDeferCmd.Flags().String("spec", "", "Select the owning spec when the slug is ambiguous")
	panelDeferCmd.Flags().String("slot", "", "Roster slot to defer")
	panelDeferCmd.Flags().Int("round", 0, "Latest verdict round")
	panelDeferCmd.Flags().String("finding", "", "Finding id ccr[i]; omit to cover the whole slot")
	panelDeferCmd.Flags().String("bead", "", "Open follow-up bead outside the owning epic")
	panelDeferCmd.Flags().String("by", "", "Name of the person recording the deferral")
	panelCmd.AddCommand(panelDeferCmd)
}
