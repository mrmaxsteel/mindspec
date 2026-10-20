package panel

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

func subjectGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=Subject Test", "-c", "user.email=subject@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func subjectCommit(t *testing.T, root, name, text string) string {
	t.Helper()
	dest := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	subjectGit(t, root, "add", "--", name)
	subjectGit(t, root, "commit", "-qm", "fixture")
	return subjectGit(t, root, "rev-parse", "HEAD")
}

func TestSubject_PerGate(t *testing.T) {
	for _, specs := range []string{".mindspec/specs", ".mindspec/docs/specs"} {
		for _, gate := range []string{"spec_approve", "plan_approve", "bead", "final_review", "adhoc", ""} {
			t.Run(specs+"/"+gate, func(t *testing.T) {
				dir := specs + "/130-example"
				wantRoots := []string{"."}
				if gate == "spec_approve" || gate == "plan_approve" {
					wantRoots = []string{dir + "/"}
				}
				want := SubjectRoots{wantRoots, []string{specs + "/*/reviews/**", ".mindspec/reviews/**", "review/**", ".beads/issues.jsonl"}}
				inputs := []string{dir, "/repo/" + dir, "/repo/.worktrees/nested/" + dir}
				if gate != "spec_approve" && gate != "plan_approve" {
					inputs = append(inputs, specs, "/repo/"+specs)
				}
				for _, input := range inputs {
					if got := Subject(gate, input); !reflect.DeepEqual(got, want) {
						t.Fatalf("Subject(%q,%q)=%+v want %+v", gate, input, got, want)
					}
				}
			})
		}
	}
}

func TestSubjectDigest_ExcludesPanelRootsOnly(t *testing.T) {
	for _, specs := range []string{".mindspec/specs", ".mindspec/docs/specs"} {
		t.Run(specs, func(t *testing.T) {
			root := t.TempDir()
			subjectGit(t, root, "init", "-q", "-b", "main")
			base := subjectCommit(t, root, specs+"/130/spec.md", "base")
			read := SubjectTreeReader(gitutil.LsTree)
			for _, gate := range []string{"bead", "spec_approve"} {
				before, err := read(root, base, Subject(gate, specs+"/130"))
				if err != nil {
					t.Fatal(err)
				}
				for _, file := range []string{specs + "/130/reviews/p/BRIEF-round-1.md", specs + "/other/reviews/p/dispositions.jsonl", ".mindspec/reviews/p/panel.json", "review/p/R1-round-1.json", ".beads/issues.jsonl"} {
					sha := subjectCommit(t, root, file, gate+file)
					after, err := read(root, sha, Subject(gate, specs+"/130"))
					if err != nil || after != before {
						t.Fatalf("excluded-only commit %s changed digest: %s != %s (%v)", file, after, before, err)
					}
				}
			}
			before, err := read(root, "HEAD", Subject("bead", specs+"/130"))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range []string{"internal/reviews/x.go", "reviews/x.go", specs + "/130/internal/reviews/x.go", ".beads/other.jsonl", "reviewing/x.go", "odd\tname.go", "mode only"} {
				var sha string
				if file == "mode only" {
					// Change only the mode of a file already present in the tree.
					subjectGit(t, root, "update-index", "--chmod=+x", specs+"/130/spec.md")
					subjectGit(t, root, "commit", "-qm", "mode only")
					sha = subjectGit(t, root, "rev-parse", "HEAD")
				} else {
					sha = subjectCommit(t, root, file, "real edit")
				}
				after, err := read(root, sha, Subject("bead", specs+"/130"))
				if err != nil || after == before {
					t.Fatalf("real edit %s did not change digest (%v)", file, err)
				}
				before = after
			}
			// A nested workdir must hash repository-relative paths, including root files.
			got, err := read(filepath.Join(root, specs, "130"), "HEAD", Subject("bead", specs+"/130"))
			if err != nil || got != before {
				t.Fatalf("nested workdir: %s != %s (%v)", got, before, err)
			}
		})
	}
}

func TestSubjectDigest_FailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name, listing string
		err           error
	}{
		{"git error", "", errors.New("unreadable tree")}, {"empty", "", nil}, {"excluded only", "100644 blob abc\t.beads/issues.jsonl\n", nil}, {"outside root", "100644 blob abc\tinternal/a.go\n", nil}, {"malformed", "not a listing\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := SubjectTreeReader(func(string, string) (string, error) { return tc.listing, tc.err })
			if digest, err := read("", "HEAD", Subject("spec_approve", ".mindspec/specs/130")); err == nil || digest != "" {
				t.Fatalf("must refuse: digest=%q err=%v", digest, err)
			}
		})
	}
}

