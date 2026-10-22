package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

const localConfigPath = ".mindspec/config.local.yaml"
const runnerProbeTimeout = 10 * time.Second

// checkConfiguration shares one load across all config-dependent checks.
func checkConfiguration(r *Report, root string, opts Options) {
	cfg, err := config.Load(root)
	if err != nil {
		message := err.Error()
		if !strings.Contains(message, "recovery:") {
			message += "\nrecovery: repair the config file and run mindspec doctor"
		}
		r.Checks = append(r.Checks, Check{Name: "MindSpec config", Status: Error, Message: message})
		return
	}
	checkSourceGlobs(r, root, cfg)
	checkModels(r, root, cfg)
	checkCommands(r, root, cfg)
	git := executor.NewMindspecExecutor(root)
	tracked, trackErr := git.LsFiles(root, localConfigPath)
	checkPanelConfigWarnings(r, root, cfg, git, tracked)
	checkPanelRunners(r, root, cfg, opts, tracked, trackErr)
	if !opts.SkipLocalEnv {
		checkStaleCheckouts(r, root, cfg)
	}
}

// Only these read-only repository operations are needed by config diagnostics.
type configRepository interface {
	CheckIgnore(workdir, file string) error
}

func checkPanelConfigWarnings(r *Report, root string, cfg *config.Config, git configRepository, tracked bool) {
	if tracked {
		r.Checks = append(r.Checks, Check{Name: "Local panel config", Status: Warn, Message: localConfigPath + " is tracked; keep machine-local config untracked\nrecovery: untrack the local config file and run mindspec init"})
	}
	if _, err := os.Lstat(filepath.Join(root, localConfigPath)); err == nil && git.CheckIgnore(root, localConfigPath) != nil {
		r.Checks = append(r.Checks, Check{Name: "Local config ignore", Status: Warn, Message: localConfigPath + " is not ignored\nrecovery: mindspec init"})
	}
	if git.CheckIgnore(root, strings.TrimSuffix(cfg.Panel.IsolationRoot, "/")+"/") != nil {
		r.Checks = append(r.Checks, Check{Name: "Panel isolation ignore", Status: Warn, Message: cfg.Panel.IsolationRoot + " is not ignored\nrecovery: mindspec init"})
	}
}

func checkPanelRunners(r *Report, root string, cfg *config.Config, opts Options, tracked bool, trackErr error) {
	for _, message := range cfg.ModelOnlyWarnings() {
		r.Checks = append(r.Checks, Check{Name: "Panel reviewer", Status: Warn, Message: message})
	}
	used := map[string]bool{}
	seenErrors := map[string]bool{}
	addResolutionError := func(err error) {
		if !seenErrors[err.Error()] {
			r.Checks = append(r.Checks, Check{Name: "Panel reviewers", Status: Error, Message: err.Error()})
			seenErrors[err.Error()] = true
		}
	}
	checkEntries := func(label string, entries []config.Reviewer) {
		for i, entry := range entries {
			if entry.Family == "" {
				continue
			}
			if _, ok := cfg.RunnerFor(entry.Family); !ok {
				addResolutionError(fmt.Errorf("%s[%d] names family %q with no runner\nrecovery: add a runner for the family to panel.runners", label, i, entry.Family))
			}
		}
	}
	checkEntries("panel.reviewers", cfg.Panel.Reviewers)
	for _, gate := range config.PanelGateKeys {
		checkEntries("panel.gates."+gate+".reviewers", cfg.Panel.Gates[gate].Reviewers)
	}
	for _, gate := range config.PanelGateKeys {
		slots, err := cfg.ResolveSlots(gate)
		if err != nil {
			addResolutionError(err)
			continue
		}
		for _, slot := range slots {
			used[slot.Family] = true
		}
	}
	if opts.SkipLocalEnv {
		return
	}
	for _, message := range cfg.ToolSuggestions() {
		r.Checks = append(r.Checks, Check{Name: "Review diversity", Status: Info, Message: message})
	}
	for _, runner := range cfg.Panel.Runners {
		if runner.Kind != "process" {
			continue
		}
		if used[runner.Family] {
			status, message := OK, fmt.Sprintf("runner %q command %q is on PATH", runner.Family, runner.Argv[0])
			if _, err := exec.LookPath(runner.Argv[0]); err != nil {
				status = Error
				message = fmt.Sprintf("runner %q command %q is not on PATH\nrecovery: install the command or update argv in panel.runners", runner.Family, runner.Argv[0])
			}
			r.Checks = append(r.Checks, Check{Name: "Panel launcher", Status: status, Message: message})
		}
		if len(runner.Probe) == 0 {
			continue
		}
		local := cfg.RunnerSource(runner.Family) == "config.local.yaml"
		if !opts.Probe && (!local || tracked || trackErr != nil) {
			r.Checks = append(r.Checks, Check{Name: "Panel probe", Status: Info, Message: fmt.Sprintf("runner %q probe was not run; use mindspec doctor --probe to run it", runner.Family)})
			continue
		}
		checkRunnerProbe(r, root, runner)
	}
}

func checkRunnerProbe(r *Report, root string, runner config.Runner) {
	ctx, cancel := context.WithTimeout(context.Background(), runnerProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, runner.Probe[0], runner.Probe[1:]...)
	cmd.Dir = root
	err := cmd.Run()
	status, message := OK, fmt.Sprintf("runner %q probe passed", runner.Family)
	if err != nil {
		status = Error
		if ctx.Err() != nil {
			message = fmt.Sprintf("runner %q probe did not exit within 10 seconds", runner.Family)
		} else {
			message = fmt.Sprintf("runner %q probe failed: %v", runner.Family, err)
		}
		message += "\nrecovery: repair the runner probe in panel.runners"
	}
	r.Checks = append(r.Checks, Check{Name: "Panel probe", Status: status, Message: message})
}
