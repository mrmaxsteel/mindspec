package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

const checkoutFixerSlot = "fixer"

var checkoutReadMetadataFn = bead.GetMetadata

// Bead 7 connects the process orphan check here. Callers hold all affected slot
// locks before this hook runs and before taking the ledger lock.
var panelCheckoutOrphanCheck = func(root string, reg panel.Registration, layout panel.SlotLayout) error { return nil }

func checkoutFailure(message, recovery string) error {
	return termsafe.NewFailure(message, recovery)
}

func checkoutError(err error, recovery string) error {
	if err == nil {
		return err
	}
	var failure termsafe.RecoveryFailure
	if errors.As(err, &failure) {
		return failure
	}
	return checkoutFailure(err.Error(), recovery)
}

func checkoutSpecFlag(reg panel.Registration) string {
	if reg.Panel.Spec == "" || reg.Panel.Spec == "adhoc" {
		return ""
	}
	return " --spec " + shellQuoteTarget(reg.Panel.Spec)
}

func checkoutPruneRecovery(slug, slot string) string {
	return "mindspec panel checkout " + shellQuoteTarget(slug) + " --prune --slot " + shellQuoteTarget(slot)
}

// checkCheckoutPath never follows a symlink below the main working tree.
// Missing components are allowed, so callers can validate before mkdir.
func checkCheckoutPath(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("scratch path is outside the main root")
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("scratch component is not a real directory: %s", termsafe.Escape(current))
		}
	}
	return nil
}

func checkoutIsolation(root string) (string, error) {
	isolation, err := config.IsolationRootFor(root)
	if err != nil {
		return "", err
	}
	if err := checkCheckoutPath(root, filepath.Join(root, isolation)); err != nil {
		return "", checkoutFailure(err.Error(), "use a real, ignored directory for panel.isolation_root, then run mindspec init")
	}
	if err := newExecutor(root).CheckSlotIgnore(root, isolation); err != nil {
		return "", checkoutFailure("panel isolation root is not ignored or cannot be checked", "mindspec init")
	}
	return isolation, nil
}

func checkoutRegistration(dir string) (panel.Registration, error) {
	data, err := os.ReadFile(filepath.Join(dir, panel.FileName))
	if err != nil {
		return panel.Registration{}, err
	}
	reg := panel.Registration{Dir: dir}
	if err := json.Unmarshal(data, &reg.Panel); err != nil {
		return reg, err
	}
	if reg.Panel.Abandoned {
		return reg, checkoutFailure("panel is abandoned", fmt.Sprintf("mindspec panel create %s --round %d", shellQuoteTarget(reg.Slug()), reg.Panel.Round+1))
	}
	if field, err := panel.CheckHandEdits(dir, reg.Panel); err != nil {
		return reg, err
	} else if field != "" {
		return reg, checkoutFailure("panel registration differs from its ledger: "+field, "repair the panel registration with mindspec panel create")
	}
	return reg, nil
}

func checkoutLayout(root, isolation string, reg panel.Registration, slot string) (panel.SlotLayout, error) {
	return panel.NewSlotLayout(root, isolation, reg.Panel.Spec, reg.Slug(), reg.Panel.Round, slot)
}

