package kernel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// CloseWriteHook builds the writes of a validated record before any close write.
// The returned writes are size-checked together with the stored bytes and receipt.
type CloseWriteHook func(State, Record, Receipt) ([]Write, error)

var closeWriteHooks = map[string]CloseWriteHook{}

func RegisterCloseWriteHook(kind string, hook CloseWriteHook) error {
	if !recordKind(kind) || hook == nil {
		return fmt.Errorf("invalid close-write hook")
	}
	if _, exists := closeWriteHooks[kind]; exists {
		return fmt.Errorf("duplicate close-write hook %q", kind)
	}
	closeWriteHooks[kind] = hook
	return nil
}

// ReplaceCloseWriteHookForTest installs a hook and returns its restoration function.
// Tests that use it must not run in parallel.
func ReplaceCloseWriteHookForTest(kind string, hook CloseWriteHook) func() {
	old, exists := closeWriteHooks[kind]
	closeWriteHooks[kind] = hook
	return func() {
		if exists {
			closeWriteHooks[kind] = old
		} else {
			delete(closeWriteHooks, kind)
		}
	}
}

func strictRecord[T any](data []byte) (T, error) {
	var value T
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, pathFailure(err, "invalid record: ", "mindspec next-action")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return value, actorFailure("invalid record: expected one JSON value")
	}
	return value, nil
}

func readRecordFile(s State, step, name string) (string, error) {
	checked, err := checkSuppliedPath(s.Root, name, recordPathRecovery)
	if err != nil {
		return "", recordPathFailure(err)
	}
	info := checked.info
	if !info.Mode().IsRegular() {
		return "", recordPathFailure(fmt.Errorf("record is not a regular file"))
	}
	roots, err := repositoryRoots(s.Root)
	if err != nil {
		return "", recordPathFailure(err)
	}
	for _, root := range roots {
		for _, path := range []string{checked.clean, checked.resolved} {
			inside, err := insidePath(root, path)
			if err != nil {
				return "", recordPathFailure(err)
			}
			if inside {
				return "", recordPathFailure(fmt.Errorf("record path must be outside every worktree of this repository"))
			}
		}
	}
	if info.Size() > MetadataLimit {
		return "", recordSizeFailure(step, info.Size())
	}
	return readCheckedRecord(step, checked.resolved, info)
}

func readCheckedRecord(step, name string, checked os.FileInfo) (string, error) {
	f, err := openRecord(name)
	if err != nil {
		return "", recordPathFailure(fmt.Errorf("record is unreadable: %w", err))
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return "", recordPathFailure(err)
	}
	if !opened.Mode().IsRegular() {
		return "", recordPathFailure(fmt.Errorf("opened record is not a regular file"))
	}
	if !os.SameFile(checked, opened) {
		return "", termsafe.NewFailure("record changed after its path was checked", "rerun the submit with a record file that is not being changed")
	}
	return readRecordBytes(step, f)
}

func recordPathFailure(err error) error {
	return pathFailure(err, "", recordPathRecovery)
}

const recordPathRecovery = "give --record an existing readable regular file outside every worktree of this repository without .. segments"

func recordSizeFailure(step string, size int64) error {
	tuple, _ := kernelid.Parse(step)
	return metadataSizeFailure(tuple.Gate, step, KeyRecord, size, "record size")
}

// readRecordBytes also bounds a file that grows after the size check.
func readRecordBytes(step string, reader io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MetadataLimit+1))
	if err != nil {
		return "", recordPathFailure(err)
	}
	if len(data) > MetadataLimit {
		return "", recordSizeFailure(step, int64(len(data)))
	}
	if !utf8.Valid(data) {
		return "", actorFailure("record must be valid UTF-8")
	}
	return string(data), nil
}

func recordBytes(r Record) ([]byte, error) {
	text, present, err := Decode[string](r.Metadata, KeyRecord)
	if err != nil || !present {
		return nil, actorFailure("mindspec_record is absent or invalid")
	}
	if !utf8.ValidString(text) {
		return nil, actorFailure("mindspec_record must be valid UTF-8")
	}
	return []byte(text), nil
}

