package config

import (
	"fmt"
	"os"
	"sort"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"gopkg.in/yaml.v3"
)

// RoundLimit returns the configured round allowance, or the default of five.
// Load resolves the deprecated alias before returning the config.
func (c *Config) RoundLimit() int {
	if c.Panel.MaxRounds > 0 {
		return c.Panel.MaxRounds
	}
	return 5
}

// RemovedLoopKeys returns sorted names for display. These have no policy effect,
// except for the deprecated round-limit alias resolved by Load.
func (c *Config) RemovedLoopKeys() []string {
	keys := make([]string, 0, len(c.removedLoop))
	for k := range c.removedLoop {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// RemovedLoopValue returns the original value for display only.
func (c *Config) RemovedLoopValue(key string) any { return c.removedLoop[key] }

func (c *Config) readRemovedLoop(raw map[string]any) error {
	c.removedLoop = nil
	var collect func(string, map[string]any)
	collect = func(prefix string, entries map[string]any) {
		for k, v := range entries {
			key := prefix + k
			if nested, ok := v.(map[string]any); ok && (key == "halt" || key == "budget" || key == "context") {
				collect(key+".", nested)
			} else {
				c.removedLoop[key] = v
			}
		}
	}
	if entries, ok := raw["loop"].(map[string]any); ok && len(entries) > 0 {
		c.removedLoop = map[string]any{}
		collect("", entries)
	}
	panel, _ := raw["panel"].(map[string]any)
	_, explicit := panel["max_rounds"]
	for _, key := range c.RemovedLoopKeys() {
		if key != "halt.max_rounds_per_bead" {
			fmt.Fprintf(os.Stderr, "warning: loop.%s is removed and ignored\n", termsafe.Escape(key))
			continue
		}
		value := c.removedLoop[key]
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		var n int
		if !explicit {
			if err := yaml.Unmarshal(data, &n); err != nil || n < 1 {
				return termsafe.NewFailure("the deprecated round limit must be a positive integer", "set panel.max_rounds to at least 1")
			}
			c.Panel.MaxRounds = n
		}
		fmt.Fprintf(os.Stderr, "warning: loop.%s value %s is a deprecated alias for panel.max_rounds; effective value %d\n", termsafe.Escape(key), termsafe.Escape(fmt.Sprint(value)), c.RoundLimit())
	}
	return nil
}
