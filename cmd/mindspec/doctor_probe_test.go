package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
)

func TestDoctor_ProbeFlag(t *testing.T) {
	flag := doctorCmd.Flags().Lookup("probe")
	if flag == nil || flag.DefValue != "false" || flag.Value.Type() != "bool" {
		t.Fatalf("doctor must offer --probe, disabled by default: %+v", flag)
	}
	if !strings.Contains(doctorCmd.Long, "--probe") {
		t.Fatal("doctor help must explain probe consent")
	}
	var out bytes.Buffer
	printDoctorChecks(&out, []doctor.Check{{Name: "Panel probe", Status: doctor.Info, Message: "probe was not run"}})
	if got, want := out.String(), "Panel probe: [INFO] probe was not run\n"; got != want {
		t.Fatalf("Info line = %q, want %q", got, want)
	}
	if (&doctor.Report{Checks: []doctor.Check{{Status: doctor.Info}}}).HasFailures() {
		t.Fatal("Info must not fail doctor")
	}
	bin := buildMindspecBinary(t)
	for _, tc := range []struct {
		name string
		args []string
		runs bool
	}{
		{"plain", nil, false},
		{"explicit", []string{"--probe"}, true},
		{"ci", []string{"--ci", "--probe"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := mkMigrationWorkspace(t)
			probe := filepath.Join(root, "probe")
			if err := os.WriteFile(probe, []byte("#!/bin/sh\n: > probe-marker\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			contents := fmt.Sprintf("panel:\n  reviewers: [{family: local-test, count: 2}]\n  runners:\n    - {family: local-test, kind: process, argv: [/bin/sh], prompt: stdin, output: stdout, probe: [%q]}\n", probe)
			if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, append([]string{"doctor"}, tc.args...)...)
			cmd.Dir = root
			cmd.Env = strippedEnv(t)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("doctor %v: %v\n%s", tc.args, err, output)
			}
			_, statErr := os.Stat(filepath.Join(root, "probe-marker"))
			if tc.runs && statErr != nil || !tc.runs && !os.IsNotExist(statErr) {
				t.Fatalf("probe execution: want runs=%v, stat=%v\n%s", tc.runs, statErr, output)
			}
			switch tc.name {
			case "plain":
				if !strings.Contains(string(output), "Panel probe: [INFO]") {
					t.Fatalf("plain doctor must report skipped probe:\n%s", output)
				}
			case "explicit":
				if !strings.Contains(string(output), "probe passed") {
					t.Fatalf("--probe must run configured probe:\n%s", output)
				}
			case "ci":
				if strings.Contains(string(output), "Panel probe:") || strings.Contains(string(output), "Panel launcher:") {
					t.Fatalf("--ci must omit runner environment lines:\n%s", output)
				}
			}
		})
	}
}
