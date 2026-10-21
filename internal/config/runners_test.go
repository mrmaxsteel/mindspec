package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func panelConfigFixture(t *testing.T, main, local string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"config.yaml": main, "config.local.yaml": local} {
		if data != "" {
			if err := os.WriteFile(filepath.Join(root, ".mindspec", name), []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(ResetCache)
	return root
}

func mustPanelConfig(t *testing.T, main, local string) *Config {
	t.Helper()
	c, err := Load(panelConfigFixture(t, main, local))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRunners_Validate(t *testing.T) {
	for name, runner := range map[string]string{
		"duplicate":           "[{family: a, kind: harness}, {family: a, kind: harness}]",
		"empty family":        "[{kind: harness}]",
		"harness argv":        "[{family: a, kind: harness, argv: [run]}]",
		"harness empty argv":  "[{family: a, kind: harness, argv: []}]",
		"harness quota":       "[{family: a, kind: harness, on_quota: harness}]",
		"harness prompt":      "[{family: a, kind: harness, prompt: stdin}]",
		"harness output":      "[{family: a, kind: harness, output: stdout}]",
		"harness probe":       "[{family: a, kind: harness, probe: [run]}]",
		"harness signals":     "[{family: a, kind: harness, quota_signals: [wall]}]",
		"missing output":      "[{family: a, kind: process, argv: [run], prompt: stdin}]",
		"missing argv":        "[{family: a, kind: process, prompt: stdin, output: stdout}]",
		"missing prompt":      "[{family: a, kind: process, argv: [run], output: stdout}]",
		"file placeholder":    "[{family: a, kind: process, argv: [run], prompt: file, output: stdout}]",
		"arg placeholder":     "[{family: a, kind: process, argv: [run], prompt: arg, output: stdout}]",
		"unknown placeholder": "[{family: a, kind: process, argv: [run, '{shell}'], prompt: stdin, output: stdout}]",
		"partial placeholder": "[{family: a, kind: process, argv: [run, 'x{tree}'], prompt: stdin, output: stdout}]",
		"command placeholder": "[{family: a, kind: process, argv: ['{tree}/run'], prompt: stdin, output: stdout}]",
		"self quota":          "[{family: a, kind: process, argv: [run], prompt: stdin, output: stdout, on_quota: a}]",
		"unknown quota":       "[{family: a, kind: process, argv: [run], prompt: stdin, output: stdout, on_quota: b}]",
		"empty quota":         "[{family: a, kind: process, argv: [run], prompt: stdin, output: stdout, on_quota: ''}]",
		"unknown kind":        "[{family: a, kind: other}]",
		"negative timeout":    "[{family: a, kind: harness, timeout_minutes: -1}]",
		"unknown key":         "[{family: a, kind: harness, shell: true}]",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(panelConfigFixture(t, "panel:\n  runners: "+runner+"\n", ""))
			if err == nil || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("invalid runner accepted or no recovery: %v", err)
			}
		})
	}
	for _, prompt := range []string{"stdin", "file", "arg"} {
		placeholder := "{model}"
		if prompt == "file" {
			placeholder = "{prompt_file}"
		}
		if prompt == "arg" {
			placeholder = "{prompt}"
		}
		mustPanelConfig(t, fmt.Sprintf("panel:\n  runners: [{family: a, kind: process, argv: [run, '%s', '{slot_dir}', '{tree}'], prompt: %s, output: file}]\n", placeholder, prompt), "")
	}
}

func TestRunnerAdapter_RefusalListsValidValues(t *testing.T) {
	_, err := Load(panelConfigFixture(t, "runner: unknown-adapter\n", ""))
	want := "recovery: set runner to one of claude-code-skills, claude-code-workflow, external in .mindspec/config.yaml"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("adapter recovery does not list valid values: %v", err)
	}
}

func TestRunners_TimeoutDefault(t *testing.T) {
	c := mustPanelConfig(t, "panel:\n  runners: [{family: a, kind: process, argv: [run], prompt: stdin, output: stdout}, {family: b, kind: harness, timeout_minutes: 2}]\n", "")
	a, _ := c.RunnerFor("a")
	h, _ := c.RunnerFor("harness")
	b, _ := c.RunnerFor("b")
	if a.TimeoutMinutes == nil || *a.TimeoutMinutes != 60 || h.TimeoutMinutes != nil || b.TimeoutMinutes == nil || *b.TimeoutMinutes != 2 {
		t.Fatalf("bad timeout defaults: %+v %+v %+v", a, h, b)
	}
}

