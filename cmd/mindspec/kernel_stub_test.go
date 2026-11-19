package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134VerbStub_PreKernelNoWrites(t *testing.T) {
	f := kerneltest.PreKernelSpec(t)
	bin := buildMindspecBinary(t)
	for _, args := range [][]string{{"next-action", "y"}, {"lease", f.Items[0] + ".impl.a1"}, {"submit", f.Items[0] + ".impl.a1", "--lease", "token", "--commit", "HEAD"}} {
		before := kerneltest.Snapshot(t, f)
		out, err := f.Command(bin, args...)
		if err != nil {
			out = append(out, []byte(err.Error())...)
		}
		if err == nil {
			t.Fatalf("%v accepted pre-kernel spec: %s", args, out)
		}
		for _, verb := range []string{"mindspec next", "mindspec complete", "mindspec impl approve"} {
			if !strings.Contains(string(out), verb) {
				t.Errorf("%v missing %s: %s", args, verb, out)
			}
		}
		if after := kerneltest.Snapshot(t, f); after != before {
			t.Fatalf("%v changed the snapshot", args)
		}
	}
}
func TestS134Setup_PlainSetupAddsStep(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".beads", "config.yaml"), []byte("issue-prefix: project\ntypes.custom: gate,spike2\nforeign: keep\n"), 0644); err != nil {
		t.Fatal(err)
	}
	bin := buildMindspecBinary(t)
	cmd := exec.Command(bin, "setup")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("plain setup: %v %s", err, out)
	}
	if got := strings.Join(bead.CustomTypes(root), ","); got != "gate,spike2,step" {
		t.Fatalf("plain setup types: %s", got)
	}
}
func TestS134VerbStub_HelpGolden(t *testing.T) {
	for _, name := range []string{"next-action", "lease", "submit"} {
		cmd := resolveCommand(t, name)
		if cmd.Hidden {
			t.Fatalf("%s hidden", name)
		}
		if cmd.Short == "" {
			t.Fatalf("%s lacks help", name)
		}
	}
	bin := buildMindspecBinary(t)
	cmd := exec.Command(bin, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"next-action", "lease", "submit"} {
		if !containsToken(string(out), name) {
			t.Errorf("help missing %s", name)
		}
	}
}
