package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

const surfaceSpecBase = "371dc360"

var surfacePlainRefs = regexp.MustCompile(`\bADR[- ]?[0-9]{4}\b|\b[Ss]pec[ _-]?[0-9]{3}\b|\bS[0-9]{3}\b|\b(AC|OQ)[0-9]+\b|§|issues\.jsonl`)
var surfaceRuleRefs = regexp.MustCompile(`\b[RB][0-9]{1,2}\b`)
var surfaceIssueRefs = regexp.MustCompile(`#[0-9]+\b`)
var surfaceBeadRefs = regexp.MustCompile(`mindspec-[a-z0-9]{3,8}(\.[0-9]+)*`)
var surfaceHunk = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,[0-9]+)? @@`)

var surfaceAllowedWords = map[string]bool{
	"owned": true, "required": true, "managed": true, "level": true,
	"fixture": true, "driven": true, "approved": true, "written": true,
	"started": true, "rendered": true, "internal": true, "emitted": true,
	"complete": true, "build": true,
}

// surfaceInternalRefs applies RE2 patterns, then checks the contexts that RE2
// cannot express with lookaround. The result names each offending token.
func surfaceInternalRefs(line string) []string {
	hits := surfacePlainRefs.FindAllString(line, -1)
	for _, loc := range surfaceRuleRefs.FindAllStringIndex(line, -1) {
		start, end := loc[0], loc[1]
		quoted := start > 0 && end < len(line) && line[start-1] == '"' && line[end] == '"'
		dotted := start > 0 && line[start-1] == '.'
		if !quoted && !dotted {
			hits = append(hits, line[start:end])
		}
	}
	for _, loc := range surfaceIssueRefs.FindAllStringIndex(line, -1) {
		if loc[0] == 0 || strings.ContainsRune(" \"`([{,;:", rune(line[loc[0]-1])) {
			hits = append(hits, line[loc[0]:loc[1]])
		}
	}
	for _, loc := range surfaceBeadRefs.FindAllStringIndex(line, -1) {
		start, end := loc[0], loc[1]
		before, _ := utf8.DecodeLastRuneInString(line[:start])
		after, _ := utf8.DecodeRuneInString(line[end:])
		word := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' }
		if (start > 0 && (word(before) || before == '.')) || (end < len(line) && word(after)) {
			continue
		}
		token := line[start:end]
		if !surfaceAllowedWords[strings.TrimPrefix(token, "mindspec-")] {
			hits = append(hits, token)
		}
	}
	return hits
}

func surfaceScanPath(path string) bool {
	if strings.HasPrefix(path, ".claude/skills/") || strings.HasPrefix(path, "plugins/mindspec/skills/") {
		return true
	}
	if !strings.HasPrefix(path, "cmd/") && !strings.HasPrefix(path, "internal/") {
		return false
	}
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") &&
		!strings.Contains("/"+path, "/testdata/")
}

// Each scanner has its own immutable history floor. Temp repositories use their
// root commit; the branch test uses the pinned spec base.
type surfaceScanner struct {
	repo, floor string
}