func TestRunners_ZeroConfigHarness(t *testing.T) {
	c := mustPanelConfig(t, "", "")
	slots, err := c.PanelGateReviewerSlots("bead")
	if err != nil || len(slots) != 4 {
		t.Fatalf("slots: %+v %v", slots, err)
	}
	if len(c.Panel.Runners) != 1 || c.Panel.Runners[0].Family != "harness" || c.Panel.Runners[0].Kind != "harness" {
		t.Fatal(c.Panel.Runners)
	}
	for _, s := range slots {
		if s.Family != "harness" || s.Model != "harness" {
			t.Fatalf("bad default: %+v", s)
		}
	}
}

func TestRunners_MergeByFamily(t *testing.T) {
	t.Run("local replacement keeps position", func(t *testing.T) {
		c := mustPanelConfig(t, "panel:\n  runners: [{family: a, kind: harness}, {family: b, kind: harness}]\n", "panel:\n  runners: [{family: a, kind: harness, timeout_minutes: 7}]\n")
		var got []string
		for _, r := range c.Panel.Runners {
			got = append(got, r.Family)
		}
		if !reflect.DeepEqual(got, []string{"harness", "a", "b"}) {
			t.Fatalf("runner order = %v; want [harness a b]", got)
		}
		if timeout := c.Panel.Runners[1].TimeoutMinutes; timeout == nil || *timeout != 7 {
			t.Fatal("local replacement was not applied")
		}
	})
	t.Run("config replacement keeps default position", func(t *testing.T) {
		c := mustPanelConfig(t, "panel:\n  runners: [{family: a, kind: harness}, {family: harness, kind: harness, timeout_minutes: 7}]\n", "")
		if len(c.Panel.Runners) != 2 || c.Panel.Runners[0].Family != "harness" || c.Panel.Runners[1].Family != "a" || c.Panel.Runners[0].TimeoutMinutes == nil || *c.Panel.Runners[0].TimeoutMinutes != 7 {
			t.Fatalf("replacement moved the default runner: %+v", c.Panel.Runners)
		}
	})
	c := mustPanelConfig(t, "panel:\n  runners: [{family: a, kind: harness}, {family: b, kind: harness}]\n", "")
	slots, err := c.PanelGateReviewerSlots("bead")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range slots {
		got = append(got, s.Family)
	}
	if !reflect.DeepEqual(got, []string{"harness", "a", "b", "harness"}) {
		t.Fatal(got)
	}
	c = mustPanelConfig(t, "panel:\n  runners: [{family: harness, kind: process, argv: [run], prompt: stdin, output: stdout}, {family: a, kind: harness}]\n", "")
	if len(c.Panel.Runners) != 2 || c.Panel.Runners[0].Family != "harness" || c.Panel.Runners[0].Kind != "process" {
		t.Fatal(c.Panel.Runners)
	}
}

func TestRunners_ExplicitCountUsesMergedOrder(t *testing.T) {
	c := mustPanelConfig(t, "panel:\n  runners: [{family: a, kind: harness}, {family: b, kind: harness}, {family: c, kind: harness}, {family: d, kind: harness}, {family: e, kind: harness}]\n", "")
	for _, tc := range []struct {
		name  string
		count int
		want  []string
	}{
		{"all runners", 6, []string{"harness", "a", "b", "c", "d", "e"}},
		{"partial second cycle", 8, []string{"harness", "a", "b", "c", "d", "e", "harness", "a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slots, err := c.PanelGateReviewerSlots("bead", tc.count)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, slot := range slots {
				got = append(got, slot.Family)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("override families = %v; want %v", got, tc.want)
			}
		})
	}
	t.Run("explicit list keeps its order", func(t *testing.T) {
		c.Panel.Reviewers = []Reviewer{{Family: "e", Count: intp(3)}, {Family: "b"}}
		slots, err := c.PanelGateReviewerSlots("bead", 3)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, slot := range slots {
			got = append(got, slot.Family)
		}
		if !reflect.DeepEqual(got, []string{"e", "b", "e"}) {
			t.Fatal(got)
		}
	})
}

func TestSubstitution_Retired(t *testing.T) {
	for _, value := range []string{"{}", "null", "{claude_sub_on_quota: false}", "{substitutes: {a: b}}"} {
		_, err := Load(panelConfigFixture(t, "panel:\n  substitution: "+value+"\n", ""))
		if err == nil || !strings.Contains(err.Error(), "set on_quota on the runner in panel.runners") {
			t.Fatalf("retired key: %v", err)
		}
	}
}