func checkLens(s State, r Record, _ Receipt) error {
	data, err := recordBytes(r)
	if err != nil {
		return err
	}
	lens, err := strictRecord[LensRecord](data)
	if err != nil {
		return err
	}
	if lens.Step != r.Key() || lens.Lenses == nil {
		return actorFailure("lens record needs its step and lenses")
	}
	round, err := activeActorRound(s, r)
	if err != nil {
		return err
	}
	t, _, _ := r.Tuple()
	gateKind := "bead"
	if t.WorkItem == "" {
		gateKind = "final_review"
	}
	cfg, err := config.Load(s.Root)
	if err != nil {
		return err
	}
	roster, err := cfg.PanelGateReviewerSlots(gateKind, round.SlotCount)
	if err != nil {
		return err
	}
	expected := map[string]bool{}
	for i, slot := range roster {
		if slot.Lens != "adversarial" {
			expected[fmt.Sprintf("R%d", i+1)] = true
		}
	}
	seen := map[string]bool{}
	for _, entry := range lens.Lenses {
		if !expected[entry.Slot] || strings.TrimSpace(entry.Lens) == "" || seen[entry.Slot] {
			return actorFailure("lens record has an extra, empty or repeated slot lens: " + entry.Slot)
		}
		seen[entry.Slot] = true
	}
	for slot := range expected {
		if !seen[slot] {
			return actorFailure("lens record is missing slot " + slot)
		}
	}
	return nil
}

func checkVerify(s State, r Record, _ Receipt) error {
	data, err := recordBytes(r)
	if err != nil {
		return err
	}
	v, err := strictRecord[VerifyRecord](data)
	if err != nil {
		return err
	}
	subject, _, err := Decode[Subject](r.Metadata, KeySubject)
	if err != nil {
		return err
	}
	if v.Step != r.Key() || v.Subject != subject.Commit || len(v.Returns) == 0 {
		return actorFailure("verify record needs its step, subject and returns")
	}
	if _, err := activeActorRound(s, r); err != nil {
		return err
	}
	findings, present, err := Decode[[]Finding](r.Metadata, KeyFindings)
	if err != nil {
		return err
	}
	if !present || len(findings) == 0 {
		return actorFailure("verify step has no mindspec_findings")
	}
	identity := func(slot string, round int, finding, sha string) string {
		return fmt.Sprintf("%s/%d/%s/%s", slot, round, finding, sha)
	}
	expected := map[string]Finding{}
	for _, f := range findings {
		expected[identity(f.Slot, f.Round, f.Finding, f.FindingSHA256)] = f
	}
	seen := map[string]bool{}
	for _, ret := range v.Returns {
		if ret.Slot == "" || ret.Round < 1 || ret.Finding == "" || ret.FindingSHA256 == "" {
			return actorFailure("verify return needs a finding identity")
		}
		key := identity(ret.Slot, ret.Round, ret.Finding, ret.FindingSHA256)
		finding, ok := expected[key]
		if !ok || seen[key] {
			return actorFailure("verify record has an unknown or repeated finding: " + key)
		}
		seen[key] = true
		switch ret.Return {
		case "reproduced":
		case "refuted":
			if ret.Ground != "disproven" && ret.Ground != "not-load-bearing" && ret.Ground != "out_of_acceptance_scope" {
				return actorFailure("refuted finding needs a valid ground")
			}
			if ret.Ground == "out_of_acceptance_scope" && strings.TrimSpace(ret.AC) == "" {
				return actorFailure("out_of_acceptance_scope needs ac")
			}
			if strings.TrimSpace(ret.Reason) == "" || !recordEvidenceResolves(s.Root, ret.Evidence) {
				return actorFailure("refuted finding needs reason and resolving evidence")
			}
		case "deferred":
			if ret.Finding == "veto" {
				return actorFailure("a veto cannot be deferred")
			}
			if strings.TrimSpace(ret.Reason) == "" || ret.Target == "" {
				return actorFailure("deferred finding needs reason and target")
			}
			t, _, _ := r.Tuple()
			if err := bead.CheckDeferral(ret.Target, s.Epic, t.WorkItem, []string{finding.Text}, func(text string) string { return text }); err != nil {
				var requirements *bead.DeferralRequirementsError
				if errors.As(err, &requirements) {
					// Escape the finding before preserving the list's own line breaks.
					message := strings.ReplaceAll(requirements.Error(), finding.Text, termsafe.EscapeRecovery(finding.Text))
					lines := strings.Split(message, "\n")
					for i := range lines {
						lines[i] = termsafe.EscapeRecovery(lines[i])
					}
					return &preservedRecovery{cause: err, text: strings.Join(lines, "\n") + "\nrecovery: " + termsafe.EscapeRecovery(requirements.Recovery)}
				}
				return pathFailure(err, "", "mindspec next-action")
			}
		default:
			return actorFailure("verify return must be reproduced, refuted or deferred")
		}
	}
	for key := range expected {
		if !seen[key] {
			return actorFailure("verify record is missing finding: " + key)
		}
	}
	return nil
}

