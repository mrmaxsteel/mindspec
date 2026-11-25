package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const slotPublishRecovery = "restore access to the repository, then run mindspec next-action"

// SlotVerdictRef is the ref namespace that keeps the commits of published slot verdicts.
const SlotVerdictRef = "refs/mindspec/verdicts/"

// slotEvidence selects the evidence of a slot submit. The verdict file that the
// slot prompt names is in the slot's attempt directory, which no commit can
// hold. The binary publishes its bytes at the registered verdict path. Any
// other file keeps the committed-file rules.
func slotEvidence(s State, r Record, t Tuple, o SubmitOptions) (SubmitOptions, error) {
	checked, err := checkSuppliedPath(s.Root, o.Artifact, artifactPathRecovery)
	if err != nil {
		return o, pathFailure(err, "artifact must be a regular committed file: ", artifactPathRecovery)
	}
	if !checked.info.Mode().IsRegular() || !inSlotAttempt(s, t, checked.resolved) {
		if _, err := resolveArtifact(s, o); err != nil {
			var failure *termsafe.Failure
			if errors.As(err, &failure) {
				return o, termsafe.NewWrappedFailure(err, failure.Message(), slotRecovery(s, t))
			}
			return o, err
		}
		return o, nil
	}
	commit, err := publishSlotVerdict(s, r, t, checked.resolved)
	if err != nil {
		return o, err
	}
	o.Artifact, o.Commit = "", commit
	return o, nil
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

// publishSlotVerdict writes the verdict bytes at the registered verdict path on
// a commit whose parent is the step subject. A ref per step and blob keeps the
// commit, so a retry with the same bytes gets the same commit.
func publishSlotVerdict(s State, r Record, t Tuple, file string) (string, error) {
	path, err := slotVerdictPath(s, t)
	if err != nil {
		return "", err
	}
	subject, has, err := Decode[Subject](r.Metadata, KeySubject)
	if err != nil {
		return "", err
	}
	if !has || !objectID(subject.Commit) {
		return "", actorFailure("slot step has no subject")
	}
	f, err := os.Open(file)
	if err != nil {
		return "", pathFailure(err, "cannot read verdict file: ", artifactPathRecovery)
	}
	data, err := io.ReadAll(io.LimitReader(f, panel.MaxVerdictBytes+1))
	f.Close()
	if err != nil {
		return "", pathFailure(err, "cannot read verdict file: ", artifactPathRecovery)
	}
	if len(data) > panel.MaxVerdictBytes {
		return "", actorFailure("verdict file is too large")
	}
	blob, err := publishGit(s.Root, nil, data, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	if _, err := kernelid.Parse(r.Key()); err != nil {
		return "", actorFailure("slot step id is not a kernel step id")
	}
	ref := SlotVerdictRef + r.Key() + "/" + blob
	if _, err := publishGit(s.Root, nil, nil, "check-ref-format", ref); err != nil {
		return "", actorFailure("slot step id is not a valid ref name")
	}
	if commit, ok := publishedVerdict(s.Root, ref, path, blob, subject.Commit); ok {
		return commit, nil
	}
	tmp, err := os.MkdirTemp("", "mindspec-verdict-")
	if err != nil {
		return "", pathFailure(err, "cannot publish slot verdict: ", slotPublishRecovery)
	}
	defer os.RemoveAll(tmp)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(tmp, "index")}
	if _, err = publishGit(s.Root, env, nil, "read-tree", subject.Commit); err != nil {
		return "", err
	}
	if _, err = publishGit(s.Root, env, nil, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+path); err != nil {
		return "", err
	}
	tree, err := publishGit(s.Root, env, nil, "write-tree")
	if err != nil {
		return "", err
	}
	identity := []string{"GIT_AUTHOR_NAME=mindspec", "GIT_AUTHOR_EMAIL=mindspec@localhost", "GIT_COMMITTER_NAME=mindspec", "GIT_COMMITTER_EMAIL=mindspec@localhost"}
	commit, err := publishGit(s.Root, identity, nil, "commit-tree", tree, "-p", subject.Commit, "-m", "Record slot verdict "+r.Key())
	if err != nil {
		return "", err
	}
	if _, err = publishGit(s.Root, nil, nil, "update-ref", ref, commit, ""); err != nil {
		// Another writer can make the ref first. Its commit holds the same bytes.
		if existing, ok := publishedVerdict(s.Root, ref, path, blob, subject.Commit); ok {
			return existing, nil
		}
		return "", err
	}
	return commit, nil
}

// publishedVerdict returns the commit of ref when it holds blob at path on the subject.
func publishedVerdict(root, ref, path, blob, subject string) (string, bool) {
	commit, err := publishGit(root, nil, nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil || !objectID(commit) {
		return "", false
	}
	held, err := publishGit(root, nil, nil, "rev-parse", "--verify", "--quiet", commit+":"+path)
	if err != nil || held != blob {
		return "", false
	}
	parent, err := publishGit(root, nil, nil, "rev-parse", "--verify", "--quiet", commit+"^1")
	if err != nil || parent != subject {
		return "", false
	}
	return commit, true
}

func publishGit(root string, env []string, stdin []byte, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			err = fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(exit.Stderr)))
		} else {
			err = fmt.Errorf("git %s: %w", args[0], err)
		}
		return "", pathFailure(err, "cannot publish slot verdict: ", slotPublishRecovery)
	}
	return strings.TrimSpace(string(out)), nil
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
