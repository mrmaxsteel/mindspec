package kernel_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

func TestS134Finalize_RecordedPanelPaths(t *testing.T) {
	f := reducerFixture(t)
	for gate, dir := range map[string]string{f.Items[0] + ".review": "review/bead", f.Epic + ".final": "reviews/final", "other-1.final": "reviews/other-spec-panel"} {
		f.Must(os.MkdirAll(filepath.Join(f.Root, dir), 0755))
		if gate == "other-1.final" {
			f.Fake.Records[gate] = kernel.Record{ID: gate, Parent: "other-1", Type: "step", Status: "closed", Metadata: map[string]any{kernel.KeyTuple: kernel.Tuple{Epic: "other-1", Gate: gate, Kind: "final-gate", Attempt: 1}, kernel.KeyRegistration: kernel.RegistrationRecord{Slug: "honest", Directories: []string{dir}}}}
			continue
		}
		f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, kernel.RegistrationRecord{Slug: "honest", Directories: []string{dir}})))
	}
	s := f.State()
	s.Gates = append(s.Gates, "other-1.final")
	s.Records["other-1.final"] = f.Fake.Records["other-1.final"]
	if got, want := kernel.FinalizePanelPaths(s), []string{"review/bead/", "reviews/final/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("paths=%v want=%v", got, want)
	}
	// Old records grant only the canonical directory inside this spec.
	f.Must(f.Store.WriteMetadata(f.Epic+".final", kernel.Metadata(kernel.KeyRegistration, kernel.RegistrationRecord{Slug: "old"})))
	f.Must(os.MkdirAll(filepath.Join(f.SpecDir, "reviews", "old"), 0755))
	rel, err := filepath.Rel(f.Root, filepath.Join(f.SpecDir, "reviews", "old"))
	f.Must(err)
	if got, want := kernel.FinalizePanelPaths(f.State()), []string{filepath.ToSlash(rel) + "/", "review/bead/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy paths=%v want=%v", got, want)
	}
}

func TestS134Finalize_PanelPathFilters(t *testing.T) {
	for _, mode := range []string{"unenrolled-item", "unowned-gate"} {
		t.Run(mode, func(t *testing.T) {
			f := reducerFixture(t)
			s := f.State()
			gate := f.Epic + ".99.review"
			tuple := kernel.Tuple{Epic: f.Epic, WorkItem: f.Epic + ".99", Gate: gate, Kind: "bead-gate", Attempt: 1}
			if mode == "unowned-gate" {
				gate = f.Epic + ".final"
				tuple = kernel.Tuple{Epic: "other-1", Gate: "other-1.final", Kind: "final-gate", Attempt: 1}
			} else {
				s.Gates = append(s.Gates, gate)
			}
			s.Records[gate] = kernel.Record{ID: gate, Parent: f.Epic, Type: "step", Status: "open", Metadata: map[string]any{kernel.KeyTuple: tuple, kernel.KeyRegistration: kernel.RegistrationRecord{Slug: "foreign", Directories: []string{"review/foreign"}}}}
			if got := kernel.RegisteredPanelPaths(s, gate); !reflect.DeepEqual(got, []string{"review/foreign/"}) {
				t.Fatalf("fixture registration not reachable: %v", got)
			}
			if got := kernel.FinalizePanelPaths(s); len(got) != 0 {
				t.Fatalf("unqualified gate granted paths: %v", got)
			}
		})
	}
}

func TestS134Finalize_MissingRecordedPanelDirectory(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Epic + ".final"
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, kernel.RegistrationRecord{Slug: "panel", Directories: []string{"review/panel"}})))
	dir := filepath.Join(f.Root, "review", "panel")
	f.Must(os.MkdirAll(dir, 0755))
	f.Must(os.Rename(dir, filepath.Join(t.TempDir(), "panel")))
	if got, want := kernel.FinalizePanelPaths(f.State()), []string{"review/panel/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing recorded directory: paths=%v want=%v", got, want)
	}
}
