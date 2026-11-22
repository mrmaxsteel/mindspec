package kernel_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

// subjectEvidenceCommit puts committed evidence on the subject branch itself.
func subjectEvidenceCommit(t *testing.T, f *kerneltest.Fixture, branch, artifact string, mixed, merge bool) string {
	t.Helper()
	base := reducerGit(t, f, "rev-parse", branch)
	evidence := reducerGit(t, f, "rev-parse", "HEAD")
	prior, had := os.LookupEnv("GIT_INDEX_FILE")
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "index"))
	defer func() {
		if had {
			f.Must(os.Setenv("GIT_INDEX_FILE", prior))
		} else {
			f.Must(os.Unsetenv("GIT_INDEX_FILE"))
		}
	}()
	reducerGit(t, f, "read-tree", base)
	for _, name := range []string{"panel.json", "dispositions.jsonl"} {
		rel := filepath.ToSlash(filepath.Join(filepath.Dir(artifact), name))
		oid := reducerGit(t, f, "hash-object", "-w", filepath.Join(f.Root, filepath.FromSlash(rel)))
		reducerGit(t, f, "update-index", "--add", "--cacheinfo", "100644,"+oid+","+rel)
	}
	blob := reducerGit(t, f, "rev-parse", evidence+":"+filepath.ToSlash(artifact))
	reducerGit(t, f, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+filepath.ToSlash(artifact))
	if mixed {
		reducerGit(t, f, "update-index", "--add", "--cacheinfo", "100644,"+blob+",source.txt")
	}
	tree := reducerGit(t, f, "write-tree")
	args := []string{"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit-tree", tree, "-p", base}
	if merge {
		args = append(args, "-p", evidence)
	}
	args = append(args, "-m", "review evidence")
	head := reducerGit(t, f, args...)
	reducerGit(t, f, "update-ref", "refs/heads/"+branch, head)
	return head
}

func TestS134Subject_PanelCommitsOnSubjectBranch(t *testing.T) {
	for _, final := range []bool{false, true} {
		for _, mode := range []string{"panel-only", "mixed", "panel-merge", "mixed-merge", "empty"} {
			t.Run(fmt.Sprintf("final=%t/%s", final, mode), func(t *testing.T) {
				f := reducerFixture(t)
				gate, branch := f.Items[0]+".review", "bead/"+f.Items[0]
				if final {
					gate, branch = f.Epic+".final", "spec/"+f.SpecID
					for _, item := range f.Items {
						doneItem(t, f, item)
					}
				}
				registerGate(t, f, gate, "review", 1)
				reduce(t, f, true)
				old := recoveryRound(t, f, gate)
				slot := gate + ".r1.R2"
				lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: slot}, kernel.LeaseDeps{})
				f.Must(err)
				artifact := recoveryVerdict(t, f, "R2", 1, old.Subject, "verdict-1-R2.json")
				mixed := mode == "mixed" || mode == "mixed-merge" || mode == "empty"

				if mode == "empty" {
					wt := filepath.Join(t.TempDir(), "empty-worktree")
					reducerGit(t, f, "worktree", "add", "--detach", wt, branch)
					reducerGit(t, f, "-C", wt, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "empty subject move")
					head := reducerGit(t, f, "-C", wt, "rev-parse", "HEAD")
					reducerGit(t, f, "update-ref", "refs/heads/"+branch, head)
				} else {
					subjectEvidenceCommit(t, f, branch, artifact, mixed, mode == "panel-merge" || mode == "mixed-merge")
				}
				// A read or apply must not supersede a round for evidence alone.
				reduce(t, f, true)

				next := recoveryRound(t, f, gate)
				if next.Round != old.Round || next.Subject != old.Subject {
					t.Fatalf("panel-only commit moved round: old=%+v new=%+v", old, next)
				}
				_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: slot, Lease: lease.Token, Artifact: artifact})
				f.Must(err)
				if record(t, f, slot).Status != "closed" {
					t.Fatal("panel evidence did not submit")
				}
			})
		}
	}
}