func TestModelValue_Refusals(t *testing.T) {
	for _, model := range []string{"--x", "m x", "", "m\tvalue", "m\x1b", "m\u0085value"} {
		for _, body := range []string{fmt.Sprintf("models: {reviewer.b: %q}\n", model), fmt.Sprintf("panel:\n  reviewers: [{model: %q, count: 2}]\n", model)} {
			_, err := Load(panelConfigFixture(t, body, ""))
			if err == nil {
				t.Fatalf("model %q accepted from %s", model, body)
			}
		}
	}
	c := DefaultConfig()
	c.Panel.Reviewers = []Reviewer{{Model: "-x", Count: intp(2)}}
	if _, err := c.PanelGateReviewerSlots("bead"); err == nil {
		t.Fatal("slot expansion accepted unsafe model")
	}
}

func TestSlotModelResolution(t *testing.T) {
	c := mustPanelConfig(t, "panel:\n  runners: [{family: b, kind: harness}]\n  reviewers: [{family: b}, {family: b, model: entry}, {family: harness}]\nmodels: {reviewer.b: m1}\n", "")
	slots, err := c.PanelGateReviewerSlots("bead")
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"m1", "entry", "harness"} {
		if slots[i].Model != want {
			t.Fatalf("slot %d: %+v", i, slots[i])
		}
	}
	if slots[0].Family != "b" {
		t.Fatal(slots)
	}
}

func TestReviewer_ModelOnlyIsHarness(t *testing.T) {
	c := mustPanelConfig(t, "panel:\n  reviewers: [{model: m7, count: 2}]\n", "")
	slots, err := c.PanelGateReviewerSlots("bead")
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 2 || slots[0].Family != "harness" || slots[0].Model != "m7" {
		t.Fatal(slots)
	}
}

func TestSubstituteModel(t *testing.T) {
	c := mustPanelConfig(t, "panel:\n  runners: [{family: a, kind: harness}, {family: b, kind: harness}]\n  reviewers: [{family: a, model: m1, count: 2}]\nmodels: {reviewer.b: m2}\n", "")
	if c.SubstituteModel("b") != "m2" || c.SubstituteModel("a") != "a" {
		t.Fatal("substitution used the original entry model")
	}
}

func TestLocalConfig_Merge(t *testing.T) {
	c := mustPanelConfig(t, "panel:\n  runners: [{family: a, kind: harness}, {family: b, kind: process, argv: [run], prompt: stdin, output: stdout}]\nmodels: {reviewer.a: old, reviewer.b: keep}\n", "panel:\n  runners: [{family: b, kind: harness}, {family: c, kind: harness}]\nmodels: {reviewer.a: local}\n")
	var families []string
	for _, r := range c.Panel.Runners {
		families = append(families, r.Family)
	}
	if !reflect.DeepEqual(families, []string{"harness", "a", "b", "c"}) || c.Panel.Runners[2].Kind != "harness" || c.Models["reviewer.a"] != "local" || c.Models["reviewer.b"] != "keep" || c.RunnerSource("b") != "config.local.yaml" {
		t.Fatalf("bad local merge: %+v %v", c.Panel.Runners, c.Models)
	}
	c = mustPanelConfig(t, "", "panel:\n  runners: [{family: local, kind: harness}]\n")
	if _, ok := c.RunnerFor("local"); !ok {
		t.Fatal("local file ignored without main config")
	}
	c = mustPanelConfig(t, "model: &model m7\npanel:\n  reviewers: [{model: *model, count: 2}]\n", "")
	slots, err := c.ResolveSlots("bead")
	if err != nil || slots[0].Model != "m7" || slots[0].Sources["model"] != "config.yaml" {
		t.Fatalf("alias source: %+v %v", slots, err)
	}
	if _, err := Load(panelConfigFixture(t, "extra: &cycle [*cycle]\n", "")); err == nil {
		t.Fatal("cyclic alias accepted")
	}
}

func TestLocalConfig_Refusals(t *testing.T) {
	t.Run("dangling symlink", func(t *testing.T) {
		root := panelConfigFixture(t, "", "")
		if err := os.Symlink(filepath.Join(root, "absent.yaml"), filepath.Join(root, ".mindspec", "config.local.yaml")); err != nil {
			t.Fatal(err)
		}
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "config.local.yaml") || !strings.Contains(err.Error(), "recovery: set only panel.runners and models in .mindspec/config.local.yaml") {
			t.Fatalf("dangling local file accepted or no recovery: %v", err)
		}
	})
	for name, local := range map[string]string{"loop": "loop: {}", "other panel": "panel: {isolation_root: scratch}", "large": strings.Repeat(" ", 65537), "parse": "panel: [", "unknown runner": "panel: {runners: [{family: a, kind: harness, surprise: yes}]}", "multiple": "models: {}\n---\nmodels: {}"} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(panelConfigFixture(t, "", local))
			if err == nil || !strings.Contains(err.Error(), "config.local.yaml") {
				t.Fatalf("local refusal: %v", err)
			}
		})
	}
	root := panelConfigFixture(t, "", "")
	if err := os.Mkdir(filepath.Join(root, ".mindspec", "config.local.yaml"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("unreadable local file accepted")
	}
	mustPanelConfig(t, "", strings.Repeat(" ", 65536))
}