// ensureSlotCheckout is also the launch seam: the caller holds the slot lock.
func ensureSlotCheckout(root string, reg panel.Registration, layout panel.SlotLayout, slot string) error {
	recovery := checkoutPruneRecovery(reg.Slug(), slot)
	if err := checkCheckoutPath(root, layout.Dir); err != nil {
		return checkoutFailure(err.Error(), recovery)
	}
	metadata := panel.SlotMetadata{Spec: panelRecordSpec(reg.Panel), Slug: reg.Slug(), Round: reg.Panel.Round, Slot: slot, MainRoot: root}
	if _, err := os.Lstat(layout.Metadata); err == nil {
		raw, err := workspace.ReadSlotMetadata(layout.Metadata)
		if err != nil {
			return checkoutFailure(err.Error(), recovery)
		}
		var old panel.SlotMetadata
		if json.Unmarshal(raw, &old) != nil || old != metadata {
			return checkoutFailure("slot metadata differs from the selected panel", recovery)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Lstat(layout.Tree); err == nil {
		if err := newExecutor(root).CheckSlotTree(layout.Tree, reg.Panel.ReviewedHeadSHA); err != nil {
			return checkoutFailure(err.Error(), recovery)
		}
	} else if !os.IsNotExist(err) {
		return err
	} else {
		if err := os.MkdirAll(layout.Dir, 0o700); err != nil {
			return err
		}
		if err := newExecutor(root).CloneSlotTree(layout.Tree, reg.Panel.ReviewedHeadSHA); err != nil {
			return checkoutFailure(err.Error(), recovery)
		}
	}
	if _, err := os.Lstat(layout.Metadata); os.IsNotExist(err) {
		return panel.WriteSlotMetadata(layout, metadata)
	}
	return nil
}

func checkoutDispatchResolution(root string, reg panel.Registration, slot panel.RosterSlot, replace bool) (config.Runner, string, string, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return config.Runner{}, "", "", err
	}
	family, model := slot.Family, slot.Model
	if family == "" {
		family = "harness"
	}
	lines, err := panel.ReadLedger(reg.Dir)
	if err != nil {
		return config.Runner{}, "", "", err
	}
	for _, line := range lines {
		var sub panel.SubstitutionRecord
		if err := json.Unmarshal(line, &sub); err != nil {
			return config.Runner{}, "", "", err
		}
		if sub.Record == panel.RecordSubstitution && sub.Round == reg.Panel.Round && sub.Slot == slot.Slot {
			family, model = sub.ToFamily, sub.Model
		}
	}
	runner, ok := cfg.RunnerFor(family)
	if !ok {
		return runner, "", "", checkoutFailure("slot family has no runner", "add a runner for the family to panel.runners")
	}
	if runner.Kind != "harness" {
		recovery := "mindspec panel launch " + shellQuoteTarget(reg.Slug()) + " " + shellQuoteTarget(slot.Slot) + checkoutSpecFlag(reg)
		return runner, "", "", checkoutFailure("slot uses a process runner", recovery)
	}
	latest, err := panel.LatestSubmissions(reg.Dir, reg.Panel.Round)
	if err != nil {
		return runner, "", "", err
	}
	if submission, ok := latest[slot.Slot]; ok && submission.Status == "final" && submission.Verdict != "VOID" && !replace {
		return runner, "", "", checkoutFailure("slot already has a final submission", "mindspec panel checkout "+shellQuoteTarget(reg.Slug())+" --slot "+shellQuoteTarget(slot.Slot)+" --dispatch --replace"+checkoutSpecFlag(reg))
	}
	beadID := ""
	if reg.Panel.BeadID != nil {
		beadID = *reg.Panel.BeadID
	}
	landed, err := landedBindingFor(checkoutReadMetadataFn, beadID, reg.Slug(), reg.Panel.Round)
	if err != nil {
		return runner, "", "", err
	}
	if landed {
		return runner, "", "", checkoutFailure("panel round is already landed", fmt.Sprintf("mindspec panel create %s --round %d", shellQuoteTarget(reg.Slug()), reg.Panel.Round+1))
	}
	return runner, model, cfg.RunnerSource(family), nil
}

func dispatchSlot(root string, reg panel.Registration, layout panel.SlotLayout, slot panel.RosterSlot, replace bool) error {
	runner, model, source, err := checkoutDispatchResolution(root, reg, slot, replace)
	if err != nil {
		return err
	}
	before, err := panel.ReadLedger(reg.Dir)
	if err != nil {
		return err
	}
	k, err := panel.NextAttemptNumber(reg.Dir, reg.Panel.Round, slot.Slot, layout.Dir)
	if err != nil {
		return err
	}
	attempts, err := panel.SlotAttempts(reg.Dir, reg.Panel.Round)
	if err != nil {
		return err
	}
	last := map[int]panel.LaunchRecord{}
	for _, attempt := range attempts[slot.Slot] {
		last[attempt.Attempt] = attempt.LaunchRecord
	}
	var numbers []int
	for number, event := range last {
		if event.Status == "dispatched" {
			numbers = append(numbers, number)
		}
	}
	sort.Ints(numbers)
	var records [][]byte
	for _, number := range numbers {
		event := last[number]
		event.Status, event.CreatedAt = "launch_failed", time.Now().UTC().Format(time.RFC3339Nano)
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		records = append(records, raw)
	}
	event := panel.LaunchRecord{Record: panel.RecordLaunch, Spec: panelRecordSpec(reg.Panel), Panel: reg.Slug(), Slot: slot.Slot, Round: reg.Panel.Round, Attempt: k, Family: runner.Family, Model: model, Status: "dispatched", RunnerKind: runner.Kind, RunnerSource: source, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	records = append(records, raw)
	return panel.AppendRecordsWithCheck(reg.Dir, func() error {
		current, err := checkoutRegistration(reg.Dir)
		if err != nil {
			return err
		}
		if current.Panel.Round != reg.Panel.Round {
			return checkoutFailure("panel round changed during checkout", "repeat panel checkout for the current round")
		}
		if _, _, _, err := checkoutDispatchResolution(root, current, slot, replace); err != nil {
			return err
		}
		after, err := panel.ReadLedger(reg.Dir)
		if err != nil {
			return err
		}
		if !bytes.Equal(bytes.Join(before, []byte{'\n'}), bytes.Join(after, []byte{'\n'})) {
			return checkoutFailure("panel changed during dispatch", "repeat panel checkout --dispatch")
		}
		// Exclusive mkdir also reserves attempts left by an interrupted dispatch.
		return os.Mkdir(layout.AttemptDir(k), 0o700)
	}, records...)
}

var panelCheckoutCmd = &cobra.Command{
	Use:   "checkout <slug> (--slot S | --all | --fixer | --prune)",
	Short: "Create isolated reviewer checkouts or remove panel scratch",
	Args:  cobra.ExactArgs(1), SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		defer func() {
			runErr = checkoutError(runErr, "check the panel and scratch paths, then repeat panel checkout")
		}()
		slotName, _ := cmd.Flags().GetString("slot")
		all, _ := cmd.Flags().GetBool("all")
		fixer, _ := cmd.Flags().GetBool("fixer")
		prune, _ := cmd.Flags().GetBool("prune")
		dispatch, _ := cmd.Flags().GetBool("dispatch")
		replace, _ := cmd.Flags().GetBool("replace")
		modes := 0
		for _, selected := range []bool{all, fixer, prune, slotName != "" && !prune} {
			if selected {
				modes++
			}
		}
		if modes != 1 || (dispatch && (slotName == "" || prune)) || (replace && !dispatch) {
			return checkoutFailure("invalid checkout flags", "select --slot, --all, --fixer or --prune; --dispatch needs --slot; --replace needs --dispatch")
		}
		root, err := findPanelRoot(config.IsolationRootFor)
		if err != nil {
			return err
		}
		if err := validatePanelSlug(args[0]); err != nil {
			return err
		}
		spec, _ := cmd.Flags().GetString("spec")
		reg, err := findPanelRegistration(root, args[0], spec)
		if err != nil {
			return err
		}
		if dispatch {
			if _, err := panel.ReadFilledBrief(filepath.Join(reg.Dir, "BRIEF.md")); err != nil {
				return err
			}
		}
		isolation, err := checkoutIsolation(root)
		if err != nil {
			return err
		}
		if prune {
			release, err := cleanupPanelCheckouts(root, reg, isolation, 0, slotName)
			if release != nil {
				defer release()
			}
			return err
		}
		reg, err = checkoutRegistration(reg.Dir)
		if err != nil {
			return err
		}
		var slots []panel.RosterSlot
		if fixer {
			slots = []panel.RosterSlot{{Slot: checkoutFixerSlot}}
		} else {
			for _, s := range reg.Panel.Roster {
				if all || s.Slot == slotName {
					slots = append(slots, s)
				}
			}
			if len(slots) == 0 {
				return checkoutFailure("slot is not on the panel roster", "select a slot from panel verify")
			}
		}
		for _, slot := range slots {
			layout, err := checkoutLayout(root, isolation, reg, slot.Slot)
			if err != nil {
				return err
			}
			if err := checkCheckoutPath(root, filepath.Dir(layout.Lock)); err != nil {
				return err
			}
			lock, err := panel.AcquireSlotLock(layout.Lock)
			if err != nil {
				return err
			}
			err = func() error {
				defer lock.Release()
				current, err := checkoutRegistration(reg.Dir)
				if err != nil {
					return err
				}
				if current.Panel.Round != reg.Panel.Round || current.Panel.ReviewedHeadSHA != reg.Panel.ReviewedHeadSHA {
					return checkoutFailure("panel changed while checkout waited for its lock", "repeat panel checkout for the current round")
				}
				if dispatch {
					if _, _, _, err := checkoutDispatchResolution(root, reg, slot, replace); err != nil {
						return err
					}
				}
				if fixer {
					if err := checkCheckoutPath(root, layout.Dir); err != nil {
						return err
					}
					return os.MkdirAll(layout.Dir, 0o700)
				}
				if err := ensureSlotCheckout(root, reg, layout, slot.Slot); err != nil {
					return err
				}
				if dispatch {
					return dispatchSlot(root, reg, layout, slot, replace)
				}
				return nil
			}()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), layout.Dir)
		}
		return nil
	},
}

