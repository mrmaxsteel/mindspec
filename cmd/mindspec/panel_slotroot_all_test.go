package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// A main-only invalid key makes root resolution observable before any write.
// A verb that uses the clone instead cannot see this sentinel.
func TestSlotRoot_EveryPanelVerb(t *testing.T) {
	f := newCheckoutFixture(t, ".mindspec-scratch")
	checkoutOK(t, "--slot", "R1")
	sub := filepath.Join(f.layout(t, "R1").Tree, "sub")
	slotWrite(t, filepath.Join(sub, "keep"), "subdirectory")
	withTestChdir(t, sub)
	const sentinel = "root_probe_main_only"
	slotWrite(t, filepath.Join(f.root, ".mindspec/config.local.yaml"), sentinel+": true\n")
	config.ResetCache()
	drivers := map[string][]string{
		"panel create": {"p", "--gate", "adhoc", "--target", "main"},
		"panel verify": {"p"}, "panel tally": {"p"}, "panel checkout": {"p", "--slot", "R1"},
		"panel prompt": {"p", "--slot", "R1"}, "panel launch": {"p", "R1"}, "panel recover": {"p", "R1", "--reason", "fixture"},
		"panel submit":  {"p", "--slot", "R1", "--round", "1", "--file", "-"},
		"panel abandon": {"p", "--reason", "fixture", "--by", "operator"}, "panel defer": {"p"}, "panel refute": {"p"}, "panel rebind": {"p", "--reason", "fixture"},
		"panel disposition append": {"--spec", "900-test", "--panel", "p", "--data", "{}"},
		"panel disposition check":  {"--spec", "900-test", "--panel", "p"},
		"panel disposition query":  {"--metric", "Q1"},
	}
	free := map[string]bool{"panel": true, "panel disposition": true, "panel schema": true, "panel disposition validate": true}
	visited := map[string]bool{}
	var walk func(*cobra.Command, string)
	walk = func(cmd *cobra.Command, path string) {
		visited[path] = true
		if !free[path] {
			args, ok := drivers[path]
			if !ok {
				t.Errorf("new panel verb has no root driver: %s", path)
			} else {
				t.Run(path, func(t *testing.T) {
					cmd.Flags().VisitAll(func(flag *pflag.Flag) { _ = flag.Value.Set(flag.DefValue); flag.Changed = false })
					defer cmd.Flags().VisitAll(func(flag *pflag.Flag) { _ = flag.Value.Set(flag.DefValue); flag.Changed = false })
					if e := cmd.ParseFlags(args); e != nil {
						t.Fatal(e)
					}
					var out bytes.Buffer
					cmd.SetOut(&out)
					cmd.SetIn(strings.NewReader("{}"))
					defer cmd.SetOut(nil)
					defer cmd.SetIn(nil)
					err := cmd.RunE(cmd, cmd.Flags().Args())
					if err == nil || !strings.Contains(err.Error(), sentinel) {
						t.Fatalf("verb did not resolve main config: %v %s", err, out.String())
					}
				})
			}
		}
		for _, child := range cmd.Commands() {
			walk(child, path+" "+child.Name())
		}
	}
	walk(panelCmd, "panel")
	for path := range drivers {
		if !visited[path] {
			t.Errorf("driver names an absent verb: %s", path)
		}
	}
}
