package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

func runPanelDeferToPlan(cmd *cobra.Command, reg panel.Registration, slot string, round int, finding, by string) error {
	read := func() (*panel.Result, []panel.Finding, string, error) {
		res, err := panelVerbSnapshot(reg.Dir)
		if err != nil {
			return nil, nil, "", err
		}
		if res.Panel.Abandoned {
			return nil, nil, "", fmt.Errorf("panel is abandoned")
		}
		if round != res.Panel.Round || round != res.LatestRound {
			return nil, nil, "", fmt.Errorf("round must be the latest verdict round")
		}
		member := false
		for _, s := range res.Panel.Roster {
			if s.Slot == slot {
				member = true
			}
		}
		if !member {
			return nil, nil, "", fmt.Errorf("slot must belong to the recorded roster")
		}
		// Keep the shared finding checks.
		if _, _, err := deferFinding(reg.Dir, slot, round, finding); err != nil {
			return nil, nil, "", err
		}
		v, verdict, err := deferVerdict(reg.Dir, slot, round)
		if err != nil {
			return nil, nil, "", err
		}
		fs, err := panel.Findings(v)
		if err != nil {
			return nil, nil, "", err
		}
		if finding != "" {
			for _, f := range fs {
				if f.ID == finding {
					fs = []panel.Finding{f}
					break
				}
			}
		}
		return res, fs, verdict, nil
	}
	res, findings, verdict, err := read()
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
	severity := "blocking"
	if verdict == panel.VerdictApprove {
		severity = "minor"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var records [][]byte
	var entries []panel.PlanDeferral
	for _, f := range findings {
		d := panel.PlanDeferral{ID: panel.PlanDeferralID(res.Panel.Spec, reg.Slug(), slot, round, f), Slot: slot, Round: round,
			Finding: f.ID, FindingSHA256: f.SHA256, FindingText: f.Text, By: by, CreatedAt: now}
		entries = append(entries, d)
		for _, value := range []any{
			panel.PlanDeferralRecord{Record: panel.RecordDeferredToPlan, Spec: panelRecordSpec(*res.Panel), Panel: reg.Slug(), PlanDeferral: d},
			panel.DispositionRow{Record: panel.RecordDisposition, ID: d.ID + ":disposition", Spec: panelRecordSpec(*res.Panel), Gate: res.Panel.Gate,
				Panel: reg.Slug(), Reviewer: slot, Model: model, Severity: severity, Summary: termsafe.Escape(f.Text), ConvergentWith: []string{},
				Disposition: "confirmed-deferred", EvidenceRef: "plan:" + d.ID, CreatedAt: now, Round: &round},
		} {
			raw, err := json.Marshal(value)
			if err != nil {
				return panelVerbFailure(err)
			}
			records = append(records, raw)
		}
	}
	err = panel.AppendRecordsWithCheck(reg.Dir, func() error {
		current, fs, word, err := read()
		if err != nil {
			return err
		}
		if word != verdict || !reflect.DeepEqual(fs, findings) || !reflect.DeepEqual(current.Panel.Roster, res.Panel.Roster) || current.Panel.Gate != "spec_approve" {
			return fmt.Errorf("finding changed; retry against the current verdict")
		}
		res = current
		return nil
	}, records...)
	if err != nil {
		return panelVerbFailure(err)
	}
	for _, d := range entries {
		found := false
		for _, old := range res.Panel.PlanDeferrals {
			if old.ID == d.ID {
				found = true
			}
		}
		if !found {
			res.Panel.PlanDeferrals = append(res.Panel.PlanDeferrals, d)
		}
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
