package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panellaunch"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

var panelLaunchCmd = &cobra.Command{
	Use: "launch <slug> <slot>", Short: "Run a process reviewer and submit its verdict",
	Args: cobra.ExactArgs(2), SilenceUsage: true, RunE: runPanelLaunch,
}

var panelLaunchProcessFn = panellaunch.Run
var panelLaunchAcquireSlotLock = panel.AcquireSlotLock

func launchTarget(cmd *cobra.Command, args []string) (string, panel.Registration, panel.SlotLayout, error) {
	root, err := findPanelRoot(config.IsolationRootFor)
	if err != nil {
		return "", panel.Registration{}, panel.SlotLayout{}, err
	}
	if err := validatePanelSlug(args[0]); err != nil {
		return root, panel.Registration{}, panel.SlotLayout{}, err
	}
	spec, _ := cmd.Flags().GetString("spec")
	reg, err := findPanelRegistration(root, args[0], spec)
	if err != nil {
		return root, reg, panel.SlotLayout{}, err
	}
	reg, err = checkoutRegistration(reg.Dir)
	if err != nil {
		return root, reg, panel.SlotLayout{}, err
	}
	iso, err := checkoutIsolation(root)
	if err != nil {
		return root, reg, panel.SlotLayout{}, err
	}
	layout, err := checkoutLayout(root, iso, reg, args[1])
	return root, reg, layout, err
}

func launchResolution(root string, reg panel.Registration, slotID string, replace, checkFinal bool) (*config.Config, panel.SubmitResolution, config.Runner, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, panel.SubmitResolution{}, config.Runner{}, err
	}
	var slot panel.RosterSlot
	for _, s := range reg.Panel.Roster {
		if s.Slot == slotID {
			slot = s
			break
		}
	}
	if slot.Slot == "" {
		return cfg, panel.SubmitResolution{}, config.Runner{}, checkoutFailure("slot is not on the roster", "select a slot from panel verify")
	}
	round, err := panel.RoundSchema(reg.Dir)
	if err != nil {
		return cfg, panel.SubmitResolution{}, config.Runner{}, err
	}
	if round.VerdictSchema != 2 || round.Round != reg.Panel.Round {
		return cfg, panel.SubmitResolution{}, config.Runner{}, checkoutFailure("round does not accept process launches", fmt.Sprintf("mindspec panel create %s --round %d", shellQuoteTarget(reg.Slug()), reg.Panel.Round+1))
	}
	resolved, err := resolveEffectivePanelSlot(cfg, reg.Dir, reg.Panel, slot)
	if err != nil {
		return cfg, resolved, config.Runner{}, err
	}
	runner, _ := cfg.RunnerFor(resolved.Family)
	beadID := ""
	if reg.Panel.BeadID != nil {
		beadID = *reg.Panel.BeadID
	}
	resolved.Landed, err = landedBindingFor(checkoutReadMetadataFn, beadID, reg.Slug(), reg.Panel.Round)
	if err != nil {
		return cfg, resolved, runner, err
	}
	if resolved.Landed {
		return cfg, resolved, runner, checkoutFailure("panel round is already landed", fmt.Sprintf("mindspec panel create %s --round %d", shellQuoteTarget(reg.Slug()), reg.Panel.Round+1))
	}
	if runner.Kind != "process" {
		return cfg, resolved, runner, checkoutFailure("slot uses a harness runner", "mindspec panel prompt "+shellQuoteTarget(reg.Slug())+" --slot "+shellQuoteTarget(slotID)+checkoutSpecFlag(reg))
	}
	if err := panellaunch.Supported(); err != nil {
		return cfg, resolved, runner, err
	}
	if checkFinal && !replace {
		latest, err := panel.LatestSubmissions(reg.Dir, reg.Panel.Round)
		if err != nil {
			return cfg, resolved, runner, err
		}
		if sub := latest[slotID]; sub.Status == "final" && sub.Verdict != "VOID" {
			return cfg, resolved, runner, checkoutFailure("slot already has a final submission", "mindspec panel launch "+shellQuoteTarget(reg.Slug())+" "+shellQuoteTarget(slotID)+" --replace"+checkoutSpecFlag(reg))
		}
	}
	return cfg, resolved, runner, nil
}

