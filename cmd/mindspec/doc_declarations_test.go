package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS133DocOwner_CLIAcceptedCheckFlags(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, policy := range []string{"warn", "enforce"} {
		for _, mode := range []string{"declaration", "open-owner", "closed-owner"} {
			t.Run(policy+"/"+mode, func(t *testing.T) {
				root := s133CLIFixture(t, policy, "doc")
				fake := t.TempDir()
				script := `#!/bin/sh
case "$1 $2" in
 'show bead-1') echo '[{"id":"bead-1","status":"in_progress","metadata":{"sentinel":"unchanged"},"dependencies":[{"id":"epic-docs","issue_type":"epic","dependency_type":"parent-child"}]}]' ;;
 'show doc-owner') printf '[{"id":"doc-owner","status":"%s","metadata":{},"dependencies":[{"id":"epic-docs","issue_type":"epic","dependency_type":"parent-child"}]}]\n' "$S133_DOC_STATUS" ;;
 'show epic-docs'|'list --type=epic') echo '[{"id":"epic-docs","issue_type":"epic","status":"open","metadata":{"spec_id":"999-check","spec_title":"999-check","mindspec_phase":"implement"}}]' ;;
 'worktree list') echo '[]' ;;
 *) echo "unexpected tracker operation: $*" >&2; exit 99 ;;
esac
`
				if err := os.WriteFile(filepath.Join(fake, "bd"), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
				status := "open"
				if mode == "closed-owner" {
					status = "closed"
				}
				t.Setenv("S133_DOC_STATUS", status)
				state := func() string {
					data, err := os.ReadFile(filepath.Join(root, ".beads/tracker.json"))
					if err != nil {
						t.Fatal(err)
					}
					return s133CLIGit(t, root, "status", "--porcelain") + s133CLIGit(t, root, "rev-parse", "HEAD") + string(data)
				}
				args := []string{"complete", "bead-1", "--spec", "999-check", "--check"}
				if mode == "declaration" {
					args = append(args, "--no-doc-impact", "core: internal change, no public effect")
				} else {
					args = append(args, "--allow-doc-skew", "docs later", "--doc-owner", "doc-owner")
				}
				before := state()
				out, err := runBinary(t, bin, root, args...)
				if before != state() {
					t.Fatal("check changed git or tracker bytes")
				}
				if mode == "closed-owner" {
					if err == nil || !strings.Contains(out, "doc-sync owner") {
						t.Fatalf("closed owner must refuse: %v\n%s", err, out)
					}
					return
				}
				if err != nil || !strings.Contains(out, "core") || !strings.Contains(out, "Content gates passed.") {
					t.Fatalf("accepted flags must pass and name the domain: %v\n%s", err, out)
				}
				if mode == "declaration" && !strings.Contains(out, "internal change, no public effect") {
					t.Fatalf("flag parser lost reason: %s", out)
				}
			})
		}
	}
}