func surfaceGit(repo string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

// scan compares committed trees only. It refuses bases that could hide changes
// from this branch, and never treats missing history as a reason to skip.
func (s surfaceScanner) scan(base, head, mainRef string) error {
	resolve := func(label, ref string) (string, error) {
		out, err := surfaceGit(s.repo, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
		if err != nil {
			return "", fmt.Errorf("missing %s %q; run git fetch origin main or a full-history fetch: %w", label, ref, err)
		}
		return strings.TrimSpace(out), nil
	}
	baseOID, err := resolve("base", base)
	if err != nil {
		return err
	}
	headOID, err := resolve("head", head)
	if err != nil {
		return err
	}
	mainOID, err := resolve("main ref", mainRef)
	if err != nil {
		return err
	}
	floorOID, err := resolve("history floor", s.floor)
	if err != nil {
		return err
	}
	if baseOID == headOID {
		return fmt.Errorf("base must not equal head")
	}
	for _, rule := range []struct{ ancestor, descendant, message string }{
		{baseOID, headOID, "base must be an ancestor of head"},
		{floorOID, baseOID, "base must include the pinned history floor as an ancestor"},
		{baseOID, mainOID, "base must be reachable from the main ref"},
	} {
		if _, err := surfaceGit(s.repo, "merge-base", "--is-ancestor", rule.ancestor, rule.descendant); err != nil {
			return fmt.Errorf("%s; run git fetch origin main or a full-history fetch: %w", rule.message, err)
		}
	}
	paths, err := surfaceGit(s.repo, "diff", "--no-ext-diff", "--no-textconv", "--no-renames",
		"--name-only", "-z", baseOID, headOID, "--")
	if err != nil {
		return err
	}
	var violations []string
	for _, path := range strings.Split(paths, "\x00") {
		if !surfaceScanPath(path) {
			continue
		}
		diff, err := surfaceGit(s.repo, "diff", "--no-ext-diff", "--no-textconv", "--no-renames",
			"--no-color", "--text", "--unified=0", baseOID, headOID, "--", ":(literal)"+path)
		if err != nil {
			return err
		}
		inHunk, lineNumber := false, 0
		for _, line := range strings.Split(diff, "\n") {
			if hunk := surfaceHunk.FindStringSubmatch(line); hunk != nil {
				lineNumber, err = strconv.Atoi(hunk[1])
				if err != nil {
					return fmt.Errorf("invalid diff hunk: %w", err)
				}
				inHunk = true
				continue
			}
			if !inHunk || len(line) == 0 {
				continue
			}
			switch line[0] {
			case '+':
				if hits := surfaceInternalRefs(line[1:]); len(hits) > 0 {
					violations = append(violations, fmt.Sprintf("%s:%d: internal references %q", path, lineNumber, hits))
				}
				lineNumber++
			case ' ':
				lineNumber++
			}
		}
	}
	if len(violations) > 0 {
		return fmt.Errorf("surface scan failed:\n%s", strings.Join(violations, "\n"))
	}
	return nil
}

func TestS134SurfaceMatcher_PlantedReferences(t *testing.T) {
	for _, line := range []string{
		"ADR-0035", "ADR 0035", "ADR0035", "spec 134", "spec_134", "spec-134", "Spec134", "S134",
		"AC14", "OQ8", "(R3", `"B7: ..."`, `"R11 forbids"`, "§8", "#7", "#146", `"#146 broke"`,
		"mindspec-xaoq", "mindspec-4qd", "mindspec-2fy.1", ".beads/issues.jsonl", "mindspec-kernel",
		"mindspec-abc.12.3", "mindspec-owned.1", "B12", "R99",
	} {
		t.Run(line, func(t *testing.T) {
			if got := surfaceInternalRefs(line); len(got) == 0 {
				t.Fatalf("planted internal reference went undetected: %q", line)
			}
		})
	}
}

func TestS134SurfaceMatcher_AllowedContexts(t *testing.T) {
	lines := []string{`"R1"`, ".R3", "mindspec-owned", ".mindspec-scratch", `"#ffffff"`, `"%#v"`,
		"ARR3", "R100", "XAC14", "S1340", "spec 1340", "ADR-00350", "mindspec-abcdefgh9",
		"xmindspec-xaoq", "9mindspec-xaoq", "-mindspec-xaoq", ".mindspec-xaoq", "mindspec-xaoq-more",
		"é-mindspec-xaoq", "émindspec-xaoq", "mindspec-xaoqé", "x#146", "/#146", "'#146", "\t#146"}
	for _, word := range []string{"owned", "required", "managed", "level", "fixture", "driven", "approved",
		"written", "started", "rendered", "internal", "emitted", "complete", "build"} {
		lines = append(lines, "mindspec-"+word)
	}
	for _, line := range lines {
		t.Run(line, func(t *testing.T) {
			if got := surfaceInternalRefs(line); len(got) != 0 {
				t.Fatalf("allowed text reported internal references: %q: %q", line, got)
			}
		})
	}
}

func TestS134SurfaceMatcher_ContextChecks(t *testing.T) {
	for _, prefix := range []string{"", " ", `"`, "`", "(", "[", "{", ",", ";", ":"} {
		t.Run("issue_"+prefix, func(t *testing.T) {
			if got := surfaceInternalRefs(prefix + "#146"); len(got) != 1 || got[0] != "#146" {
				t.Fatalf("issue context lost: %q", got)
			}
		})
	}
	for _, line := range []string{`"R3" R3`, ".R3 (R3", "xmindspec-xaoq mindspec-xaoq", "x#146 #146"} {
		if got := surfaceInternalRefs(line); len(got) != 1 {
			t.Errorf("an allowed match hid a forbidden match in %q: %q", line, got)
		}
	}
}

func surfaceTestGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := surfaceGit(repo, args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func surfaceTestWrite(t *testing.T, repo, path, text string) {
	t.Helper()
	abs := filepath.Join(repo, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func surfaceTestCommit(t *testing.T, repo string) string {
	t.Helper()
	surfaceTestGit(t, repo, "add", "--all")
	surfaceTestGit(t, repo, "-c", "core.hooksPath=/dev/null", "-c", "user.name=Surface Test",
		"-c", "user.email=surface@example.invalid", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "fixture")
	return surfaceTestGit(t, repo, "rev-parse", "HEAD")
}

func surfaceTestRepo(t *testing.T) (surfaceScanner, string) {
	t.Helper()
	repo := t.TempDir()
	surfaceTestGit(t, repo, "init", "--initial-branch=main")
	surfaceTestWrite(t, repo, "cmd/base.go", "package main\n// ADR-0035\n")
	base := surfaceTestCommit(t, repo)
	surfaceTestGit(t, repo, "update-ref", "refs/remotes/origin/main", base)
	return surfaceScanner{repo: repo, floor: base}, base
}

func TestS134SurfaceDiffFn_PathClasses(t *testing.T) {
	for _, tc := range []struct {
		path string
		fail bool
	}{
		{"cmd/nested/new.go", true}, {"internal/new.go", true},
		{".claude/skills/example/SKILL.md", true}, {"plugins/mindspec/skills/example/SKILL.md", true},
		{"cmd/new_test.go", false}, {"internal/new_test.go", false},
		{"cmd/testdata/new.go", false}, {"internal/nested/testdata/new.go", false},
		{"testdata/new.go", false}, {"project-docs/new.md", false}, {"cmd/new.md", false},
		{".claude/skills/example/data.txt", true}, {"plugins/mindspec/skills/example/helper.go", true},
		{"cmd/a\nodd.go", true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			s, base := surfaceTestRepo(t)
			surfaceTestWrite(t, s.repo, tc.path, "// ADR-0035\n")
			head := surfaceTestCommit(t, s.repo)
			err := s.scan(base, head, "origin/main")
			if (err != nil) != tc.fail {
				t.Fatalf("scan(%q) error = %v, want failure %t", tc.path, err, tc.fail)
			}
			if tc.fail && (!strings.Contains(err.Error(), tc.path+":1:") || !strings.Contains(err.Error(), "ADR-0035")) {
				t.Fatalf("failure did not identify the planted line: %v", err)
			}
		})
	}
}

func TestS134SurfaceDiffFn_ChangedAndUnchangedLines(t *testing.T) {
	s, base := surfaceTestRepo(t)
	surfaceTestWrite(t, s.repo, "cmd/base.go", "package main\n// ADR-0035\n// plain text\n")
	head := surfaceTestCommit(t, s.repo)
	if err := s.scan(base, head, "origin/main"); err != nil {
		t.Fatalf("unchanged base reference must pass: %v", err)
	}
	surfaceTestWrite(t, s.repo, "cmd/base.go", "package main\n// ADR-0035 changed\n// plain text\n// #146\n")
	head = surfaceTestCommit(t, s.repo)
	err := s.scan(base, head, "origin/main")
	if err == nil || !strings.Contains(err.Error(), "cmd/base.go:2:") || !strings.Contains(err.Error(), "cmd/base.go:4:") {
		t.Fatalf("changed line and added line must both fail at their head line numbers: %v", err)
	}
	surfaceTestWrite(t, s.repo, "cmd/base.go", "package main\n// plain text\n")
	head = surfaceTestCommit(t, s.repo)
	if err := s.scan(base, head, "origin/main"); err != nil {
		t.Fatalf("removed references must pass: %v", err)
	}
}

func TestS134SurfaceDiffFn_BaseRules(t *testing.T) {
	s, base := surfaceTestRepo(t)
	head := surfaceTestCommit(t, s.repo)
	for _, tc := range []struct{ name, base, head, main, floor, want string }{
		{"missing-base", "missing", head, "origin/main", base, "missing base"},
		{"missing-main", base, head, "missing-main", base, "missing main ref"},
		{"missing-head", base, "missing-head", "origin/main", base, "missing head"},
		{"missing-floor", base, head, "origin/main", "missing-floor", "missing history floor"},
		{"equal-head", head, head, "origin/main", base, "base must not equal head"},
		{"not-ancestor", head, base, "origin/main", base, "base must be an ancestor of head"},
		{"before-floor", base, head, "origin/main", head, "base must include the pinned history floor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.floor = tc.floor
			err := s.scan(tc.base, tc.head, tc.main)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want rule %q, got %v", tc.want, err)
			}
			if strings.HasPrefix(tc.name, "missing-") && !strings.Contains(err.Error(), "git fetch origin main") {
				t.Fatalf("missing ref must name fetch recovery: %v", err)
			}
		})
	}
}

func TestS134SurfaceDiffFn_SpecBranchBaseRefused(t *testing.T) {
	s, base := surfaceTestRepo(t)
	surfaceTestWrite(t, s.repo, "cmd/new.go", "// ADR-0035\n")
	specCommit := surfaceTestCommit(t, s.repo)
	head := surfaceTestCommit(t, s.repo)
	if err := s.scan(specCommit, head, "origin/main"); err == nil || !strings.Contains(err.Error(), "base must be reachable from the main ref") {
		t.Fatalf("spec-only base must not hide its lines: %v", err)
	}
	if err := s.scan(base, head, "origin/main"); err == nil || !strings.Contains(err.Error(), "cmd/new.go:1:") {
		t.Fatalf("valid base must expose the spec line: %v", err)
	}
}

func TestS134SurfaceDiffFn_MergedMainBase(t *testing.T) {
	s, base := surfaceTestRepo(t)
	specCommit := surfaceTestCommit(t, s.repo)
	surfaceTestWrite(t, s.repo, "internal/main.go", "// ADR-0035\n")
	candidate := surfaceTestCommit(t, s.repo)
	tree := surfaceTestGit(t, s.repo, "rev-parse", candidate+"^{tree}")
	mainCommit := surfaceTestGit(t, s.repo, "-c", "user.name=Surface Test", "-c", "user.email=surface@example.invalid",
		"commit-tree", tree, "-p", base, "-m", "main advance")
	surfaceTestGit(t, s.repo, "update-ref", "refs/remotes/origin/main", mainCommit)
	// Build a merge graph locally without invoking merge or checking out a branch.
	merge := surfaceTestGit(t, s.repo, "-c", "user.name=Surface Test", "-c", "user.email=surface@example.invalid",
		"commit-tree", tree, "-p", specCommit, "-p", mainCommit, "-m", "merged main")
	if err := s.scan(mainCommit, merge, "origin/main"); err != nil {
		t.Fatalf("merged main's lines must pass: %v", err)
	}
	if err := s.scan(base, merge, "origin/main"); err == nil || !strings.Contains(err.Error(), "internal/main.go:1:") {
		t.Fatalf("old base must still expose main's planted line: %v", err)
	}
}
