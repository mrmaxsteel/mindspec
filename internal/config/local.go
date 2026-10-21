package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const localLimit = 65536

//go:embed review_tools.yaml
var reviewToolsData []byte

// ReviewTool is a command that an operator can configure as a process runner.
type ReviewTool struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
}

// ReviewTools returns the shipped suggestion table.
func ReviewTools() []ReviewTool {
	var tools []ReviewTool
	if err := yaml.Unmarshal(reviewToolsData, &tools); err != nil {
		panic(err)
	}
	return tools
}

// ToolSuggestions returns advice for installed commands that no process runner uses.
func (c *Config) ToolSuggestions() []string {
	used := map[string]bool{}
	for _, r := range c.runners() {
		if len(r.Argv) > 0 {
			used[r.Argv[0]] = true
		}
	}
	var names []string
	for _, tool := range ReviewTools() {
		if _, err := exec.LookPath(tool.Command); err == nil && !used[tool.Command] {
			names = append(names, tool.Name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("configure a process runner for %s in panel.runners for review diversity", strings.Join(names, ", "))}
}

func loadPanelLayers(root string, data []byte, c *Config) error {
	c.sources = map[string]string{}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if err := recordSources(&doc, "", "config.yaml", c); err != nil {
		return err
	}
	var main struct {
		Panel struct {
			Runners []Runner `yaml:"runners"`
		}
	}
	if err := yaml.Unmarshal(data, &main); err != nil {
		return err
	}
	merged, err := mergeRunners(DefaultConfig().Panel.Runners, main.Panel.Runners)
	if err != nil {
		return err
	}
	c.Panel.Runners = merged
	localPath := filepath.Join(root, ".mindspec", "config.local.yaml")
	f, err := os.Open(localPath)
	if os.IsNotExist(err) {
		if _, statErr := os.Lstat(localPath); os.IsNotExist(statErr) {
			return nil
		}
	}
	if err != nil {
		return localError(err)
	}
	defer f.Close()
	local, err := io.ReadAll(io.LimitReader(f, localLimit+1))
	if err != nil {
		return localError(err)
	}
	if len(local) > localLimit {
		return localError(fmt.Errorf("file exceeds %d bytes", localLimit))
	}
	var overlay struct {
		Panel struct {
			Runners []Runner `yaml:"runners"`
		} `yaml:"panel"`
		Models map[string]string `yaml:"models"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(local))
	decoder.KnownFields(true)
	if err := decoder.Decode(&overlay); err != nil && err != io.EOF {
		return localError(err)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return localError(fmt.Errorf("expected one YAML document"))
	}
	doc = yaml.Node{}
	if err := yaml.Unmarshal(local, &doc); err != nil {
		return localError(err)
	}
	if err := recordSources(&doc, "", "config.local.yaml", c); err != nil {
		return localError(err)
	}
	c.Panel.Runners, err = mergeRunners(c.Panel.Runners, overlay.Panel.Runners)
	if err != nil {
		return localError(err)
	}
	if c.Models == nil {
		c.Models = map[string]string{}
	}
	for key, value := range overlay.Models {
		c.Models[key] = value
	}
	return nil
}

func localError(err error) error {
	return fmt.Errorf("config.local.yaml: %s\nrecovery: set only panel.runners and models in .mindspec/config.local.yaml", err)
}

func recordSources(n *yaml.Node, path, source string, c *Config) error {
	return recordSourcesActive(n, path, source, c, map[*yaml.Node]bool{})
}

func recordSourcesActive(n *yaml.Node, path, source string, c *Config, active map[*yaml.Node]bool) error {
	if active[n] {
		return fmt.Errorf("cyclic config alias\nrecovery: remove the alias cycle from the config")
	}
	active[n] = true
	defer delete(active, n)
	if n.Kind == yaml.AliasNode {
		return recordSourcesActive(n.Alias, path, source, c, active)
	}
	if n.Kind == yaml.DocumentNode {
		for _, child := range n.Content {
			if err := recordSourcesActive(child, path, source, c, active); err != nil {
				return err
			}
		}
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if key == "<<" {
				if err := recordSourcesActive(n.Content[i+1], path, source, c, active); err != nil {
					return err
				}
				continue
			}
			next := key
			if path != "" {
				next = path + "." + key
			}
			if next == "panel.substitution" {
				return fmt.Errorf("panel.substitution is retired\nrecovery: set on_quota on the runner in panel.runners")
			}
			if key == "model" && strings.Contains(path, "reviewers[") {
				var model string
				if err := n.Content[i+1].Decode(&model); err != nil {
					return err
				}
				if err := ValidateModel(model); err != nil {
					return fmt.Errorf("%s: %w", next, err)
				}
			}
			if next == "panel.isolation_root" {
				var root string
				if err := n.Content[i+1].Decode(&root); err != nil {
					return err
				}
				if root == "" {
					c.Panel.IsolationRoot = ""
				}
			}
			c.sources[next] = source
			if err := recordSourcesActive(n.Content[i+1], next, source, c, active); err != nil {
				return err
			}
		}
	}
	if n.Kind == yaml.SequenceNode {
		for i, child := range n.Content {
			next := fmt.Sprintf("%s[%d]", path, i)
			if path == "panel.runners" {
				var r Runner
				if err := child.Decode(&r); err != nil {
					return err
				}
				c.sources[path+"."+r.Family] = source
			}
			if err := recordSourcesActive(child, next, source, c, active); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Config) source(path string) string {
	if s := c.sources[path]; s != "" {
		return s
	}
	return "default"
}

func (c *Config) reviewerLabel(gate string) string {
	if len(c.Panel.Gates[gate].Reviewers) > 0 {
		return "panel.gates." + gate + ".reviewers"
	}
	if gate == "adhoc" && len(c.Panel.Gates["bead"].Reviewers) > 0 {
		return "panel.gates.bead.reviewers"
	}
	if len(c.Panel.Reviewers) > 0 {
		return "panel.reviewers"
	}
	return "default reviewers"
}

// ModelOnlyWarnings names each configured entry that implicitly uses the harness.
func (c *Config) ModelOnlyWarnings() []string {
	var lines []string
	add := func(label string, rs []Reviewer) {
		for i, r := range rs {
			if r.Family == "" {
				lines = append(lines, fmt.Sprintf("%s[%d] runs on the harness runner; add a family and a runner to run it elsewhere", label, i))
			}
		}
	}
	add("panel.reviewers", c.Panel.Reviewers)
	for _, g := range PanelGateKeys {
		add("panel.gates."+g+".reviewers", c.Panel.Gates[g].Reviewers)
	}
	return lines
}

// ResolvedSlot is a preview slot with the source of each displayed value.
type ResolvedSlot struct {
	Slot    string            `json:"slot"`
	Family  string            `json:"family"`
	Model   string            `json:"model"`
	Lens    string            `json:"lens"`
	Kind    string            `json:"kind"`
	Sources map[string]string `json:"sources"`
}

// ResolveSlots previews a new panel. It does not read an existing roster.
func (c *Config) ResolveSlots(gate string) ([]ResolvedSlot, error) {
	slots, err := c.PanelGateReviewerSlots(gate)
	if err != nil {
		return nil, err
	}
	rs := c.resolveGateReviewers(gate)
	entry := 0
	used := 0
	result := make([]ResolvedSlot, 0, len(slots))
	for _, s := range slots {
		r := rs[entry]
		label := fmt.Sprintf("%s[%d]", c.reviewerLabel(gate), entry)
		runner, _ := c.RunnerFor(s.Family)
		familySource := c.source(label + ".family")
		if c.reviewerLabel(gate) == "default reviewers" {
			familySource = c.RunnerSource(s.Family)
		}
		modelSource := c.source(label + ".model")
		if r.Model == "" {
			modelSource = c.source("models.reviewer." + s.Family)
			if _, ok := c.Models["reviewer."+s.Family]; !ok {
				modelSource = familySource
			}
		}
		lensSource := c.source(label + ".lens")
		if r.Lens == "" {
			lensSource = c.source("panel.gates." + gate + ".lenses")
		}
		result = append(result, ResolvedSlot{Slot: s.Slot, Family: s.Family, Model: s.Model, Lens: s.Lens, Kind: runner.Kind, Sources: map[string]string{"slot": "default", "family": familySource, "model": modelSource, "lens": lensSource, "kind": c.RunnerSource(s.Family)}})
		used++
		if used == r.CountValue() {
			entry++
			used = 0
		}
	}
	return result, nil
}