func appendLaunchRecord(dir string, record interface{}) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return panel.AppendRecords(dir, raw)
}

func attemptOutcome(dir string, start panel.LaunchRecord, status string) (panel.LaunchRecord, error) {
	out := start
	out.Status, out.CreatedAt = status, time.Now().UTC().Format(time.RFC3339Nano)
	var outputErr error
	if exit, err := panellaunch.ReadExit(dir); err == nil {
		out.ExitCode = &exit.Code
	} else if !os.IsNotExist(err) {
		outputErr = err
	}
	digests, err := panellaunch.Digests(dir)
	outputErr = errors.Join(outputErr, err)
	out.StdoutSHA256, out.StderrSHA256, out.VerdictSHA256 = digests["stdout.log"], digests["stderr.log"], digests["verdict.json"]
	return out, outputErr
}

// checkProcessOrphans runs only under the slot lock. Completed attempts are
// never probed or signaled, even if their old pid now belongs to another group.
func checkProcessOrphans(root string, reg panel.Registration, layout panel.SlotLayout) error {
	round, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(filepath.Dir(layout.Dir)), "r"))
	if err != nil {
		return err
	}
	events, err := panel.SlotAttempts(reg.Dir, round)
	if err != nil {
		return err
	}
	starts, ended := map[int]panel.LaunchRecord{}, map[int]bool{}
	for _, e := range events[filepath.Base(layout.Dir)] {
		if e.Status == "started" {
			starts[e.Attempt] = e.LaunchRecord
		} else {
			ended[e.Attempt] = true
		}
	}
	entries, err := os.ReadDir(layout.Dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	numbers := map[int]bool{}
	for k := range starts {
		numbers[k] = true
	}
	for _, entry := range entries {
		k, err := strconv.Atoi(strings.TrimPrefix(entry.Name(), "attempt-"))
		if err == nil && k > 0 && entry.Name() == fmt.Sprintf("attempt-%d", k) {
			numbers[k] = true
		}
	}
	var ordered []int
	for k := range numbers {
		ordered = append(ordered, k)
	}
	sort.Ints(ordered)
	for _, k := range ordered {
		if ended[k] {
			continue
		}
		dir := layout.AttemptDir(k)
		if err := checkCheckoutPath(root, dir); err != nil {
			return err
		}
		start, registered := starts[k]
		pgid := start.PGID
		if !registered {
			raw, err := panellaunch.ReadAttemptFile(dir, "pgid", 64)
			if err == nil {
				value := strings.TrimSpace(string(raw))
				if value != "" && !strings.ContainsFunc(value, func(r rune) bool { return r < '0' || r > '9' }) {
					pgid, _ = strconv.Atoi(value)
				}
			} else if !os.IsNotExist(err) {
				return checkoutFailure(err.Error(), "inspect the attempt directory and its live supervisor")
			}
		}
		if err := panellaunch.StopOrphan(dir, pgid); err != nil {
			return err
		}
		if registered {
			out, _ := attemptOutcome(dir, start, "launch_failed")
			if err := appendLaunchRecord(reg.Dir, out); err != nil {
				return err
			}
		}
	}
	return nil
}

func launchSubmit(root string, reg panel.Registration, slot string, k int, data []byte, via, reason string) (panel.SubmissionRecord, error) {
	return panel.Submit(panel.SubmitTarget{Dir: reg.Dir, Panel: reg.Slug(), Slot: slot, Round: reg.Panel.Round, Via: via, Attempt: k, Reason: reason}, data, func(p panel.Panel, roster panel.RosterSlot) (panel.SubmitResolution, error) {
		cfg, err := config.Load(root)
		if err != nil {
			return panel.SubmitResolution{}, err
		}
		resolved, err := resolveEffectivePanelSlot(cfg, reg.Dir, p, roster)
		if err != nil {
			return resolved, err
		}
		beadID := ""
		if p.BeadID != nil {
			beadID = *p.BeadID
		}
		resolved.Landed, err = landedBindingFor(checkoutReadMetadataFn, beadID, reg.Slug(), p.Round)
		return resolved, err
	})
}