func TestLenses_GateDefaults(t *testing.T) {
	c := mustPanelConfig(t, "", "")
	for _, gate := range PanelGateKeys {
		slots, err := c.PanelGateReviewerSlots(gate)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"author-of-record", "empirical-prober", "codebase-pin", "adversarial", "contract-stability", "integration"}
		if gate == "spec_approve" || gate == "plan_approve" {
			want = []string{"falsifiability", "scope-honesty", "contradiction", "feasibility"}
		}
		for i, s := range slots {
			if s.Lens != want[i%len(want)] {
				t.Fatalf("%s slot %d lens %q", gate, i, s.Lens)
			}
		}
		if c.GateAcceptance(gate).Question == "" || len(c.GateAcceptance(gate).NotBlocking) == 0 {
			t.Fatalf("missing acceptance data: %s", gate)
		}
	}
	for _, focus := range c.Panel.Lenses {
		for _, word := range []string{"claude", "codex", "copilot", "gemini", "golang", "python", "javascript", "react", "cobra", "yaml"} {
			if strings.Contains(strings.ToLower(focus), word) {
				t.Fatalf("focus names %s: %s", word, focus)
			}
		}
	}
}

func TestLenses_LensOnlyGateEntry(t *testing.T) {
	c := mustPanelConfig(t, "panel:\n  lenses: {custom: Check the boundary.}\n  gates:\n    bead:\n      lenses: [custom, adversarial]\n", "")
	slots, err := c.PanelGateReviewerSlots("bead")
	if err != nil {
		t.Fatal(err)
	}
	if slots[0].Lens != "custom" || slots[1].Lens != "adversarial" || slots[2].Lens != "custom" {
		t.Fatal(slots)
	}
	c = mustPanelConfig(t, "panel:\n  gates:\n    bead:\n      acceptance: {question: 'Is it correct?', not_blocking: [style]}\n", "")
	if c.GateAcceptance("bead").Question != "Is it correct?" {
		t.Fatal(c.GateAcceptance("bead"))
	}
}

func TestLenses_UnknownRefused(t *testing.T) {
	_, err := Load(panelConfigFixture(t, "panel:\n  gates:\n    bead:\n      lenses: [nope]\n", ""))
	if err == nil || !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "panel.lenses") {
		t.Fatal(err)
	}
}

func TestIsolationRoot_Validate(t *testing.T) {
	for _, root := range []string{"", "/absolute", ".", ".git/x", ".worktrees/x", ".mindspec/x", "a/../b", "./.git/x", "C:\\scratch"} {
		_, err := Load(panelConfigFixture(t, fmt.Sprintf("panel: {isolation_root: %q}\n", root), ""))
		if err == nil || !strings.Contains(err.Error(), "recovery:") {
			t.Fatalf("root %q accepted: %v", root, err)
		}
	}
	for name, body := range map[string]string{
		"direct null":  "panel: {isolation_root: null}\n",
		"aliased null": "empty: &empty null\npanel: {isolation_root: *empty}\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(panelConfigFixture(t, body, ""))
			if err == nil || !strings.Contains(err.Error(), "panel.isolation_root") || !strings.HasSuffix(err.Error(), "\nrecovery: set panel.isolation_root to an untracked relative directory outside .git, .worktrees and .mindspec") {
				t.Fatalf("null isolation root accepted or no recovery: %v", err)
			}
		})
	}
	for _, root := range []string{"review-scratch", "nested/reviews"} {
		c := mustPanelConfig(t, fmt.Sprintf("panel: {isolation_root: %q}\n", root), "")
		if c.Panel.IsolationRoot != root {
			t.Fatal(c.Panel.IsolationRoot)
		}
	}
	root := panelConfigFixture(t, "", "")
	value, err := IsolationRootFor(root)
	if err != nil || value != ".mindspec-scratch" {
		t.Fatalf("root helper: %s %v", value, err)
	}
}
