package bead

import (
	"errors"
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

// TestDeferral_ReportsEveryMissingRequirement: a follow-up bead with no
// discovered-from edge AND a description that lacks the finding gets one
// refusal naming both, with the exact edge command. The old check
// stopped at the first miss.
func TestDeferral_ReportsEveryMissingRequirement(t *testing.T) {
	dir := t.TempDir()
	body := `[{"id":"follow-up","status":"open","description":"other"}]`
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte("#!/bin/sh\ncat '"+dir+"/issue'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issue"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := CheckDeferral("follow-up", "epic-x", "", []string{"fix the parser"}, func(s string) string { return s })
	if err == nil {
		t.Fatal("check = nil, want refusal")
	}
	for _, want := range []string{
		"discovered-from edge",
		"bd dep add follow-up epic-x --type discovered-from",
		"must contain every deferred finding",
		"fix the parser",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks %q:\n%v", want, err)
		}
	}
	var reqErr *DeferralRequirementsError
	if !errors.As(err, &reqErr) || !strings.HasPrefix(reqErr.Recovery, "bd dep add follow-up epic-x --type discovered-from") {
		t.Errorf("recovery must start with the edge command: %#v", err)
	}
}
