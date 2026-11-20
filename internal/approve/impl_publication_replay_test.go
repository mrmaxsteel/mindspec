package approve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

func TestS134ImplApprove_FinalizeExportReplay(t *testing.T) {
	for _, mode := range []string{"recorded-tip", "moved-head", "open-epic", "wrong-checkpoint", "unrelated-publication", "source-publication"} {
		t.Run(mode, func(t *testing.T) {
			f, ex, _ := s134ApproveFixture(t)
			accepted, err := kernel.AcceptedSubject(f.State(), f.Epic+".final")
			f.Must(err)
			f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
			f.Must(os.WriteFile(filepath.Join(f.Root, ".beads/issues.jsonl"), []byte("{}\n"), 0600))
			_, err = f.Command("git", "switch", "spec/"+f.SpecID)
			f.Must(err)
			_, err = f.Command("git", "add", "-f", ".beads/issues.jsonl")
			f.Must(err)
			_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "chore: export finalized beads")
			f.Must(err)
			if mode == "source-publication" {
				f.Must(os.WriteFile(filepath.Join(f.Root, "unreviewed.go"), []byte("package source\n"), 0644))
				_, err = f.Command("git", "add", "unreviewed.go")
				f.Must(err)
				_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "unreviewed source")
				f.Must(err)
			}
			data, err := f.Command("git", "rev-parse", "HEAD")
			f.Must(err)
			publication := strings.TrimSpace(string(data))
			if mode == "unrelated-publication" {
				tree, err := f.Command("git", "rev-parse", publication+"^{tree}")
				f.Must(err)
				data, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit-tree", strings.TrimSpace(string(tree)), "-m", "unrelated publication")
				f.Must(err)
				publication = strings.TrimSpace(string(data))
				_, err = f.Command("git", "update-ref", "refs/heads/spec/"+f.SpecID, publication)
				f.Must(err)
			}
			f.Must(f.Store.WriteMetadata(f.Epic, map[string]any{"mindspec_pre_finalize_tip": accepted, "mindspec_finalize_tip": publication}))
			if mode != "open-epic" {
				f.Must(f.Store.SetStatusIf(f.Epic, "open", "closed"))
			}
			if mode == "wrong-checkpoint" {
				f.Must(f.Store.WriteMetadata(f.Epic, map[string]any{"mindspec_pre_finalize_tip": publication}))
			}
			if mode == "moved-head" {
				_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "later source move")
				f.Must(err)
			}
			if mode == "unrelated-publication" {
				if gateErr := kernel.CheckPassedGate(f.State(), f.Epic+".final"); gateErr == nil {
					t.Fatal("publication outside checkpoint ancestry passed the final gate")
				}
			}
			_, err = ApproveImpl(f.Root, f.SpecID, ex)
			if mode == "recorded-tip" {
				if err != nil {
					t.Fatalf("recorded finalizer export prevents cleanup replay: %v", err)
				}
				if len(ex.CallsTo("FinalizeEpic")) != 1 {
					t.Fatal("retry did not reach finalizer")
				}
			} else if err == nil {
				t.Fatalf("%s accepted an unreviewed subject", mode)
			}
		})
	}
}
