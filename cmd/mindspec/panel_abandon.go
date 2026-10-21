package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/spf13/cobra"
)

var panelAbandonCmd = &cobra.Command{
	Use: "abandon <slug>", Short: "Abandon a panel with an audited reason and name",
	Args: cobra.ExactArgs(1), SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, reg, err := resolvePanelVerb(cmd, args[0])
		if err != nil {
			return panelVerbFailure(err)
		}
		reason, _ := cmd.Flags().GetString("reason")
		by, _ := cmd.Flags().GetString("by")
		if strings.TrimSpace(reason) == "" || strings.TrimSpace(by) == "" {
			return panelVerbFailure(fmt.Errorf("--reason and --by must not be empty"))
		}
		res, err := panelAbandonSnapshot(reg.Dir, reason)
		if err != nil {
			return panelVerbFailure(err)
		}
		isolation, err := config.IsolationRootFor(root)
		if err != nil {
			return panelVerbFailure(err)
		}
		event := panel.AbandonRecord{Record: panel.RecordAbandon, Spec: panelRecordSpec(*res.Panel), Panel: args[0], Reason: reason, By: by, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		// Reuse the event key on retries, including a crash before snapshot rename.
		lines, err := panel.ReadLedger(reg.Dir)
		if err != nil {
			return panelVerbFailure(err)
		}
		for _, line := range lines {
			var kind struct {
				Record string `json:"record"`
			}
			if err := json.Unmarshal(line, &kind); err != nil {
				return panelVerbFailure(err)
			}
			if kind.Record == panel.RecordRound {
				event.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			}
			if kind.Record == panel.RecordAbandon {
				var old panel.AbandonRecord
				if err := json.Unmarshal(line, &old); err != nil {
					return panelVerbFailure(err)
				}
				event.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
				if old.Reason == reason && old.By == by {
					event.CreatedAt = old.CreatedAt
				}
			}
		}
		raw, err := json.Marshal(event)
		if err != nil {
			return panelVerbFailure(err)
		}
		if err := panel.Validate(raw); err != nil {
			return panelVerbFailure(err)
		}
		if err := panel.HygienePredicate(raw); err != nil {
			return panelVerbFailure(err)
		}
		var pending, prior map[string]interface{}
		if err := json.Unmarshal(raw, &pending); err != nil {
			return panelVerbFailure(err)
		}
		for _, line := range lines {
			var old map[string]interface{}
			if err := json.Unmarshal(line, &old); err != nil {
				return panelVerbFailure(err)
			}
			if old["record"] == panel.RecordAbandon && old["created_at"] == event.CreatedAt {
				prior = old
			}
		}
		if prior != nil && !reflect.DeepEqual(pending, prior) {
			return panelVerbFailure(fmt.Errorf("abandon record already exists with different content"))
		}
		releaseCheckouts, err := cleanupPanelCheckouts(root, reg, isolation, 0, "")
		if err != nil {
			return checkoutError(err, "mindspec panel verify "+shellQuoteTarget(reg.Slug())+checkoutSpecFlag(reg))
		}
		defer releaseCheckouts()
		if err := panel.AppendRecordsWithCheck(reg.Dir, func() error {
			current, err := panelAbandonSnapshot(reg.Dir, reason)
			if err != nil {
				return err
			}
			res = current
			return nil
		}, raw); err != nil {
			return panelVerbFailure(err)
		}
		res.Panel.Abandoned, res.Panel.AbandonReason = true, reason
		if err := panel.WriteSnapshot(reg.Dir, *res.Panel); err != nil {
			return panelVerbFailure(err)
		}
		updated, err := panel.Tally(reg.Dir)
		if err != nil {
			return panelVerbFailure(err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), panel.RuleLine(updated))
		return nil
	},
}

// An abandonment can end an incomplete round. A matching durable event also
// permits recovery when its snapshot rename was interrupted.
func panelAbandonSnapshot(dir, reason string) (*panel.Result, error) {
	res, err := panel.Tally(dir)
	if err != nil {
		return nil, err
	}
	if res.PanelErr != nil {
		return nil, res.PanelErr
	}
	if res.Panel == nil {
		return nil, fmt.Errorf("panel registration is missing")
	}
	if res.IntegrityErr != nil {
		repaired := *res.Panel
		repaired.Abandoned, repaired.AbandonReason = true, reason
		field, err := panel.CheckHandEdits(dir, repaired)
		if err != nil {
			return nil, err
		}
		if field != "" {
			return nil, res.IntegrityErr
		}
	}
	return res, nil
}

func init() {
	panelAbandonCmd.Flags().String("spec", "", "Select the owning spec when the slug is ambiguous")
	panelAbandonCmd.Flags().String("reason", "", "Reason for abandonment")
	panelAbandonCmd.Flags().String("by", "", "Name of the person recording abandonment")
	panelCmd.AddCommand(panelAbandonCmd)
}
