package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

func resolveFixture(t *testing.T, main, local string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"config.yaml": main, "config.local.yaml": local} {
		if body != "" {
			if err := os.WriteFile(filepath.Join(root, ".mindspec", name), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	resetConfigShowGateFlags(t)
	reset := func() { configResolveCmd.Flags().Set("gate", ""); configResolveCmd.Flags().Set("json", "false") }
	reset()
	t.Cleanup(reset)
	return root
}

func runConfigCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs(append([]string{"config"}, args...))
	err := rootCmd.Execute()
	return out.String(), errOut.String(), err
}

func TestConfigResolve_Provenance(t *testing.T) {
	root := resolveFixture(t, "panel:\n  runners: [{family: b, kind: process, argv: [run], prompt: stdin, output: stdout}]\n  reviewers: [{family: b, model: entry}, {family: b}, {model: m7}]\n  gates: {bead: {lenses: [adversarial]}}\nmodels: {reviewer.b: main}\n", "panel:\n  runners: [{family: b, kind: harness}]\nmodels: {reviewer.b: local}\n")
	before := listAllFiles(t, root)
	out, _, err := runConfigCLI(t, "resolve", "--gate", "bead", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var slots []config.ResolvedSlot
	if err := json.Unmarshal([]byte(out), &slots); err != nil {
		t.Fatal(err)
	}
	if len(slots) != 3 || slots[0].Family != "b" || slots[0].Model != "entry" || slots[0].Kind != "harness" || slots[1].Model != "local" || slots[2].Family != "harness" {
		t.Fatalf("resolved: %s", out)
	}
	if !reflect.DeepEqual(slots[0].Sources, map[string]string{"slot": "default", "family": "config.yaml", "model": "config.yaml", "lens": "config.yaml", "kind": "config.local.yaml"}) || slots[1].Sources["model"] != "config.local.yaml" || slots[2].Sources["family"] != "default" {
		t.Fatalf("sources: %s", out)
	}
	configResolveCmd.Flags().Set("json", "false")
	out, _, err = runConfigCLI(t, "resolve", "--gate", "bead")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Preview of a new panel", "entry (config.yaml)", "local (config.local.yaml)", "harness (default)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s: %s", want, out)
		}
	}
	if after := listAllFiles(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("resolve wrote files")
	}
	t.Run("runner family without reviewer list", func(t *testing.T) {
		resolveFixture(t, "panel:\n  runners: [{family: b, kind: harness}]\n", "")
		out, _, err := runConfigCLI(t, "resolve", "--gate", "bead", "--json")
		if err != nil {
			t.Fatal(err)
		}
		var slots []config.ResolvedSlot
		if err := json.Unmarshal([]byte(out), &slots); err != nil {
			t.Fatal(err)
		}
		if len(slots) != 4 || slots[0].Family != "harness" || slots[0].Sources["family"] != "default" || slots[1].Family != "b" || slots[1].Sources["family"] != "config.yaml" || slots[3].Sources["family"] != "config.yaml" {
			t.Fatalf("runner family sources: %s", out)
		}
	})
}

func TestReviewer_ModelOnlyWarn(t *testing.T) {
	resolveFixture(t, "panel:\n  reviewers: [{model: m7, count: 2}]\n", "")
	out, warnings, err := runConfigCLI(t, "resolve", "--gate", "bead")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(warnings, "Warn:") != 1 || !strings.Contains(warnings, "panel.reviewers[0] runs on the harness runner; add a family and a runner") || !strings.Contains(out, "m7") {
		t.Fatalf("output: %s\n%s", out, warnings)
	}
	root := setupCreateIntegrity(t)
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("panel:\n  reviewers: [{model: m7, count: 2}]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	config.ResetCache()
	if out, err := runPanelVerbCmd("create", "p", "--spec", "130-test", "--gate", "bead", "--target", "bead/probe-x.1"); err != nil {
		t.Fatalf("create: %v %s", err, out)
	}
	p, _ := readCreatedPanel(t, root)
	if len(p.Roster) != 2 || p.Roster[0].Model != "m7" {
		t.Fatal(p.Roster)
	}
}

func TestCreate_RefusesUnrunnableFamily(t *testing.T) {
	for _, reviewer := range []string{"{family: x, count: 2}", "{model: '-x', count: 2}"} {
		t.Run(reviewer, func(t *testing.T) {
			root := setupCreateIntegrity(t)
			if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("panel:\n  reviewers: ["+reviewer+"]\n"), 0644); err != nil {
				t.Fatal(err)
			}
			config.ResetCache()
			before := panelTreeBytes(t, root)
			_, err := runPanelVerbCmd("create", "p", "--spec", "130-test", "--gate", "bead", "--target", "bead/probe-x.1")
			if err == nil {
				t.Fatal("create accepted invalid reviewer")
			}
			if strings.Contains(reviewer, "family") && (!strings.Contains(err.Error(), "panel.reviewers[0]") || !strings.Contains(err.Error(), "add a runner for the family to panel.runners")) {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, panelTreeBytes(t, root)) {
				t.Fatal("refusal changed files")
			}
			configResolveCmd.Flags().Set("gate", "")
			configResolveCmd.Flags().Set("json", "false")
			_, _, err = runConfigCLI(t, "resolve", "--gate", "bead")
			if err == nil {
				t.Fatal("resolve accepted invalid reviewer")
			}
			if strings.Contains(reviewer, "family") && !strings.Contains(err.Error(), "panel.runners") {
				t.Fatal(err)
			}
		})
	}
}