func TestGate_SubjectTreeStaleness(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve", "bead", "final_review", "adhoc", ""} {
		for _, tc := range []struct {
			name, path            string
			wholeStale, specStale bool
		}{
			{"archive", ".mindspec/specs/130/reviews/p/BRIEF-round-1.md", false, false},
			{"ownership", ".mindspec/domains/workflow/OWNERSHIP.yaml", true, false},
			{"real edit", ".mindspec/specs/130/spec.md", true, true},
			{"internal reviews", "internal/reviews/x.go", true, false},
		} {
			t.Run(gate+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				dir := filepath.Join(root, ".mindspec/specs/130/reviews/p")
				base := "100644 blob abc\t.mindspec/specs/130/spec.md\n"
				listing := base + "100644 blob def\t" + tc.path + "\n"
				if tc.name == "real edit" {
					listing = "100644 blob def\t.mindspec/specs/130/spec.md\n"
				}
				digest, err := SubjectDigest(base, Subject(gate, ".mindspec/specs/130"))
				if err != nil {
					t.Fatal(err)
				}
				if err := Create(dir, CreateInput{Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: "abc123", SubjectDigest: digest, Gate: gate}); err != nil {
					t.Fatal(err)
				}
				for i := 1; i <= 2; i++ {
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("R%d-round-1.json", i)), []byte(`{"verdict":"APPROVE"}`), 0644); err != nil {
						t.Fatal(err)
					}
				}
				calls := 0
				facts := ResolveGateFacts(Registration{Dir: dir}, "", root, GateIO{
					RevParse: func(string, string) (string, error) { return "def456", nil },
					SubjectTree: func(_, ref string, s SubjectRoots) (string, error) {
						calls++
						if ref != "def456" {
							t.Fatal(ref)
						}
						return SubjectDigest(listing, s)
					},
				})
				want := Allow
				stale := tc.wholeStale
				if gate == "spec_approve" || gate == "plan_approve" {
					stale = tc.specStale
				}
				if stale {
					want = Block
				}
				if d := PanelGateDecision(facts); d.Action != want || calls != 1 {
					t.Fatalf("action=%v want=%v calls=%d: %s", d.Action, want, calls, d.Message)
				}
			})
		}
	}
}

func TestGate_LegacyStaleMessageUnchanged(t *testing.T) {
	p := &Panel{Round: 1, ReviewedHeadSHA: "abc1234567"}
	reg := &Registration{Dir: "review/p"}
	d := PanelGateDecision(GateFacts{BeadID: "mindspec-test.1", Reg: reg, Res: &Result{Panel: p, LatestRound: 1}, HeadSHA: "def4567890"})
	want := "panel round 1 reviewed abc1234, branch now at def4567 — commits landed after review; bump round and re-panel (/ms-panel-run step 0)\nDo NOT bypass with raw `git merge bead/mindspec-test.1` — it skips bd closure, worktree cleanup, and this gate; only `mindspec complete` merges bead branches."
	if d.Action != Block || d.Message != want {
		t.Fatalf("legacy message changed:\n%q", d.Message)
	}
}

func TestGate_EqualSHANoSubjectCall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "review/p")
	if err := Create(dir, CreateInput{Round: 1, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:recorded"}); err != nil {
		t.Fatal(err)
	}
	f := ResolveGateFacts(Registration{Dir: dir}, "", filepath.Dir(dir), GateIO{RevParse: func(string, string) (string, error) { return "abc123", nil }, SubjectTree: func(string, string, SubjectRoots) (string, error) {
		t.Fatal("equal SHA must make no subject call")
		return "", nil
	}})
	if f.SubjectErr != nil || f.HeadSHA != "abc123" {
		t.Fatalf("facts=%+v", f)
	}
}

func TestSubjectDigest_GateErrorBlocks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "review/p")
	if err := Create(dir, CreateInput{Round: 1, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:recorded"}); err != nil {
		t.Fatal(err)
	}
	for _, reader := range []func(string, string, SubjectRoots) (string, error){nil, func(string, string, SubjectRoots) (string, error) { return "", nil }, func(string, string, SubjectRoots) (string, error) { return "", errors.New("tree unavailable") }} {
		f := ResolveGateFacts(Registration{Dir: dir}, "", filepath.Dir(dir), GateIO{RevParse: func(string, string) (string, error) { return "def456", nil }, SubjectTree: reader})
		if d := PanelGateDecision(f); d.Action != Block || !strings.Contains(d.Message, "could not verify subject tree") {
			t.Fatalf("must block: %+v", d)
		}
	}
}

func TestResolveGateFacts_LegacySkipsSubjectTree(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "review/p")
	if err := Create(dir, CreateInput{Round: 1, ReviewedHeadSHA: "abc1234567"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	facts := ResolveGateFacts(Registration{Dir: dir}, "mindspec-test.1", root, GateIO{
		RevParse: func(string, string) (string, error) { return "def4567890", nil },
		SubjectTree: func(string, string, SubjectRoots) (string, error) {
			calls++
			return "", errors.New("legacy subject must not be read")
		},
	})
	if calls != 0 || facts.SubjectErr != nil {
		t.Errorf("legacy subject calls=%d err=%v, want no call and nil error", calls, facts.SubjectErr)
	}
	want := "panel round 1 reviewed abc1234, branch now at def4567 — commits landed after review; bump round and re-panel (/ms-panel-run step 0)\nDo NOT bypass with raw `git merge bead/mindspec-test.1` — it skips bd closure, worktree cleanup, and this gate; only `mindspec complete` merges bead branches."
	if d := PanelGateDecision(facts); d.Action != Block || d.Message != want {
		t.Errorf("legacy action=%v message=%q, want Block and %q", d.Action, d.Message, want)
	}
}
