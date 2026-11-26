package kernel

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// slotRecord reads the verdict file of a slot submit and returns its bytes,
// which Submit stores as the step record. The file is the verdict that the
// slot prompt names, in an attempt directory of this slot, or a file in the
// registered panel directory. When the gate has no registered panel, a file
// in a worktree of this repository is accepted. The read writes nothing.
func slotRecord(s State, t Tuple, name string) (string, error) {
	checked, err := checkSuppliedPath(s.Root, name, slotRecovery(s, t))
	if err != nil {
		return "", pathFailure(err, "verdict file is not readable: ", slotRecovery(s, t))
	}
	if !checked.info.Mode().IsRegular() {
		return "", termsafe.NewFailure("verdict file is not a regular file", slotRecovery(s, t))
	}
	if !inSlotAttempt(s, t, checked.resolved) {
		if err := slotFileInPanel(s, t, checked.resolved); err != nil {
			return "", err
		}
	}
	f, err := os.Open(checked.resolved)
	if err != nil {
		return "", pathFailure(err, "verdict file is not readable: ", slotRecovery(s, t))
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, panel.MaxVerdictBytes+1))
	if err != nil {
		return "", pathFailure(err, "verdict file is not readable: ", slotRecovery(s, t))
	}
	if len(data) > panel.MaxVerdictBytes {
		return "", termsafe.NewFailure(fmt.Sprintf("verdict file is larger than %d bytes", panel.MaxVerdictBytes), "make the verdict shorter than "+strconv.Itoa(panel.MaxVerdictBytes)+" bytes (put long evidence in a file and name its path), then run the same submit command again")
	}
	if !utf8.Valid(data) {
		return "", termsafe.NewFailure("verdict file is not valid UTF-8", slotRecovery(s, t))
	}
	return string(data), nil
}

// slotFileInPanel accepts a verdict file outside the attempt directories only
// when it is in the registered panel directory of the slot gate. A gate with
// no registered panel accepts a file in any worktree of this repository.
func slotFileInPanel(s State, t Tuple, file string) error {
	refuse := func() error {
		message := "verdict must be the slot attempt verdict or a file in its registered panel directory"
		if path, err := slotVerdictPath(s, t); err == nil {
			message += ": " + termsafe.Escape(path)
		}
		return termsafe.NewFailure(message, slotRecovery(s, t))
	}
	roots, err := repositoryRoots(s.Root)
	if err != nil {
		return err
	}
	_, registered, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration)
	if err != nil {
		return err
	}
	for _, root := range roots {
		top, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(top, file)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if !registered || registeredPanelArtifact(s, t.Gate, filepath.ToSlash(rel)) {
			return nil
		}
	}
	return refuse()
}

// inSlotAttempt reports whether file is directly in an attempt directory of
// the slot directory of this step.
func inSlotAttempt(s State, t Tuple, file string) bool {
	reg, has, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration)
	if err != nil || !has {
		return false
	}
	iso, err := config.IsolationRootFor(s.Root)
	if err != nil {
		return false
	}
	layout, err := panel.NewSlotLayout(s.Root, iso, s.SpecID, reg.Slug, t.Round, fmt.Sprintf("R%d", t.Slot))
	if err != nil {
		return false
	}
	dir, err := filepath.EvalSymlinks(layout.Dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(dir, file)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 2 || parts[1] == "" || parts[1] == "." || parts[1] == ".." {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(parts[0], "attempt-"))
	return err == nil && n > 0 && parts[0] == "attempt-"+strconv.Itoa(n)
}

// SlotVerdictPath is the registered verdict path of a slot in the active gate
// round: R<n>-round-<k>.json in the registered panel directory.
func SlotVerdictPath(s State, step string) (string, error) {
	t, has, err := s.Records[step].Tuple()
	if err != nil || !has || t.Kind != "review-slot" {
		return "", actorFailure("step is not a review slot")
	}
	return slotVerdictPath(s, t)
}

func slotVerdictPath(s State, t Tuple) (string, error) {
	reg, has, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration)
	if err != nil {
		return "", err
	}
	if !has {
		return "", actorFailure("slot gate has no registered panel")
	}
	dirs := registrationPaths(s, reg)
	if len(dirs) == 0 {
		return "", actorFailure("slot gate has no registered panel directory")
	}
	dir := dirs[0]
	for _, candidate := range dirs {
		if strings.HasSuffix(candidate, "/"+reg.Slug+"/") {
			dir = candidate
			break
		}
	}
	return fmt.Sprintf("%sR%d-round-%d.json", dir, t.Slot, t.Round), nil
}