// cleanupPanelCheckouts takes every slot lock before any removal. It retains
// them until the calling verb completes its ledger and snapshot writes.
// beforeRound == 0 selects every round. Lock files live outside these trees.
func cleanupPanelCheckouts(root string, reg panel.Registration, isolation string, beforeRound int, onlySlot string) (func(), error) {
	ignored := newExecutor(root).CheckSlotIgnore(root, isolation) == nil
	layout, err := checkoutLayout(root, isolation, reg, checkoutFixerSlot)
	if err != nil {
		return nil, err
	}
	base := filepath.Dir(filepath.Dir(layout.Dir))
	if err := checkCheckoutPath(root, base); err != nil {
		return nil, err
	}
	rounds, err := os.ReadDir(base)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// Checkout takes its lock before it creates the slot directory.
	// Include each registered round even when its scratch path is absent.
	roundSet := make(map[int]bool)
	for n := 1; n <= reg.Panel.Round; n++ {
		if beforeRound == 0 || n < beforeRound {
			roundSet[n] = true
		}
	}
	for _, round := range rounds {
		n, err := strconv.Atoi(strings.TrimPrefix(round.Name(), "r"))
		if err == nil && n > 0 && round.Name() == fmt.Sprintf("r%d", n) && (beforeRound == 0 || n < beforeRound) {
			roundSet[n] = true
		}
	}
	var affected []panel.SlotLayout
	for n := range roundSet {
		roundDir := filepath.Join(base, fmt.Sprintf("r%d", n))
		if err := checkCheckoutPath(root, roundDir); err != nil {
			return nil, err
		}
		slots, err := os.ReadDir(roundDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		slotSet := map[string]bool{checkoutFixerSlot: true}
		for _, slot := range reg.Panel.Roster {
			slotSet[slot.Slot] = true
		}
		for _, slot := range slots {
			slotSet[slot.Name()] = true
		}
		for slot := range slotSet {
			if onlySlot != "" && slot != onlySlot {
				continue
			}
			entry, err := panel.NewSlotLayout(root, isolation, reg.Panel.Spec, reg.Slug(), n, slot)
			if err != nil {
				return nil, err
			}
			if !ignored {
				// Use existing lock files, even before a slot directory exists.
				// If the ignore check fails, do not create lock files.
				if _, err := os.Lstat(entry.Lock); os.IsNotExist(err) {
					continue
				} else if err != nil {
					return nil, err
				}
			}
			affected = append(affected, entry)
		}
	}
	sort.Slice(affected, func(i, j int) bool {
		a, b := filepath.Base(affected[i].Dir), filepath.Base(affected[j].Dir)
		if a != b {
			return a < b
		}
		return affected[i].Dir < affected[j].Dir
	})
	var locks []*panel.SlotLock
	release := func() {
		for i := len(locks) - 1; i >= 0; i-- {
			_ = locks[i].Release()
		}
	}
	var failures, held []string
	for _, entry := range affected {
		if err := checkCheckoutPath(root, filepath.Dir(entry.Lock)); err != nil {
			release()
			return nil, err
		}
		lock, err := panel.AcquireSlotLock(entry.Lock)
		if err != nil {
			message := err.Error()
			if errors.Is(err, panel.ErrSlotLocked) {
				message = fmt.Sprintf("%s: %s", panel.ErrSlotLocked, entry.Lock)
			}
			failures = append(failures, message)
			held = append(held, shellQuoteTarget(entry.Lock))
			continue
		}
		locks = append(locks, lock)
	}
	if len(failures) > 0 {
		release()
		recovery := "mindspec panel verify " + shellQuoteTarget(reg.Slug()) + checkoutSpecFlag(reg) + " # wait for slot locks: " + strings.Join(held, ", ")
		return nil, checkoutFailure(strings.Join(failures, "\n"), recovery)
	}
	var orphanFailures []error
	for _, entry := range affected {
		if err := panelCheckoutOrphanCheck(root, reg, entry); err != nil {
			orphanFailures = append(orphanFailures, err)
		}
	}
	if len(orphanFailures) > 0 {
		release()
		return nil, termsafe.JoinFailures(orphanFailures, "inspect the attempt directory and its live supervisor")
	}

	// This is the same lock inode as the panel writer. Release it before the
	// writer runs; the slot locks stay held through the caller's operation.
	if len(affected) > 0 {
		ledgerLock, err := panel.AcquireSlotLock(filepath.Join(reg.Dir, "dispositions.lock"))
		if err != nil {
			release()
			return nil, err
		}
		defer ledgerLock.Release()
	}
	for _, entry := range affected {
		// RemoveAll unlinks a symlink leaf; it does not traverse its target.
		if err := checkCheckoutPath(root, filepath.Dir(entry.Dir)); err != nil {
			release()
			return nil, err
		}
		if err := os.RemoveAll(entry.Dir); err != nil {
			release()
			return nil, err
		}
	}
	return release, nil
}

func cleanupPriorRound(root, dir string, round int) (func(), error) {
	data, err := os.ReadFile(filepath.Join(dir, panel.FileName))
	if os.IsNotExist(err) {
		return func() {}, nil
	}
	if err != nil {
		return nil, err
	}
	reg := panel.Registration{Dir: dir}
	if err := json.Unmarshal(data, &reg.Panel); err != nil {
		return nil, err
	}
	if round <= reg.Panel.Round {
		return func() {}, nil
	}
	isolation, err := config.IsolationRootFor(root)
	if err != nil {
		return nil, err
	}
	return cleanupPanelCheckouts(root, reg, isolation, round, "")
}

func init() {
	panelCheckoutCmd.Flags().String("spec", "", "Select the owning spec")
	panelCheckoutCmd.Flags().String("slot", "", "Select one roster slot")
	panelCheckoutCmd.Flags().Bool("all", false, "Create every roster slot")
	panelCheckoutCmd.Flags().Bool("dispatch", false, "Record a new harness attempt")
	panelCheckoutCmd.Flags().Bool("replace", false, "Permit dispatch after a final submission")
	panelCheckoutCmd.Flags().Bool("fixer", false, "Create fixer scratch without a tree")
	panelCheckoutCmd.Flags().Bool("prune", false, "Remove panel checkouts, including dirty files")
	panelCmd.AddCommand(panelCheckoutCmd)
}
