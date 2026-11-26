package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

type SubmitOptions struct {
	Step     string
	Lease    string
	Artifact string
	Record   string
	Commit   string
	Answer   string
	Repair   bool
	Reason   string
	// BeforeClose publishes validated evidence before the receipt and final reduction.
	// A slot verdict from an attempt directory has no holding commit yet when
	// BeforeClose runs; ReceiptBytes gives its bytes.
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
	if (options.Repair && (strings.TrimSpace(options.Reason) == "" || options.Lease != "" || options.Answer != "")) || (!options.Repair && options.Reason != "") {
		return Output{}, actorFailure("submit --repair requires --reason and excludes --lease and --answer")
	}
	if options.Answer != "" {
		if options.Record != "" || options.Artifact != "" || options.Commit != "" {
			return Output{}, actorFailure("submit needs one evidence flag")
		}
		output, err := Answer(state, options.Step, options.Answer)
		return output, pathFailure(err, "", "mindspec next-action")
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
			old, present, err := Decode[Receipt](r.Metadata, KeyClose)
			if err != nil {
				return err
			}
			valid := present && ValidReceipt(s.Root, old, r.Metadata) == nil
			generation := lease.Generation
			if options.Repair {
				if err = repairable(r, lease, valid); err != nil {
					return err
				}
				if recordKind(tuple.Kind) {
					stored, err := recordBytes(r)
					if err != nil {
						return err
					}
					if options.Record != "" && recordDigest([]byte(record)) != recordDigest(stored) {
						return actorFailure("repair --record differs from mindspec_record")
					}
					record = string(stored)
					pin, has, err := Decode[Closing](r.Metadata, KeyClosing)
					if err != nil {
						return err
					}
					if has && pin.RecordSHA256 != recordDigest(stored) {
						return actorFailure("repair record differs from mindspec_closing")
					}
				}
			} else {
				generation, err = submitGeneration(r, lease, options.Lease, old, valid)
				if err != nil {
					return err
				}
			}
			evidenceCount := 0
			for _, value := range []string{options.Record, options.Artifact, options.Commit} {
				if value != "" {
					evidenceCount++
				}
			}
			storedRepair := options.Repair && recordKind(tuple.Kind) && evidenceCount == 0
			if evidenceCount != 1 && !(storedRepair && evidenceCount == 0) {
				return actorFailure("submit needs one record, artifact or commit")
			}
			if recordKind(tuple.Kind) != (options.Record != "" || storedRepair) || (tuple.Kind == "implement" || tuple.Kind == "fix") != (options.Commit != "") {
				return actorFailure("artifact does not fit the step kind")
			}
			var receipt Receipt
			var pending *slotPending
			if recordKind(tuple.Kind) {
				receipt = Receipt{Kind: "passed", RecordSHA256: recordDigest([]byte(record))}
			} else {
				if tuple.Kind == "review-slot" && options.Artifact != "" {
					if pending, err = slotEvidence(s, r, tuple, options); err != nil {
						return err
					}
				}
				if pending != nil {
					receipt = pending.receipt()
					if valid && old.Path == receipt.Path && old.Blob == receipt.Blob {
						// The same bytes are already published and closed.
						receipt.Commit = old.Commit
					}
				} else if receipt, err = resolveArtifact(s, options); err != nil {
					return err
				}
			}
			receipt.Generation = generation
			if generation != lease.Generation {
				receipt.SupersededGeneration = lease.Generation
			}
			check, ok := artifactChecks[tuple.Kind]
			if !ok {
				return actorFailure("artifact check is not available for " + tuple.Kind)
			}
			if valid {
				receipt.Repaired = old.Repaired
				receipt.At = old.At
				receipt.Via = old.Via
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
				if !options.Repair && generation == lease.Generation && !lease.Expires.After(s.At) {
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
				if options.Repair {
					receipt.Via = "repair"
				}
				updates := map[string]any{}
				if options.Repair {
					history, err := takeHistory(r)
					if err != nil {
						return err
					}
					subject, err := leaseSubject(r, lease)
					if err != nil {
						return err
					}
					event := TakeReason{Via: "repair", Reason: options.Reason, At: s.At, Generation: lease.Generation, Subject: subject}
					updates[KeyTakes] = append(history, event)
				}
				if receipt.SupersededGeneration != 0 {
					lease.Ended = "superseded"
					updates[KeyLease] = lease
				}
				if recordKind(tuple.Kind) {
					receipt, err = recordCloseWrites(s, r, receipt, updates)
					if err != nil {
						return err
					}
				} else {
					sized := receipt
					if sized.Commit == "" {
						// The holding commit id has the length of the blob id.
						sized.Commit = strings.Repeat("0", len(sized.Blob))
					}
					if _, err = CheckMetadataSize(r.Key(), mergedMetadata(mergedMetadata(r.Metadata, updates), Metadata(KeyClose, sized))); err != nil {
						return err
					}
				}
				if options.BeforeClose != nil {
					if err = options.BeforeClose(s, receipt); err != nil {
						return err
					}
				}
				if pending != nil && receipt.Commit == "" {
					// Every check passed: publish the verdict, then write the receipt.
					if receipt.Commit, err = pending.publish(s); err != nil {
						return err
					}
					if err = ValidReceipt(s.Root, receipt, nil); err != nil {
						return err
					}
				}
				if err = checkClosingConflict(s, r); err != nil {
					return err
				}

				// The receipt, repair event and superseded lease share one durable write.
				updates[KeyClose] = receipt
				if err = s.Store.WriteMetadata(r.Key(), updates); err != nil {
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

func takeHistory(r Record) ([]TakeReason, error) {
	history, has, err := Decode[[]TakeReason](r.Metadata, KeyTakes)
	if err != nil || has {
		return history, err
	}
	old, has, err := Decode[TakeReason](r.Metadata, KeyTake)
	if has && old.Generation > 0 {
		history = append(history, old)
	}
	return history, err
}

func leaseSubject(r Record, lease LeaseRecord) (string, error) {
	t, _, _ := r.Tuple()
	if t.Kind == "implement" {
		return lease.ClaimBase, nil
	}
	subject, _, err := Decode[Subject](r.Metadata, KeySubject)
	return subject.Commit, err
}

// submitGeneration fences a revoked actor before any close-write hook can run.
func submitGeneration(r Record, lease LeaseRecord, token string, receipt Receipt, valid bool) (int, error) {
	if matchesToken(lease, token) {
		return lease.Generation, nil
	}
	history, err := takeHistory(r)
	if err != nil {
		return 0, err
	}
	for _, take := range history {
		if token == "" || take.RevokedTokenHash == "" || take.RevokedTokenHash != tokenHash(token) {
			continue
		}
		prefix, _, _ := strings.Cut(token, ".")
		generation, err := strconv.Atoi(prefix)
		if err != nil || generation <= 0 || generation != take.Generation-1 || generation >= lease.Generation {
			return 0, actorFailure("revoked lease generation is invalid")
		}
		if valid {
			if receipt.Generation == generation && receipt.SupersededGeneration == lease.Generation {
				return generation, nil
			}
			return 0, actorFailure("revoked lease refused: step already has a receipt from another generation")
		}
		if r.Status == "closed" {
			return 0, actorFailure("revoked lease refused: step is closed")
		}
		pin, has, err := Decode[Closing](r.Metadata, KeyClosing)
		if err != nil {
			return 0, err
		}
		if has && pin.Generation > generation {
			return 0, actorFailure("revoked lease refused: mindspec_closing belongs to a newer generation")
		}
		subject, err := leaseSubject(r, lease)
		if err != nil {
			return 0, err
		}
		if take.Subject != subject {
			return 0, actorFailure("revoked lease refused: step subject changed since the take")
		}
		return generation, nil
	}
	return 0, actorFailure("lease token is not current")
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
	if gitNamesRoot(args) {
		cmd.Env = rootQueryEnvironment(os.Environ(), runtime.GOOS)
	}
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

// gitNamesRoot classifies the query, independent of flag order and count.
func gitNamesRoot(args []string) bool {
	if len(args) < 2 {
		return false
	}
	if args[0] == "worktree" {
		return args[1] == "list"
	}
	if args[0] != "rev-parse" {
		return false
	}
	for _, arg := range args[1:] {
		switch arg {
		case "--show-toplevel", "--show-prefix", "--git-common-dir", "--git-dir", "--absolute-git-dir":
			return true
		}
	}
	return false
}

func rootQueryEnvironment(environ []string, goos string) []string {
	env := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		override := false
		for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE"} {
			if key == name || (goos == "windows" && strings.EqualFold(key, name)) {
				override = true
				break
			}
		}
		if !override {
			env = append(env, entry)
		}
	}
	return env
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
	expectedSubject := round.Subject
	if t.Kind == "verify" || t.Kind == "fix" {
		expectedSubject = reviewSubject(round)
	}
	if exists && subject.Commit != expectedSubject {
		return round, actorFailure("step subject is stale")
	}
	if (t.Kind == "lens" && round.Lens != "" && round.Lens != r.Key()) || (t.Kind == "verify" && round.Verify != "" && round.Verify != r.Key()) {
		return round, actorFailure("step is not the current round attempt")
	}
	if t.Kind == "review-slot" || t.Kind == "fix" {
		for _, id := range s.Steps {
			candidate := s.Records[id]
			ct, _, _ := candidate.Tuple()
			if ct.Gate != t.Gate || ct.Kind != t.Kind || ct.Round != t.Round || ct.Slot != t.Slot || ct.Attempt <= t.Attempt {
				continue
			}
			close, _, err := Decode[Receipt](candidate.Metadata, KeyClose)
			if err != nil {
				return round, pathFailure(err, "cannot read attempt receipt: ", "mindspec next-action")
			}
			if !(candidate.Status == "closed" && close.Kind == "superseded") {
				return round, actorFailure("step is not the current round attempt")
			}
		}
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
	base := reviewSubject(round)
	if subject, has, err := Decode[Subject](r.Metadata, KeySubject); err != nil {
		return err
	} else if has {
		base = subject.Commit
	}
	if err := commitPast(s, receipt, base, branch); err != nil {
		return err
	}
	paths, err := actorGitBytes(s.Root, "diff", "--no-renames", "--name-only", "-z", base, receipt.Commit, "--")
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
	bytes, err := ReceiptBytes(s.Root, receipt)
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
		message := "verdict must be in its registered panel directory"
		if path, err := slotVerdictPath(s, t); err == nil {
			message += ": " + termsafe.Escape(path)
		}
		return termsafe.NewFailure(message, slotRecovery(s, t))
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
			if option == "halt" {
				final := mergedMetadata(r.Metadata, map[string]any{KeyAnswer: answer, KeyClose: Receipt{Kind: "halted", HaltAnswer: r.Key(), Answer: &answer}})
				if _, err = CheckMetadataSize(r.Key(), final); err != nil {
					return err
				}
			}
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