// ReceiptBytes returns the record bytes that a receipt names. The step
// metadata holds them as mindspec_record, and the receipt hash must match
// them. No reader takes the bytes from git.
func ReceiptBytes(r Record, receipt Receipt) ([]byte, error) {
	data, err := recordBytes(r)
	if err != nil {
		return nil, err
	}
	if receipt.RecordSHA256 == "" || receipt.RecordSHA256 != recordDigest(data) {
		return nil, actorFailure("receipt record_sha256 does not match mindspec_record")
	}
	return data, nil
}

// stderrLine joins the non-empty lines of git stderr with "; ", so a refusal
// shows the git cause on one readable line.
func stderrLine(stderr []byte) string {
	var parts []string
	for _, line := range strings.FieldsFunc(string(stderr), func(r rune) bool { return r == '\n' || r == '\r' }) {
		if line = strings.TrimSpace(line); line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "; ")
}

// SlotPromptRecovery names the command that prints the verdict path and the
// submit command of a slot step.
func SlotPromptRecovery(s State, step string) string {
	t, has, err := s.Records[step].Tuple()
	if err != nil || !has || t.Kind != "review-slot" {
		return "mindspec next-action"
	}
	return slotRecovery(s, t)
}

// slotRecovery names the command that prints the verdict path and submit
// command of a slot.
func slotRecovery(s State, t Tuple) string {
	slug := "<slug>"
	if reg, has, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration); err == nil && has && reg.Slug != "" {
		slug = briefQuote(reg.Slug)
	}
	return fmt.Sprintf("mindspec panel prompt %s --slot R%d --spec %s, then write the verdict to the path it names and run the submit command it prints", slug, t.Slot, briefQuote(idrender.Spec(s.SpecID)))
}

// SlotRecords returns the stored verdict of each current slot step of the
// panel p, keyed by its verdict file name. The receipt hash must match the
// record, or the step supplies nothing. A fresh clone thus tallies the votes
// that the bd store holds, with no verdict file on disk. A verdict file that
// differs from its record does not count, so its slot gets a retry.
func SlotRecords(s State, p panel.Registration) map[string][]byte {
	out := map[string][]byte{}
	for key, r := range s.Records {
		t, has, err := r.Tuple()
		if err != nil || !has || t.Kind != "review-slot" || r.Status != "closed" {
			continue
		}
		reg, registered, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration)
		if err != nil || !registered || reg.Slug != p.Slug() {
			continue
		}
		receipt, _, err := Decode[Receipt](r.Metadata, KeyClose)
		if err != nil || receipt.Kind == "superseded" {
			continue
		}
		root, err := SlotLineageRoot(s, key)
		if err != nil {
			continue
		}
		if head, err := SlotLineageHead(s, root); err != nil || head.Key() != key {
			continue
		}
		data, err := ReceiptBytes(r, receipt)
		if err != nil {
			continue
		}
		out[fmt.Sprintf("R%d-round-%d.json", t.Slot, t.Round)] = data
	}
	return out
}

// TallyPanel tallies the panel p from the stored slot verdicts. A verdict
// file with no record counts as before; one that differs from its record
// does not count.
func TallyPanel(s State, p panel.Registration) (*panel.Result, error) {
	return panel.TallyRecords(p.Dir, SlotRecords(s, p))
}

// PanelRecords returns the stored slot verdicts of the panel directory dir.
// A nil state or a spec that is not enrolled gives nil, so the caller reads
// the disk files only.
func PanelRecords(s *State, dir string) map[string][]byte {
	if s == nil || !s.Enrolled {
		return nil
	}
	return SlotRecords(*s, panel.Registration{Dir: dir})
}

// PanelRecordSource returns the record source of s for every panel directory.
// A nil state or a spec that is not enrolled gives nil.
func PanelRecordSource(s *State) panel.RecordSource {
	if s == nil || !s.Enrolled {
		return nil
	}
	return func(dir string) (map[string][]byte, error) { return PanelRecords(s, dir), nil }
}

// PanelTally is the one tally reader for a panel directory. On an enrolled
// spec it counts the stored slot verdicts, as TallyPanel does. Otherwise it
// is panel.Tally.
func PanelTally(s *State, dir string) (*panel.Result, error) {
	return panel.TallyRecords(dir, PanelRecords(s, dir))
}

// PanelFindingVerdict reads the counted submission of slot in round with the
// same records as PanelTally.
func PanelFindingVerdict(s *State, dir, slot string, round int) (panel.FindingVerdict, error) {
	return panel.ReadSubmittedFindingVerdictRecords(dir, slot, round, PanelRecords(s, dir))
}
