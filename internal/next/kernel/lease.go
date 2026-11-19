package kernel

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// LeaseDeps supplies the workflow worktree adapter without importing it.
type LeaseDeps struct {
	EnsureWorktree func(root, workItem, specID string) (string, error)
}
type LeaseOptions struct {
	Step    string
	Renew   string
	Release string
	Take    bool
	Reason  string
}
type LeaseResult struct {
	Token    string
	Lease    LeaseRecord
	Worktree string
}

func Lease(state State, options LeaseOptions, deps LeaseDeps) (LeaseResult, error) {
	var result LeaseResult
	if !state.Enrolled {
		return result, ErrPreKernel
	}
	err := withActorLock(state, func(s State) error {
		return WithCASRetry(func() error {
			s, err := refreshActorState(s)
			if err != nil {
				return err
			}
			r, t, err := actorRecord(s, options.Step, false)
			if err != nil {
				return err
			}
			if err = refuseHalt(s); err != nil {
				return err
			}
			if r.Status == "closed" {
				return actorFailure("step is closed: " + r.Key())
			}
			if err = actorBlockers(s, r); err != nil {
				return err
			}
			modes := 0
			for _, enabled := range []bool{options.Renew != "", options.Release != "", options.Take} {
				if enabled {
					modes++
				}
			}
			if modes > 1 || (options.Take && strings.TrimSpace(options.Reason) == "") || (!options.Take && options.Reason != "") {
				return actorFailure("use one lease operation; lease --take requires --reason")
			}
			ttl, err := time.ParseDuration(s.Policy.LeaseTTL)
			if err != nil || ttl <= 0 {
				return actorFailure("pinned lease duration is invalid")
			}
			lease, _, err := Decode[LeaseRecord](r.Metadata, KeyLease)
			if err != nil {
				return err
			}
			updates := map[string]any{}
			status := "in_progress"
			if options.Renew != "" || options.Release != "" {
				token := options.Renew
				if options.Release != "" {
					token = options.Release
				}
				if !matchesToken(lease, token) {
					return actorFailure("lease token is not current")
				}
				if options.Release != "" {
					lease.TokenHash = ""
					lease.Expires = s.At
					status = "open"
				} else {
					lease.Expires = s.At.Add(ttl)
				}
			} else {
				if lease.TokenHash != "" && lease.Expires.After(s.At) && !options.Take {
					return fmt.Errorf("step has a live lease: %s\nrecovery: mindspec lease --take %s --reason <text>", termsafe.Escape(r.Key()), termsafe.Escape(r.Key()))
				}
				var secret [32]byte
				if _, err = rand.Read(secret[:]); err != nil {
					return err
				}
				result.Token = fmt.Sprintf("%d.%s", lease.Generation+1, hex.EncodeToString(secret[:]))
				lease.Generation++
				lease.TokenHash = tokenHash(result.Token)
				lease.Expires = s.At.Add(ttl)
				if t.Kind == "implement" {
					if deps.EnsureWorktree == nil {
						return actorFailure("implementation lease needs a worktree adapter")
					}
					result.Worktree, err = deps.EnsureWorktree(s.Root, t.WorkItem, s.SpecID)
					if err != nil {
						return err
					}
					lease.ClaimBase, err = actorGit(s.Root, "rev-parse", "--verify", "refs/heads/bead/"+t.WorkItem+"^{commit}")
					if err != nil {
						return err
					}
				}
				if options.Take {
					updates[KeyTake] = TakeReason{Reason: options.Reason, At: s.At, Generation: lease.Generation}
				}
			}
			updates[KeyLease] = lease
			if err = s.Store.SetStatusMetadataIf(r.Key(), r.Status, status, updates); err != nil {
				return err
			}
			result.Lease = lease
			return nil
		})
	})
	if err != nil {
		return LeaseResult{}, err
	}
	return result, nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func matchesToken(lease LeaseRecord, token string) bool {
	return token != "" && lease.Generation > 0 && lease.TokenHash != "" && tokenHash(token) == lease.TokenHash
}
func actorFailure(message string) error {
	return fmt.Errorf("%s\nrecovery: mindspec next-action", termsafe.Escape(message))
}
func refuseHalt(s State) error {
	ids := []string{}
	for id, answer := range s.Answers() {
		if answer.Option == "halt" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		return actorFailure("spec is halted by " + ids[0])
	}
	return nil
}
func withActorLock(s State, fn func(State) error) error {
	if s.Store == nil {
		return actorFailure("operation needs a store")
	}
	return phase.WithPhaseLock(s.Root, func() error {
		fresh, err := refreshActorState(s)
		if err != nil {
			return err
		}
		return fn(fresh)
	})
}
func refreshActorState(s State) (State, error) { return ReadState(s.Store, s.SpecID, s.SpecDir) }
func actorRecord(s State, id string, decision bool) (Record, Tuple, error) {
	r, ok := s.Records[id]
	if !ok {
		for _, candidate := range s.Records {
			if candidate.ID == id {
				r, ok = candidate, true
				break
			}
		}
	}
	t, err := kernelid.Parse(r.Key())
	have, present, e := r.Tuple()
	if !ok || err != nil || e != nil || !present || t != have || t.Epic != s.Epic {
		return Record{}, Tuple{}, actorFailure("foreign step: " + id)
	}
	if t.WorkItem != "" {
		recorded := false
		for _, item := range s.Items {
			recorded = recorded || item == t.WorkItem
		}
		if !recorded {
			return Record{}, Tuple{}, actorFailure("work item is not in the approved plan: " + t.WorkItem)
		}
	}
	valid := t.Kind == "implement" || t.Kind == "review-slot" || t.Kind == "fix" || t.Kind == "change-judge"
	if decision {
		valid = t.Kind == "decision"
	}
	if !valid {
		return Record{}, Tuple{}, actorFailure("identity is not an actor step: " + id)
	}
	return r, t, nil
}

// actorBlockers reads foreign upstreams too, without making them write targets.
func actorBlockers(s State, target Record) error {
	all, err := s.Store.All()
	if err != nil {
		return err
	}
	records := map[string]Record{}
	for _, r := range all {
		records[r.ID] = r
		records[r.Key()] = r
	}
	seen := map[string]bool{}
	var visit func(Record, bool) error
	visit = func(r Record, upstream bool) error {
		held := r.Status == "blocked" || r.Status == "deferred" || r.Status == "on_hold"
		lease, _, e := Decode[LeaseRecord](r.Metadata, KeyLease)
		if e != nil {
			return e
		}
		if r.ID != target.ID && lease.TokenHash != "" && lease.Expires.After(s.At) {
			held = true
		}
		if held || (upstream && r.Status != "closed") {
			return actorFailure("step is held or blocked by " + r.Key())
		}
		if seen[r.ID] {
			return nil
		}
		seen[r.ID] = true
		parent := r.Parent
		for _, d := range r.Dependencies {
			id := d.ID
			if d.DependsOnID != "" {
				id = d.DependsOnID
			}
			if d.Type == "parent-child" {
				parent = id
			}
			if d.Type == "blocks" {
				up, ok := records[id]
				if !ok {
					return actorFailure("cannot read blocker " + id)
				}
				if e = visit(up, true); e != nil {
					return e
				}
			}
		}
		if parent != "" {
			p, ok := records[parent]
			if !ok {
				return actorFailure("cannot read ancestor " + parent)
			}
			if e = visit(p, false); e != nil {
				return e
			}
		}
		return nil
	}
	if err = visit(target, false); err != nil {
		return err
	}
	tuple, _, _ := target.Tuple()
	for _, decision := range s.OpenDecisions() {
		dt, _, _ := decision.Tuple()
		if dt.Gate == tuple.Gate && (dt.Cause != "limit" || tuple.Kind == "fix") {
			return actorFailure("answer the decision first: " + decision.Key())
		}
	}
	return nil
}

type openActorChild struct{ id string }

func (e *openActorChild) Error() string {
	return actorFailure("open child prevents close: " + e.id).Error()
}

func actorChildren(s State, target Record) error {
	all, err := s.Store.All()
	if err != nil {
		return err
	}
	for _, child := range all {
		if child.Status == "closed" {
			continue
		}
		parent := child.Parent
		for _, d := range child.Dependencies {
			if d.Type == "parent-child" {
				parent = d.ID
				if d.DependsOnID != "" {
					parent = d.DependsOnID
				}
			}
		}
		if parent == target.ID || parent == target.Key() {
			return &openActorChild{id: child.Key()}
		}
	}
	return nil
}
