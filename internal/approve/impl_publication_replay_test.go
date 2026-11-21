package approve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

func TestS134ImplApprove_FinalizeExportReplay(t *testing.T) {
	for _, mode := range []string{"recorded-tip", "moved-head", "open-epic", "wrong-checkpoint", "unrelated-publication", "source-publication", "review/evil/x.go", "review/evil/x_test.go", "reviews/evil/x.go", "reviews/evil/x_test.go", "reviews/other-spec-panel/x.go", "reviews/other-spec-panel/x_test.go", "recorded-bead", "recorded-final"} {
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
			name := ""
			if mode == "source-publication" {
				name = "unreviewed.go"
			} else if strings.Contains(mode, "/") {
				name = mode
			} else if mode == "recorded-bead" || mode == "recorded-final" {
				gate := f.Epic + ".final"
				if mode == "recorded-bead" {
					gate = f.Items[0] + ".review"
				}
				reg, _, err := kernel.Decode[kernel.RegistrationRecord](f.State().Records[gate].Metadata, kernel.KeyRegistration)
				f.Must(err)
				reg.Directories = []string{"review/honest"}
				f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, reg)))
				name = "review/honest/x.go"
			}
			// Another spec's registration never grants publication authority.
			f.Fake.Records["other-1.final"] = kernel.Record{ID: "other-1.final", Parent: "other-1", Type: "step", Status: "closed", Metadata: map[string]any{kernel.KeyTuple: kernel.Tuple{Epic: "other-1", Gate: "other-1.final", Kind: "final-gate", Attempt: 1}, kernel.KeyRegistration: kernel.RegistrationRecord{Slug: "other-spec-panel", Directories: []string{"reviews/other-spec-panel"}}}}
			if name != "" {
				f.Must(os.MkdirAll(filepath.Dir(filepath.Join(f.Root, name)), 0755))
				f.Must(os.WriteFile(filepath.Join(f.Root, name), []byte("package source\n"), 0644))
				_, err = f.Command("git", "add", name)
				f.Must(err)
				_, err = f.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "publication content")
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
			if mode == "recorded-tip" || strings.HasPrefix(mode, "recorded-") {
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