func launchStatus(cmd *cobra.Command, root string, reg panel.Registration, slot string) error {
	res, err := panel.Tally(reg.Dir)
	if err != nil {
		return err
	}
	statuses, err := panelSlotStatuses(root, res)
	if err != nil {
		return err
	}
	for _, s := range statuses {
		if s.Slot == slot {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", termsafe.Escape(s.Slot), s.Status)
		}
	}
	return nil
}

// retryRefusal bounds the JSON pointer while preserving the refusal and recovery.
func retryRefusal(message string) string {
	pointer, rest, ok := strings.Cut(message, ": ")
	// Verdict errors may already contain a terminal-safe quoted pointer.
	if strings.HasPrefix(message, `"`) {
		if quoted, err := strconv.QuotedPrefix(message); err == nil {
			if decoded, err := strconv.Unquote(quoted); err == nil {
				pointer = decoded
				rest, ok = strings.CutPrefix(message[len(quoted):], ": ")
			}
		}
	}
	if ok && strings.HasPrefix(pointer, "/") && len(pointer) > 200 {
		message = strings.ToValidUTF8(pointer[:200], "") + ": " + rest
	}
	return termsafe.Escape(message)
}

func runPanelLaunch(cmd *cobra.Command, args []string) (runErr error) {
	defer func() { runErr = checkoutError(runErr, "run mindspec panel verify, then repeat panel launch") }()
	root, reg, layout, err := launchTarget(cmd, args)
	if err != nil {
		return err
	}
	replace, _ := cmd.Flags().GetBool("replace")
	if _, _, _, err := launchResolution(root, reg, args[1], replace, true); err != nil {
		return err
	}
	if err := checkCheckoutPath(root, filepath.Dir(layout.Lock)); err != nil {
		return err
	}
	lock, err := panelLaunchAcquireSlotLock(layout.Lock)
	if err != nil {
		return err
	}
	defer lock.Release()
	current, err := checkoutRegistration(reg.Dir)
	if err != nil {
		return err
	}
	if current.Panel.Round != reg.Panel.Round || current.Panel.ReviewedHeadSHA != reg.Panel.ReviewedHeadSHA {
		return checkoutFailure("panel changed while launch waited", "repeat panel launch for the current round")
	}
	if _, _, _, err := launchResolution(root, reg, args[1], replace, true); err != nil {
		return err
	}
	if err := ensureSlotCheckout(root, reg, layout, args[1]); err != nil {
		return err
	}
	if err := checkProcessOrphans(root, reg, layout); err != nil {
		return err
	}
	retried, refusal := false, ""
	for {
		cfg, resolved, runner, err := launchResolution(root, reg, args[1], true, false)
		if err != nil {
			return err
		}
		in, err := resolvePanelPrompt(root, reg, args[1], cfg)
		if err != nil {
			return err
		}
		dir := layout.AttemptDir(in.Attempt)
		if err := os.Mkdir(dir, 0o700); err != nil {
			return err
		}
		prompt := panel.RenderSlotPrompt(in)
		if refusal != "" {
			prompt += "\nPrevious output refused: " + refusal + "\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(prompt), 0o600); err != nil {
			return err
		}
		argv := append([]string(nil), runner.Argv...)
		values := map[string]string{"{prompt}": prompt, "{prompt_file}": filepath.Join(dir, "prompt.md"), "{model}": resolved.Model, "{slot_dir}": layout.Dir, "{tree}": layout.Tree}
		for i, arg := range argv {
			if value, ok := values[arg]; ok {
				argv[i] = value
			}
		}
		template, _ := json.Marshal(runner.Argv)
		start := panel.LaunchRecord{Record: panel.RecordLaunch, Spec: panelRecordSpec(reg.Panel), Panel: reg.Slug(), Slot: args[1], Round: reg.Panel.Round, Attempt: in.Attempt, Family: resolved.Family, Model: resolved.Model, Status: "started", RunnerKind: "process", RunnerSource: resolved.RunnerSource, ArgvSHA256: panellaunch.Hash(template), NonceSHA256: panellaunch.Hash([]byte(in.Nonce)), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		timeout := time.Hour
		if runner.TimeoutMinutes != nil {
			timeout = time.Duration(*runner.TimeoutMinutes) * time.Minute
		}
		err = panelLaunchProcessFn(panellaunch.Request{Dir: dir, SlotDir: layout.Dir, Argv: argv, Prompt: runner.Prompt, Timeout: timeout, Register: func(spawn func() (int, error)) error {
			records := [][]byte{nil}
			return panel.AppendRecordsWithCheck(reg.Dir, func() error {
				latest, err := checkoutRegistration(reg.Dir)
				if err != nil {
					return err
				}
				if latest.Panel.Round != reg.Panel.Round {
					return checkoutFailure("panel round changed", "repeat panel launch")
				}
				if _, _, _, err := launchResolution(root, latest, args[1], true, false); err != nil {
					return err
				}
				pid, err := spawn()
				if err != nil {
					return err
				}
				start.PGID = pid
				records[0], err = json.Marshal(start)
				return err
			}, records...)
		}})
		if err != nil {
			return err
		}
		out, outputErr := attemptOutcome(dir, start, "launch_failed")
		exit, exitErr := panellaunch.ReadExit(dir)
		status := out.Status
		var selected panel.SubmissionRecord
		var verdict panel.VerdictV2
		if exitErr == nil && exit.Code == 0 && !exit.Timeout {
			var data []byte
			err := outputErr
			if err == nil {
				data, err = panellaunch.Verdict(dir, runner.Output, start.NonceSHA256)
			}
			if err == nil {
				selected, err = launchSubmit(root, reg, args[1], in.Attempt, data, "launch", "")
			}
			if err != nil {
				status = "malformed"
				refusal = retryRefusal(err.Error())
				fmt.Fprintln(cmd.OutOrStdout(), refusal)
			} else {
				status = "submitted"
				verdict, _ = panel.ParseVerdictV2(data)
			}
		} else if exitErr == nil && panellaunch.Quota(dir, runner.QuotaSignals) {
			status = "quota_wall"
		}
		out.Status, out.Seq = status, selected.Seq
		if err := appendLaunchRecord(reg.Dir, out); err != nil {
			return err
		}
		if status == "malformed" && !retried {
			retried = true
			continue
		}
		if status == "quota_wall" && runner.OnQuota != "" {
			lines, err := panel.ReadLedger(reg.Dir)
			if err != nil {
				return err
			}
			substituted := false
			for _, line := range lines {
				var sub panel.SubstitutionRecord
				if json.Unmarshal(line, &sub) == nil && sub.Record == panel.RecordSubstitution && sub.Round == reg.Panel.Round && sub.Slot == args[1] {
					substituted = true
				}
			}
			if !substituted {
				sub := panel.SubstitutionRecord{Record: panel.RecordSubstitution, Spec: panelRecordSpec(reg.Panel), Panel: reg.Slug(), Slot: args[1], Round: reg.Panel.Round, FromFamily: resolved.Family, ToFamily: runner.OnQuota, Model: cfg.SubstituteModel(runner.OnQuota), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				if err := appendLaunchRecord(reg.Dir, sub); err != nil {
					return err
				}
				next, _ := cfg.RunnerFor(runner.OnQuota)
				if next.Kind == "process" {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "mindspec panel checkout %s --slot %s --dispatch%s\nmindspec panel prompt %s --slot %s%s\n", shellQuoteTarget(reg.Slug()), shellQuoteTarget(args[1]), checkoutSpecFlag(reg), shellQuoteTarget(reg.Slug()), shellQuoteTarget(args[1]), checkoutSpecFlag(reg))
			}
		}
		if err := launchStatus(cmd, root, reg, args[1]); err != nil {
			return err
		}
		if status == "submitted" && verdict.Status == "final" && verdict.Verdict != "VOID" {
			return nil
		}
		return checkoutFailure("reviewer did not submit a usable final verdict", "run mindspec panel verify for the slot status")
	}
}

func init() {
	panelLaunchCmd.Flags().String("spec", "", "Select the owning spec")
	panelLaunchCmd.Flags().Bool("replace", false, "Start a replacement for a final verdict")
	panelCmd.AddCommand(panelLaunchCmd)
	panelCheckoutOrphanCheck = checkProcessOrphans
}