func TestConfigResolve_ToolSuggestion(t *testing.T) {
	tools := config.ReviewTools()
	if len(tools) < 2 {
		t.Fatal("tool table needs two tools for the suggestion check")
	}
	resolveFixture(t, "", "")
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	_, warnings, err := runConfigCLI(t, "resolve", "--gate", "bead")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(warnings, "Suggestion:") {
		t.Fatal(warnings)
	}
	for _, tool := range tools[:2] {
		if err := os.WriteFile(filepath.Join(bin, tool.Command), []byte("#!/bin/sh\nexit 99\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	_, warnings, err = runConfigCLI(t, "resolve", "--gate", "bead")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(warnings, "Suggestion:") != 1 || len(strings.Split(strings.TrimSpace(warnings), "\n")) != 1 || !strings.Contains(warnings, "configure a process runner for ") || !strings.Contains(warnings, "review diversity") {
		t.Fatal(warnings)
	}
	c := config.DefaultConfig()
	for _, tool := range tools[:2] {
		if !strings.Contains(warnings, tool.Name) {
			t.Fatalf("missing tool %s: %s", tool.Name, warnings)
		}
		c.Panel.Runners = append(c.Panel.Runners, config.Runner{Family: tool.Name, Kind: "process", Argv: []string{tool.Command}})
	}
	if got := c.ToolSuggestions(); len(got) != 0 {
		t.Fatalf("configured command suggested: %v", got)
	}
}

func TestRepoConfig_Resolves(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mindspec", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// The existing CLI root finder resolves linked worktrees to the main tree.
	// Test this checkout's repository config in a standalone project.
	root = resolveFixture(t, string(data), "")
	configResolveCmd.Flags().Set("json", "false")
	out, _, err := runConfigCLI(t, "resolve", "--gate", "bead")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "process") {
		t.Fatal(out)
	}
	c, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := c.RunnerFor("codex")
	if !ok || r.OnQuota != "claude" || !reflect.DeepEqual(r.QuotaSignals, []string{"ERROR: You've hit your usage limit"}) {
		t.Fatalf("repo runner: %+v", r)
	}
}

func TestIsolationRoot_Validate(t *testing.T) {
	root := resolveFixture(t, "panel: {isolation_root: tracked}\n", "")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init")
	if err := os.Mkdir(filepath.Join(root, "tracked"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked", "file"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "tracked/file")
	config.ResetCache()
	_, err := config.Load(root)
	if err == nil || !strings.Contains(err.Error(), "is tracked") {
		t.Fatalf("tracked root accepted: %v", err)
	}
	_, _, err = runConfigCLI(t, "show")
	if err == nil {
		t.Fatal("show accepted tracked root")
	}
}

func TestLocalConfig_ShowRefusals(t *testing.T) {
	for name, local := range map[string]string{"loop": "loop: {}", "size": strings.Repeat(" ", 65537), "syntax": "panel: ["} {
		t.Run(name, func(t *testing.T) {
			resolveFixture(t, "", local)
			_, _, err := runConfigCLI(t, "show")
			if err == nil || !strings.Contains(err.Error(), "config.local.yaml") {
				t.Fatalf("show accepted local file: %v", err)
			}
		})
	}
}
