package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"

	"gopkg.in/yaml.v3"
)

// Runner describes one reviewer execution family. TimeoutMinutes is nil for an unlimited harness.
type Runner struct {
	Family         string   `yaml:"family" json:"family"`
	Kind           string   `yaml:"kind" json:"kind"`
	Argv           []string `yaml:"argv,omitempty" json:"argv,omitempty"`
	Prompt         string   `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	Output         string   `yaml:"output,omitempty" json:"output,omitempty"`
	QuotaSignals   []string `yaml:"quota_signals,omitempty" json:"quota_signals,omitempty"`
	OnQuota        string   `yaml:"on_quota,omitempty" json:"on_quota,omitempty"`
	Probe          []string `yaml:"probe,omitempty" json:"probe,omitempty"`
	TimeoutMinutes *int     `yaml:"timeout_minutes,omitempty" json:"timeout_minutes,omitempty"`
	present        map[string]bool
}

func (r *Runner) UnmarshalYAML(n *yaml.Node) error {
	type plain Runner
	var value plain
	if err := n.Decode(&value); err != nil {
		return err
	}
	value.present = map[string]bool{}
	allowed := map[string]bool{"family": true, "kind": true, "argv": true, "prompt": true, "output": true, "quota_signals": true, "on_quota": true, "probe": true, "timeout_minutes": true}
	if n.Kind != yaml.MappingNode {
		return termsafe.NewFailure("runner must be a mapping", "set family and kind on each panel.runners entry")
	}
	for i := 0; i < len(n.Content); i += 2 {
		key := n.Content[i].Value
		if !allowed[key] {
			return termsafe.NewFailure(fmt.Sprintf("unknown runner key %q", key), "remove the unknown key from panel.runners")
		}
		value.present[key] = true
	}
	*r = Runner(value)
	return nil
}

func mergeRunners(base, overlay []Runner) ([]Runner, error) {
	out := append([]Runner(nil), base...)
	seen := map[string]bool{}
	for _, r := range overlay {
		if r.Family == "" || seen[r.Family] {
			return nil, termsafe.NewFailure(fmt.Sprintf("empty or duplicate runner family %q", r.Family), "give each panel.runners entry a unique family")
		}
		seen[r.Family] = true
		found := false
		for i := range out {
			if out[i].Family == r.Family {
				out[i] = r
				found = true
				break
			}
		}
		if !found {
			out = append(out, r)
		}
	}
	return out, nil
}

func (c *Config) runners() []Runner {
	if len(c.Panel.Runners) == 0 {
		return []Runner{{Family: "harness", Kind: "harness"}}
	}
	return c.Panel.Runners
}

// RunnerFor returns the configured runner for family.
func (c *Config) RunnerFor(family string) (Runner, bool) {
	for _, r := range c.runners() {
		if r.Family == family {
			return r, true
		}
	}
	return Runner{}, false
}

// RunnerSource identifies the configuration layer that supplied a runner.
func (c *Config) RunnerSource(family string) string { return c.source("panel.runners." + family) }

// SubstituteModel resolves a target family independently of the original reviewer entry.
func (c *Config) SubstituteModel(toFamily string) string {
	if model, ok := c.Models["reviewer."+toFamily]; ok {
		return model
	}
	return toFamily
}

// ValidateModel checks whether a model can be passed as one command argument.
func ValidateModel(model string) error {
	if model == "" || strings.HasPrefix(model, "-") || strings.ContainsFunc(model, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return termsafe.NewFailure(fmt.Sprintf("invalid model %q: use a non-empty value without whitespace, control bytes or a leading dash", model), "set a model-safe value in reviewers or models")
	}
	return nil
}

func validateArgv(argv []string) error {
	if len(argv) == 0 || argv[0] == "" {
		return fmt.Errorf("argv needs a command")
	}
	allowed := map[string]bool{"{prompt}": true, "{prompt_file}": true, "{model}": true, "{slot_dir}": true, "{tree}": true}
	for i, arg := range argv {
		if strings.ContainsAny(arg, "{}") && (i == 0 || !allowed[arg]) {
			return fmt.Errorf("invalid argv element %q", arg)
		}
	}
	return nil
}

func validatePanelConfig(c *Config) error {
	for i := range c.Panel.Runners {
		r := &c.Panel.Runners[i]
		fail := func(msg string) error {
			return termsafe.NewFailure(fmt.Sprintf("runner %q: %s", r.Family, msg), "fix the entry in panel.runners")
		}
		switch r.Kind {
		case "harness":
			for _, key := range []string{"argv", "prompt", "output", "quota_signals", "on_quota", "probe"} {
				if r.present[key] {
					return fail("a harness runner cannot set " + key)
				}
			}
			if len(r.Argv) > 0 || r.Prompt != "" || r.Output != "" || len(r.QuotaSignals) > 0 || r.OnQuota != "" || len(r.Probe) > 0 {
				return fail("a harness runner can set only timeout_minutes")
			}
		case "process":
			if err := validateArgv(r.Argv); err != nil {
				return fail(err.Error())
			}
			if r.Prompt != "stdin" && r.Prompt != "file" && r.Prompt != "arg" {
				return fail("prompt must be stdin, file or arg")
			}
			if r.Output != "stdout" && r.Output != "file" {
				return fail("output must be stdout or file")
			}
			needed := ""
			if r.Prompt == "file" {
				needed = "{prompt_file}"
			}
			if r.Prompt == "arg" {
				needed = "{prompt}"
			}
			if needed != "" {
				found := false
				for _, arg := range r.Argv {
					if arg == needed {
						found = true
					}
				}
				if !found {
					return fail("prompt needs " + needed + " in argv")
				}
			}
			if r.TimeoutMinutes == nil {
				r.TimeoutMinutes = intp(60)
			}
			if len(r.Probe) > 0 {
				if err := validateArgv(r.Probe); err != nil {
					return fail("probe: " + err.Error())
				}
			}
		default:
			return fail("kind must be harness or process")
		}
		if r.TimeoutMinutes != nil && *r.TimeoutMinutes <= 0 {
			return fail("timeout_minutes must be positive")
		}
		if r.OnQuota != "" || r.present["on_quota"] {
			if _, ok := c.RunnerFor(r.OnQuota); !ok || r.OnQuota == r.Family {
				return fail("on_quota must name another declared family")
			}
		}
		if err := ValidateModel(c.SubstituteModel(r.Family)); err != nil {
			return err
		}
	}
	for key, model := range c.Models {
		if err := ValidateModel(model); err != nil {
			return fmt.Errorf("models[%q]: %w", key, err)
		}
	}
	for _, gate := range PanelGateKeys {
		for _, lens := range c.GateLenses(gate) {
			if _, ok := c.Panel.Lenses[lens]; !ok {
				return termsafe.NewFailure(fmt.Sprintf("panel.gates.%s.lenses names unknown lens %q", gate, lens), "define the lens in panel.lenses")
			}
		}
	}
	return validateIsolationRoot(c.Panel.IsolationRoot)
}

// validateIsolationRoot requires a clean relative path with / separators.
// It permits one trailing slash. It refuses backslashes and control bytes.
func validateIsolationRoot(root string) error {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(root)), "./")
	invalid := root == "" || filepath.IsAbs(root) || strings.HasPrefix(root, "/") || strings.Contains(root, "..") || strings.ContainsAny(root, ":\\") || strings.TrimSuffix(root, "/") != clean
	for i := 0; i < len(root); i++ {
		if root[i] < 0x20 || root[i] == 0x7f {
			invalid = true
		}
	}
	for _, reserved := range []string{".", ".git", ".worktrees", ".mindspec"} {
		if clean == reserved || strings.HasPrefix(clean, reserved+"/") {
			invalid = true
		}
	}
	if invalid {
		return termsafe.NewFailure(fmt.Sprintf("invalid panel.isolation_root %q", root), "set panel.isolation_root to an untracked relative directory outside .git, .worktrees and .mindspec")
	}
	return nil
}

// SetIsolationTrackedChecker supplies repository I/O from the command layer at startup.
func SetIsolationTrackedChecker(check func(root, path string) (bool, error)) {
	isolationTracked = check
}

var isolationTracked func(root, path string) (bool, error)

// IsolationRootFor returns the configured relative scratch root for a main working tree.
func IsolationRootFor(mainRoot string) (string, error) {
	c, err := Load(mainRoot)
	if err != nil {
		return "", err
	}
	return c.Panel.IsolationRoot, nil
}
