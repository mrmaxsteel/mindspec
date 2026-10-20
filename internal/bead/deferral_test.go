package bead

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeferral_Checks(t *testing.T) {
	for _, tc := range []struct {
		name, body, parent string
		ok                 bool
	}{
		{"open", `[{"id":"follow-up","status":"open","description":"fix <path>","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`, "", true},
		{"closed", `[{"id":"follow-up","status":"closed"}]`, "", false},
		{"missing", `[]`, "", false},
		{"grandchild", `[{"id":"follow-up","status":"open","parent":"child-x","description":"fix <path>","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`, `[{"id":"child-x","parent":"epic-x"}]`, false},
		{"no edge", `[{"id":"follow-up","status":"open","description":"fix <path>"}]`, "", false},
		{"wrong text", `[{"id":"follow-up","status":"open","description":"other","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "bd"), []byte("#!/bin/sh\ncase \"$2\" in\nfollow-up) cat '"+dir+"/issue';;\nchild-x) cat '"+dir+"/parent';;\n*) exit 1;;\nesac\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "issue"), []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "parent"), []byte(tc.parent), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			redact := func(s string) string { return strings.ReplaceAll(s, "/tmp/x", "<path>") }
			err := CheckDeferral("follow-up", "epic-x", "", []string{"fix /tmp/x"}, redact)
			if (err == nil) != tc.ok {
				t.Fatalf("check = %v, want allowed %v", err, tc.ok)
			}
		})
	}
}
func TestDeferral_InvalidIDNoSpawn(t *testing.T) {
	dir := t.TempDir()
	called := filepath.Join(dir, "called")
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte("#!/bin/sh\nprintf called > '"+called+"'\necho '[]'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := CheckDeferral("--hostile", "epic-x", "", []string{"fix"}, func(s string) string { return s }); err == nil {
		t.Fatal("invalid id accepted")
	}
	if _, err := os.Stat(called); !os.IsNotExist(err) {
		t.Fatalf("invalid id reached bd: %v", err)
	}
}
