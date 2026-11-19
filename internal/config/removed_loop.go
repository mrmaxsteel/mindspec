package config

import (
	"fmt"
	"os"
	"sort"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
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
	if value, present := raw["loop"]; present && value != nil {
		if entries, ok := value.(map[string]any); ok {
			c.removedLoop = map[string]any{}
			collect("", entries)
		} else {
			fmt.Fprintln(os.Stderr, "warning: loop is removed and ignored")
		}
	}
	panel, _ := raw["panel"].(map[string]any)
	explicit := panel["max_rounds"] != nil
	for _, key := range c.RemovedLoopKeys() {
		if key != "halt.max_rounds_per_bead" {
			fmt.Fprintf(os.Stderr, "warning: loop.%s is removed and ignored\n", termsafe.Escape(key))
			continue
		}
		value := c.removedLoop[key]
		n, valid := value.(int)
		if !valid || n < 1 {
			fallback := "the default applies"
			if explicit {
				fallback = "panel.max_rounds applies"
			}
			fmt.Fprintf(os.Stderr, "warning: loop.%s value %s is ignored; %s; effective value %d\n", termsafe.Escape(key), termsafe.Escape(fmt.Sprint(value)), fallback, c.RoundLimit())
			continue
		}
		if !explicit {
			c.Panel.MaxRounds = n
		}
		fmt.Fprintf(os.Stderr, "warning: loop.%s value %s is a deprecated alias for panel.max_rounds; effective value %d\n", termsafe.Escape(key), termsafe.Escape(fmt.Sprint(value)), c.RoundLimit())
	}
	return nil
}
