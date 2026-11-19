package kernel

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
)

const (
	KeyTuple     = "mindspec_tuple"
	KeyID        = "mindspec_key"
	KeyClose     = "mindspec_close"
	KeyFrom      = "mindspec_from"
	KeyLinks     = "mindspec_links"
	KeyRun       = "mindspec_run"
	KeyLease     = "mindspec_lease"
	KeyRound     = "mindspec_round"
	KeyTake      = "mindspec_take"
	KeyAnswer    = "mindspec_answer"
	KeyDefer     = "mindspec_defer"
	KeyJudgement = "mindspec_judgement"
	KeyRePanel   = "mindspec_repanel"
)

const (
	KeyPlanApproval = "plan_approved"
	KeyDecision     = "mindspec_decision"
	KeySubject      = "mindspec_subject"
	KeyRegistration = "mindspec_registration"
)

type RegistrationRecord struct {
	Slug    string   `json:"slug"`
	Round   int      `json:"round"`
	Subject string   `json:"subject"`
	Slots   []string `json:"slots"`
}

type Tuple = kernelid.Tuple
type PlanApproval bool
type ComputedKey string
type From string
type PlanLink struct {
	ID       string   `json:"id"`
	Upstream []string `json:"upstream"`
}
type Links []PlanLink
type Policy struct {
	RoundLimit int    `json:"round_limit"`
	LeaseTTL   string `json:"lease_ttl"`
	Rules      string `json:"rules,omitempty"`
}

var DefaultPolicy = Policy{RoundLimit: 5, LeaseTTL: "2h", Rules: "kernel-1"}

type LeaseRecord struct {
	Generation int       `json:"generation"`
	Expires    time.Time `json:"expires"`
	TokenHash  string    `json:"token_hash"`
	ClaimBase  string    `json:"claim_base"`
}
type GateRound struct {
	Round     int      `json:"round"`
	Subject   string   `json:"subject"`
	Slots     []string `json:"slots"`
	SlotCount int      `json:"slot_count"`
	Paths     []string `json:"paths,omitempty"`
}
type TakeReason struct {
	Reason     string    `json:"reason"`
	At         time.Time `json:"at"`
	Generation int       `json:"generation"`
}
type DecisionRecord struct {
	Options []string `json:"options"`
	Cause   string   `json:"cause"`
	Gate    string   `json:"gate"`
}
type DecisionAnswer struct {
	Option string    `json:"option"`
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"`
}
type DeferRecord struct {
	Finding string    `json:"finding"`
	To      string    `json:"to"`
	Reason  string    `json:"reason"`
	At      time.Time `json:"at"`
}
type Judgement struct {
	Value      string   `json:"value"`
	OldSubject string   `json:"old_subject"`
	NewSubject string   `json:"new_subject"`
	Reason     string   `json:"reason,omitempty"`
	SlotCount  int      `json:"slot_count,omitempty"`
	Paths      []string `json:"paths,omitempty"`
}
type RePanel struct {
	Judge     string   `json:"judge,omitempty"`
	SlotCount int      `json:"slot_count"`
	Paths     []string `json:"paths"`
}
type CarryOver struct {
	Rebind     string   `json:"rebind,omitempty"`
	OldSubject string   `json:"old_subject"`
	NewSubject string   `json:"new_subject"`
	Judge      string   `json:"judge"`
	Receipt    *Receipt `json:"receipt,omitempty"`
}
type Receipt struct {
	Kind       string          `json:"kind"`
	Path       string          `json:"path,omitempty"`
	Commit     string          `json:"commit,omitempty"`
	Blob       string          `json:"blob,omitempty"`
	Repaired   bool            `json:"repaired,omitempty"`
	Rebind     string          `json:"rebind,omitempty"`
	CarryOvers []CarryOver     `json:"carry_overs,omitempty"`
	Subject    string          `json:"subject,omitempty"`
	HaltAnswer string          `json:"halt_answer,omitempty"`
	Retally    string          `json:"retally,omitempty"`
	Answer     *DecisionAnswer `json:"answer,omitempty"`
}

// Metadata encodes named records for the store without losing JSON field names.
func Metadata(key string, value any) map[string]any { return map[string]any{key: value} }
func Decode[T any](metadata map[string]any, key string) (T, bool, error) {
	var value T
	raw, ok := metadata[key]
	if !ok {
		return value, false, nil
	}
	data, err := json.Marshal(raw)
	if err == nil {
		err = json.Unmarshal(data, &value)
	}
	return value, true, err
}

// ValidReceipt verifies either durable artifact identity or a reducer reason.
func ValidReceipt(root string, r Receipt) error {
	switch r.Kind {
	case "passed", "refused", "superseded", "halted":
	default:
		return fmt.Errorf("unknown receipt kind")
	}
	if r.Path != "" || r.Commit != "" || r.Blob != "" {
		if r.Path == "" || !objectID(r.Commit) || !objectID(r.Blob) {
			return fmt.Errorf("receipt needs path, commit and blob")
		}
		cmd := exec.Command("git", "rev-parse", "--verify", r.Commit+"^{commit}")
		cmd.Dir = root
		if _, err := cmd.Output(); err != nil {
			return fmt.Errorf("receipt holding commit does not exist: %w", err)
		}
		cmd = exec.Command("git", "rev-parse", "--verify", r.Commit+":"+r.Path)
		cmd.Dir = root
		blob, err := cmd.Output()
		if err != nil || strings.TrimSpace(string(blob)) != r.Blob {
			return fmt.Errorf("receipt blob does not match holding commit")
		}
		cmd = exec.Command("git", "cat-file", "-t", r.Blob)
		cmd.Dir = root
		kind, err := cmd.Output()
		if err != nil || strings.TrimSpace(string(kind)) != "blob" {
			return fmt.Errorf("receipt artifact is not a blob")
		}
		return nil
	}
	switch r.Kind {
	case "superseded":
		if r.Subject != "" || r.Retally != "" || r.Rebind != "" {
			return nil
		}
	case "halted":
		if r.HaltAnswer != "" {
			return nil
		}
	case "passed", "refused":
		if r.Retally != "" || (r.Answer != nil && r.Answer.Option != "") {
			return nil
		}
	}
	return fmt.Errorf("receipt has no artifact or close reason")
}

func objectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
