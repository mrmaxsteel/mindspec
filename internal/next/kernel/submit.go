package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

type SubmitOptions struct {
	Step     string
	Lease    string
	Artifact string
	Record   string
	Commit   string
	Answer   string
	// BeforeClose publishes validated evidence before the receipt and final reduction.
	BeforeClose func(State, Receipt) error
}
type ArtifactCheck func(State, Record, Receipt) error

var artifactChecks = map[string]ArtifactCheck{}

func RegisterArtifactCheck(kind string, check ArtifactCheck) error {
	switch kind {
	case "implement", "review-slot", "fix", "change-judge", "lens", "verify":
	default:
		return fmt.Errorf("unknown artifact kind %q", kind)
	}
	if _, ok := artifactChecks[kind]; ok {
		return fmt.Errorf("duplicate artifact check %q", kind)
	}
	if check == nil {
		return fmt.Errorf("nil artifact check")
	}
	artifactChecks[kind] = check
	return nil
}

func init() {
	for kind, check := range map[string]ArtifactCheck{"implement": checkImplement, "fix": checkFix, "review-slot": checkVerdict, "lens": checkLens, "verify": checkVerify} {
		if err := RegisterArtifactCheck(kind, check); err != nil {
			panic(err)
		}
	}
}

func Submit(state State, options SubmitOptions) (Output, error) {
	if !state.Enrolled {
		return Output{}, ErrPreKernel
	}
	if options.Answer != "" {
		if options.Record != "" || options.Artifact != "" || options.Commit != "" {
			return Output{}, actorFailure("submit needs one evidence flag")
		}
		return Answer(state, options.Step, options.Answer)
	}
	var output Output
	err := withActorLock(state, func(s State) error {
		var record string
		if options.Record != "" {
			recordStep := options.Step
			if r, _, err := actorRecord(s, recordStep, false); err == nil {
				recordStep = r.Key()
			}
			var err error
			record, err = readRecordFile(s, recordStep, options.Record)
			if err != nil {
				return err
			}
		}
		if err := refuseHalt(s); err != nil {
			return err
		}
		if _, err := passExcept(Rules(), s, true, options.Step); err != nil {
			return err
		}
		changed := false
		err := WithCASRetry(func() error {
			fresh, err := refreshActorState(s)
			if err != nil {
				return err
			}
			s = fresh
			if err = refuseHalt(s); err != nil {
				return err
			}
			r, tuple, err := actorRecord(s, options.Step, false)
			if err != nil {
				return err
			}
			lease, _, err := Decode[LeaseRecord](r.Metadata, KeyLease)
			if err != nil {
				return err
			}
			if !matchesToken(lease, options.Lease) {
				return actorFailure("lease token is not current")
			}
			evidenceCount := 0
			for _, value := range []string{options.Record, options.Artifact, options.Commit} {
				if value != "" {
					evidenceCount++
				}
			}
			if evidenceCount != 1 {
				return actorFailure("submit needs one record, artifact or commit")
			}
			if recordKind(tuple.Kind) != (options.Record != "") || (tuple.Kind == "implement" || tuple.Kind == "fix") != (options.Commit != "") {
				return actorFailure("artifact does not fit the step kind")
			}
			var receipt Receipt
			if recordKind(tuple.Kind) {
				receipt = Receipt{Kind: "passed", RecordSHA256: recordDigest([]byte(record))}
			} else {
				receipt, err = resolveArtifact(s, options)
				if err != nil {
					return err
				}
			}
			receipt.Generation = lease.Generation
			check, ok := artifactChecks[tuple.Kind]
			if !ok {
				return actorFailure("artifact check is not available for " + tuple.Kind)
			}
			old, present, err := Decode[Receipt](r.Metadata, KeyClose)
			if err != nil {
				return err
			}
			if present && ValidReceipt(s.Root, old, r.Metadata) == nil {
				receipt.Repaired = old.Repaired
				receipt.At = old.At
				if !reflect.DeepEqual(old, receipt) {
					if old.Kind == "superseded" {
						return actorFailure("step was superseded by a new subject or round; use the current actions")
					}
					return actorFailure("step already has a different result")
				}
				// A closed receipt is immutable, even after the gate advances or TTL expires.
				if r.Status == "closed" {
					return nil
				}
			} else {
				if !lease.Expires.After(s.At) {
					return actorFailure("lease has expired")
				}
				if recordKind(tuple.Kind) {
					r.Metadata = mergedMetadata(r.Metadata, Metadata(KeyRecord, record))
				}
				if err = check(s, r, receipt); err != nil {
					return err
				}
				if err = actorBlockers(s, r); err != nil {
					return err
				}
				if err = actorChildren(s, r); err != nil {
					return err
				}
				receipt.Repaired = r.Status == "closed"
				if recordKind(tuple.Kind) {
					receipt, err = recordCloseWrites(s, r, receipt)
					if err != nil {
						return err
					}
				}
				if options.BeforeClose != nil {
					if err = options.BeforeClose(s, receipt); err != nil {
						return err
					}
				}
				if err = checkClosingConflict(s, r); err != nil {
					return err
				}
				if err = s.Store.WriteReceipt(r.Key(), receipt); err != nil {
					return err
				}
				changed = true
			}
			if err = actorChildren(s, r); err != nil {
				return err
			}
			if r.Status != "closed" {
				if err = s.Store.SetStatusIf(r.Key(), r.Status, "closed"); err != nil {
					return err
				}
				changed = true
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !changed {
			result, err := Pass(Rules(), s, false)
			output = OutputForPass(result, s, false)
			return err
		}
		fresh, err := refreshActorState(s)
		if err != nil {
			return err
		}
		result, err := Pass(Rules(), fresh, true)
		if err != nil {
			return err
		}
		committed, err := refreshActorState(s)
		if err != nil {
			return err
		}
		committed.At = result.State.At
		output = OutputForPass(result, committed, true)
		return nil
	})
	return output, pathFailure(err, "", "mindspec next-action")
}

func actorOutput(s State) Output {
	status := "wait"
	if s.Halted() {
		status = "halted"
	}
	return Output{Schema: "mindspec.next-action/v2", Status: status, Applied: true,
		Policy:  PolicyView{RoundLimit: s.Policy.RoundLimit, LeaseTTL: s.Policy.LeaseTTL},
		Actions: []Action{}, Waiting: []Waiting{}, Decisions: []Decision{}, Run: []Run{}, Flags: []Flag{}}
}

func actorGit(root string, args ...string) (string, error) {
	data, err := actorGitBytes(root, args...)
	return strings.TrimSpace(string(data)), err
}
func actorGitBytes(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	data, err := cmd.Output()
	if err != nil {
		message := "cannot resolve committed evidence: "
		recovery := "restore access to the repository and evidence, then run mindspec next-action"
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "list" {
			message = "cannot list repository worktrees: "
			recovery = "restore access to the repository and repair its Git worktree registration"
		} else if len(args) == 2 && args[0] == "rev-parse" && args[1] == "--show-toplevel" {
			message = "cannot find repository worktree top: "
			recovery = "restore access to the repository and repair its Git worktree registration"
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			err = fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, exit.Stderr)
		} else {
			err = fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return nil, pathFailure(err, message, recovery)
	}
	return data, nil
}
func resolveArtifact(s State, o SubmitOptions) (Receipt, error) {
	receipt := Receipt{Kind: "passed"}
	evidenceRoot := s.Root
	if o.Commit != "" {
		if !objectID(o.Commit) {
			return receipt, actorFailure("submit needs a full commit object id")
		}
		commit, err := actorGit(s.Root, "rev-parse", "--verify", o.Commit+"^{commit}")
		if err != nil {
			return receipt, err
		}
		receipt.Commit = commit
		paths, err := actorGitBytes(s.Root, "diff-tree", "--root", "--no-commit-id", "--name-only", "--diff-filter=AM", "-r", "-z", commit)
		if err != nil {
			return receipt, err
		}
		for _, path := range strings.Split(string(paths), "\x00") {
			if path != "" {
				receipt.Path = path
				break
			}
		}
		if receipt.Path == "" {
			tree, e := actorGitBytes(s.Root, "ls-tree", "-r", "-z", commit)
			if e != nil {
				return receipt, e
			}
			for _, entry := range strings.Split(string(tree), "\x00") {
				fields := strings.SplitN(entry, "\t", 2)
				if len(fields) == 2 && strings.Contains(fields[0], " blob ") {
					receipt.Path = fields[1]
					break
				}
			}
		}
		if receipt.Path == "" {
			return receipt, actorFailure("commit needs a durable artifact blob")
		}
	} else {
		checked, err := checkSuppliedPath(s.Root, o.Artifact, artifactPathRecovery)
		if err != nil {
			return receipt, pathFailure(err, "artifact must be a regular committed file: ", artifactPathRecovery)
		}
		info, err := os.Lstat(checked.clean)
		if err != nil {
			return receipt, pathFailure(err, "cannot inspect artifact: ", artifactPathRecovery)
		}
		if !info.Mode().IsRegular() {
			return receipt, actorFailure("artifact must be a regular committed file")
		}
		path := checked.resolved
		evidenceRoot, err = actorGit(filepath.Dir(path), "rev-parse", "--show-toplevel")
		if err != nil {
			return receipt, pathFailure(err, "artifact must be a committed file in a worktree of this repository: ", artifactPathRecovery)
		}
		common, err := actorGit(s.Root, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			return receipt, pathFailure(err, "cannot inspect repository Git directory: ", artifactPathRecovery)
		}
		evidenceCommon, err := actorGit(evidenceRoot, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			return receipt, err
		}
		commonInfo, err := os.Stat(common)
		if err != nil {
			return receipt, pathFailure(err, "cannot inspect repository Git directory: ", artifactPathRecovery)
		}
		evidenceInfo, err := os.Stat(evidenceCommon)
		if err != nil {
			return receipt, pathFailure(err, "cannot inspect artifact Git directory: ", artifactPathRecovery)
		}
		if !os.SameFile(commonInfo, evidenceInfo) {
			return receipt, actorFailure("artifact must belong to this repository")
		}
		path, err = filepath.Rel(evidenceRoot, path)
		if err != nil {
			return receipt, pathFailure(err, "cannot locate artifact in its worktree: ", artifactPathRecovery)
		}
		if path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			return receipt, actorFailure("artifact must be inside its worktree")
		}
		path = filepath.ToSlash(path)
		receipt.Path = path
		receipt.Commit, err = actorGit(evidenceRoot, "log", "-1", "--format=%H", "HEAD", "--", path)
		if err != nil {
			return receipt, pathFailure(err, "artifact has no holding commit: ", artifactPathRecovery)
		}
		if receipt.Commit == "" {
			return receipt, actorFailure("artifact has no holding commit")
		}
	}
	var err error
	receipt.Blob, err = actorGit(s.Root, "rev-parse", "--verify", receipt.Commit+":"+receipt.Path)
	if err != nil {
		return receipt, err
	}
	if err = ValidReceipt(s.Root, receipt, nil); err != nil {
		return receipt, pathFailure(err, "", artifactPathRecovery)
	}
	if o.Artifact != "" {
		actual, err := actorGit(evidenceRoot, "hash-object", "--", receipt.Path)
		if err != nil {
			return receipt, pathFailure(err, "cannot read artifact blob: ", artifactPathRecovery)
		}
		if actual != receipt.Blob {
			return receipt, actorFailure("artifact blob differs from its holding commit")
		}
	}
	return receipt, nil
}

const artifactPathRecovery = "give --artifact an existing committed file without .. segments"

func commitPast(s State, receipt Receipt, base, branch string) error {
	if !objectID(base) || receipt.Commit == base {
		return actorFailure("commit is not past the claim base or round subject")
	}
	if _, err := actorGit(s.Root, "merge-base", "--is-ancestor", base, receipt.Commit); err != nil {
		return actorFailure("commit is not past the claim base or round subject")
	}
	if _, err := actorGit(s.Root, "merge-base", "--is-ancestor", receipt.Commit, "refs/heads/"+branch); err != nil {
		return actorFailure("commit must be on " + branch)
	}
	return nil
}
func checkImplement(s State, r Record, receipt Receipt) error {
	tuple, _, _ := r.Tuple()
	lease, _, err := Decode[LeaseRecord](r.Metadata, KeyLease)
	if err != nil {
		return err
	}
	branch, err := workspace.BeadBranch(tuple.WorkItem)
	if err != nil {
		return err
	}
	return commitPast(s, receipt, lease.ClaimBase, branch)
}
func activeActorRound(s State, r Record) (GateRound, error) {
	t, _, _ := r.Tuple()
	gateID := activeGate(s, t.Gate)
	gate, ok := s.Records[gateID]
	if !ok {
		return GateRound{}, actorFailure("gate is absent")
	}
	round, exists, err := Decode[GateRound](gate.Metadata, KeyRound)
	if err != nil {
		return round, err
	}
	expectedRound := t.Round
	if t.Kind == "fix" {
		expectedRound--
	}
	if !exists || gate.Status == "closed" || round.Round != expectedRound {
		return round, actorFailure("step subject or round is stale")
	}
	subject, exists, err := Decode[Subject](r.Metadata, KeySubject)
	if err != nil {
		return round, err
	}
	if exists && subject.Commit != round.Subject {
		return round, actorFailure("step subject is stale")
	}
	return round, nil
}
func checkFix(s State, r Record, receipt Receipt) error {
	t, _, _ := r.Tuple()
	round, err := activeActorRound(s, r)
	if err != nil {
		return err
	}
	var branch string
	if t.WorkItem == "" {
		branch, err = workspace.SpecBranch(s.SpecID)
	} else {
		branch, err = workspace.BeadBranch(t.WorkItem)
	}
	if err != nil {
		return err
	}
	if err := commitPast(s, receipt, round.Subject, branch); err != nil {
		return err
	}
	paths, err := actorGitBytes(s.Root, "diff", "--no-renames", "--name-only", "-z", round.Subject, receipt.Commit, "--")
	if err != nil {
		return err
	}
	for _, name := range strings.Split(string(paths), "\x00") {
		if name != "" && !registeredPanelArtifact(s, t.Gate, name) {
			return nil
		}
	}
	return actorFailure("fix commit must change a path outside panel directories")
}
func checkVerdict(s State, r Record, receipt Receipt) error {
	t, _, _ := r.Tuple()
	round, err := activeActorRound(s, r)
	if err != nil {
		return err
	}
	bytes, err := actorGitBytes(s.Root, "show", receipt.Commit+":"+receipt.Path)
	if err != nil {
		return err
	}
	verdict, err := panel.ParseVerdictV2(bytes)
	if err != nil {
		return err
	}
	if verdict.Status != "final" || verdict.Slot != "R"+strconv.Itoa(t.Slot) || verdict.Round != round.Round || verdict.ReviewedSHA != round.Subject {
		return actorFailure("verdict slot, round or subject does not match the step")
	}
	root, err := SlotLineageRoot(s, r.Key())
	if err != nil {
		return err
	}
	listed := false
	for _, slot := range round.Slots {
		if slot == root {
			listed = true
		}
	}
	if !listed {
		return actorFailure("slot is not in the active gate round")
	}
	_, exists, err := Decode[RegistrationRecord](s.Records[activeGate(s, t.Gate)].Metadata, KeyRegistration)
	if err != nil {
		return err
	}
	if exists && !registeredPanelArtifact(s, t.Gate, receipt.Path) {
		return actorFailure("verdict must be in its registered panel directory")
	}
	return nil
}

func decisionOption(record DecisionRecord, option string) bool {
	for _, allowed := range record.Options {
		if allowed == option && option != "continue:<n>" {
			return true
		}
		if allowed == "continue:<n>" && strings.HasPrefix(option, "continue:") {
			n, err := strconv.Atoi(strings.TrimPrefix(option, "continue:"))
			if err == nil && n > 0 {
				return true
			}
		}
	}
	return false
}

// answerLocked writes the answer before a close, so a child cannot erase a halt.
func answerLocked(s State, decision, option string) (Output, error) {
	if haltErr := refuseHalt(s); haltErr != nil {
		r, _, err := actorRecord(s, decision, true)
		answer, answered := s.Answers()[r.Key()]
		if err != nil || !answered || option != "halt" || answer.Option != option {
			return Output{}, haltErr
		}
	} else if _, err := passExcept(Rules(), s, true, decision); err != nil {
		return Output{}, err
	}
	changed := false
	err := WithCASRetry(func() error {
		fresh, err := refreshActorState(s)
		if err != nil {
			return err
		}
		s = fresh
		r, _, err := actorRecord(s, decision, true)
		if err != nil {
			return err
		}
		old, present, err := Decode[Receipt](r.Metadata, KeyClose)
		if err != nil {
			return err
		}
		same := present && old.Answer != nil && old.Answer.Option == option && ValidReceipt(s.Root, old, r.Metadata) == nil
		if err = refuseHalt(s); err != nil && !(same && option == "halt") {
			return err
		}
		if present && ValidReceipt(s.Root, old, r.Metadata) == nil {
			if !same {
				return actorFailure("decision already has a different answer")
			}
			if r.Status == "closed" {
				return nil
			}
		} else {
			if r.Status == "closed" {
				return actorFailure("decision is closed without an answer")
			}
			d, exists, e := Decode[DecisionRecord](r.Metadata, KeyDecision)
			if e != nil {
				return e
			}
			if !exists || !decisionOption(d, option) {
				return actorFailure("answer must be a listed decision option")
			}
			answer := DecisionAnswer{Option: option, At: s.At}
			if err = s.Store.WriteMetadata(r.Key(), map[string]any{KeyAnswer: answer, KeyClose: Receipt{Kind: "passed", Answer: &answer}}); err != nil {
				return err
			}
			changed = true
		}
		if err = actorChildren(s, r); err != nil {
			// An answer is durable even when a foreign child keeps the decision open.
			var child *openActorChild
			if errors.As(err, &child) {
				return nil
			}
			return err
		}
		if option == "halt" {
			receipt := Receipt{Kind: "halted", HaltAnswer: r.Key(), Answer: old.Answer}
			if receipt.Answer == nil {
				answer := DecisionAnswer{Option: option, At: s.At}
				receipt.Answer = &answer
			}
			if err = s.Store.WriteReceipt(r.Key(), receipt); err != nil {
				return err
			}
		}
		if err = s.Store.SetStatusIf(r.Key(), r.Status, "closed"); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return Output{}, err
	}
	if !changed && option != "halt" {
		return actorOutput(s), nil
	}
	fresh, err := refreshActorState(s)
	if err != nil {
		return Output{}, err
	}
	result, err := Pass(Rules(), fresh, true)
	return result.Output, err
}