func recordEvidenceResolves(root, evidence string) bool {
	if u, err := url.Parse(evidence); err == nil && u.Scheme == "https" && u.Host != "" {
		return true
	}
	if evidence == "" {
		return false
	}
	if checked, err := checkSuppliedPath(root, evidence, "give an existing readable evidence path without .. segments"); err == nil {
		if inside, err := insidePath(root, checked.resolved); err == nil && inside {
			return true
		}
	}
	if !objectID(evidence) {
		return false
	}
	_, err := actorGit(root, "rev-parse", "--verify", evidence+"^{commit}")
	return err == nil
}

// recordCloseWrites runs only for an attempt that has no valid receipt.
func recordCloseWrites(s State, r Record, receipt Receipt) (Receipt, error) {
	t, _, _ := r.Tuple()
	if t.Kind == "verify" {
		pin, has, err := Decode[Closing](r.Metadata, KeyClosing)
		if err != nil {
			return receipt, err
		}
		if has && pin.Conflict != nil {
			return receipt, actorFailure("mindspec_closing has a conflict")
		}
		if has {
			receipt.At = pin.At.UTC().Format(timeLayout)
		} else {
			receipt.At = s.At.UTC().Format(timeLayout)
		}
	}
	writes := []Write{{Kind: "metadata", ID: r.Key(), Metadata: Metadata(KeyRecord, r.Metadata[KeyRecord])}}
	if hook := closeWriteHooks[t.Kind]; hook != nil {
		more, err := hook(s, r, receipt)
		if err != nil {
			return receipt, err
		}
		writes = append(writes, more...)
	}
	// Project all writes first so an oversized later write cannot leave a pin.
	projected := s.Clone()
	for _, w := range writes {
		if w.Kind != "metadata" {
			return receipt, actorFailure("record close hook must return metadata writes")
		}
		if err := overlay(&projected, w); err != nil {
			return receipt, err
		}
		if _, err := CheckMetadataSize(w.ID, projected.Records[w.ID].Metadata); err != nil {
			return receipt, err
		}
	}
	final := mergedMetadata(projected.Records[r.Key()].Metadata, Metadata(KeyClose, receipt))
	if err := ValidReceipt(s.Root, receipt, final); err != nil {
		return receipt, err
	}
	if t.Kind == "verify" {
		pin, has, err := Decode[Closing](r.Metadata, KeyClosing)
		if err != nil {
			return receipt, err
		}
		if !has {
			pin = Closing{Generation: receipt.Generation, RecordSHA256: receipt.RecordSHA256, At: s.At.UTC()}
		}
		final[KeyClosing] = pin
		if _, err := CheckMetadataSize(r.Key(), final); err != nil {
			return receipt, err
		}
		if has && (pin.Generation != receipt.Generation || pin.RecordSHA256 != receipt.RecordSHA256) {
			pin.Conflict = &ClosingIdentity{Generation: receipt.Generation, RecordSHA256: receipt.RecordSHA256}
			if err := s.Store.WriteMetadata(r.Key(), Metadata(KeyClosing, pin)); err != nil {
				return receipt, err
			}
			return receipt, actorFailure("mindspec_closing differs from this submission")
		}
		if !has {
			if err := s.Store.WriteMetadata(r.Key(), Metadata(KeyClosing, pin)); err != nil {
				return receipt, err
			}
		}
	} else if _, err := CheckMetadataSize(r.Key(), final); err != nil {
		return receipt, err
	}
	for _, w := range writes {
		if err := applyWrite(s.Store, w); err != nil {
			return receipt, err
		}
	}
	return receipt, nil
}

const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"

func checkClosingConflict(s State, r Record) error {
	t, _, _ := r.Tuple()
	if t.Kind != "verify" {
		return nil
	}
	fresh, err := s.Store.Lookup(r.Key())
	if err != nil {
		return err
	}
	if fresh == nil {
		return actorFailure("mindspec_closing step is absent")
	}
	pin, _, err := Decode[Closing](fresh.Metadata, KeyClosing)
	if err != nil {
		return err
	}
	if pin.Conflict != nil {
		return actorFailure("mindspec_closing has a conflict")
	}
	return nil
}
