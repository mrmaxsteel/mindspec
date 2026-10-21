package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/spf13/cobra"
)

func TestPanelSchema(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := panelSchemaCmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(out.Bytes(), &schema); err != nil {
		t.Fatal(err)
	}
	expected, err := panel.VerdictV2Schema()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != string(expected) {
		t.Fatal("CLI did not print canonical schema")
	}
	if schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Fatalf("wrong schema %v", schema)
	}
	found := false
	for _, child := range panelCmd.Commands() {
		if child == panelSchemaCmd {
			found = true
		}
	}
	if !found {
		t.Fatal("schema command not wired")
	}
}
func TestLandedBindingFor(t *testing.T) {
	calls := 0
	read := func(id string) (map[string]interface{}, error) {
		calls++
		if id != "b" {
			t.Fatalf("id %s", id)
		}
		return map[string]interface{}{"mindspec_landed_review_panel": "p@r2"}, nil
	}
	if got, err := landedBindingFor(read, "", "p", 2); err != nil || got || calls != 0 {
		t.Fatal("adhoc read metadata")
	}
	for _, tc := range []struct {
		slug  string
		round int
		want  bool
	}{{"p", 2, true}, {"p", 1, false}, {"q", 2, false}} {
		got, err := landedBindingFor(read, "b", tc.slug, tc.round)
		if err != nil || got != tc.want {
			t.Fatalf("match %+v: %v %v", tc, got, err)
		}
	}
	injected := errors.New("metadata unavailable")
	if _, err := landedBindingFor(func(string) (map[string]interface{}, error) { return nil, injected }, "b", "p", 2); !errors.Is(err, injected) {
		t.Fatalf("swallowed metadata error %v", err)
	}
}
func TestFindPanelRoot_NoSlotFallsBack(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	expected, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	got, err := findPanelRoot(func(string) (string, error) { t.Fatal("no slot should not load config"); return "", nil })
	if err != nil || got != expected {
		t.Fatalf("fallback got %s want %s: %v", got, expected, err)
	}
}
func TestDispositionAppend_RefusesV2Kinds(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "specs", "999-test")
	if err := os.MkdirAll(specDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	for _, kind := range []string{"verdict", "launch", "substitution"} {
		t.Run(kind, func(t *testing.T) {
			cmd := panelDispositionAppendCmd
			for key, value := range map[string]string{"spec": "999-test", "panel": "p", "data": `{"record":"` + kind + `"}`} {
				if err := cmd.Flags().Set(key, value); err != nil {
					t.Fatal(err)
				}
			}
			err := cmd.RunE(cmd, nil)
			if err == nil || !strings.Contains(err.Error(), "reserved") {
				t.Fatalf("kind %s not refused as reserved: %v", kind, err)
			}
			if _, err := os.Stat(filepath.Join(specDir, "reviews", "p", "dispositions.jsonl")); !os.IsNotExist(err) {
				t.Fatal("append wrote forbidden record")
			}
		})
	}
}
