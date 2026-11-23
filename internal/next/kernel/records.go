package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
)

const (
	KeyTuple      = "mindspec_tuple"
	KeyID         = "mindspec_key"
	KeyClose      = "mindspec_close"
	KeyCompletion = "mindspec_completed_subject"
	KeyFrom       = "mindspec_from"
	KeyLinks      = "mindspec_links"
	KeyRun        = "mindspec_run"
	KeyLease      = "mindspec_lease"
	KeyRound      = "mindspec_round"
	KeyTake       = "mindspec_take"
	KeyTakes      = "mindspec_takes"
	KeyDeferrals  = "mindspec_deferrals"
	KeyRawClaims  = "mindspec_raw_claims"
	KeyAnswer     = "mindspec_answer"
	KeyDefer      = "mindspec_defer"
	KeyJudgement  = "mindspec_judgement"
	KeyRePanel    = "mindspec_repanel"
	KeyRecord     = "mindspec_record"
	KeyClosing    = "mindspec_closing"
	KeyFindings   = "mindspec_findings"
	KeyRead       = "mindspec_read"
	KeyReturns    = "mindspec_returns"
	KeyReproduced = "mindspec_reproduced"
)

const (
	KeyPlanApproval = "plan_approved"
	KeyDecision     = "mindspec_decision"
	KeySubject      = "mindspec_subject"
	KeyRegistration = "mindspec_registration"
)

type RegistrationRecord struct {
	Directories []string `json:"directories,omitempty"`
	Slug        string   `json:"slug"`
	Round       int      `json:"round"`
	Subject     string   `json:"subject"`
	Slots       []string `json:"slots"`
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
	Ended      string    `json:"ended,omitempty"`
}
type GateRound struct {
	Round     int               `json:"round"`
	Subject   string            `json:"subject"`
	Slots     []string          `json:"slots"`
	SlotCount int               `json:"slot_count"`
	Paths     []string          `json:"paths,omitempty"`
	Lenses    []SlotLens        `json:"lenses,omitempty"`
	Lens      string            `json:"lens,omitempty"`
	Verify    string            `json:"verify,omitempty"`
	Prior     *GateRound        `json:"prior,omitempty"`
	Read      map[string]string `json:"read,omitempty"`
	Asks      []PriorAsk        `json:"asks,omitempty"`
	Judged    *Judged           `json:"judged,omitempty"`
}

type SlotLens struct {
	Slot string `json:"slot"`
	Lens string `json:"lens"`
}
type LensRecord struct {
	Step   string     `json:"step"`
	Lenses []SlotLens `json:"lenses"`
}
type Finding struct {
	Slot          string `json:"slot"`
	Round         int    `json:"round"`
	Finding       string `json:"finding"`
	FindingSHA256 string `json:"finding_sha256"`
	Text          string `json:"text,omitempty"`
}
type VerifyReturn struct {
	Slot          string `json:"slot"`
	Round         int    `json:"round"`
	Finding       string `json:"finding"`
	FindingSHA256 string `json:"finding_sha256"`
	Return        string `json:"return"`
	Ground        string `json:"ground,omitempty"`
	AC            string `json:"ac,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Evidence      string `json:"evidence,omitempty"`
	Target        string `json:"target,omitempty"`
}
type VerifyRecord struct {
	Step    string         `json:"step"`
	Subject string         `json:"subject"`
	Returns []VerifyReturn `json:"returns"`
}
type PriorAsk struct {
	Finding
	Returns []VerifyReturn `json:"returns,omitempty"`
}
type Judged struct {
	Judge      string `json:"judge"`
	Value      string `json:"value"`
	NewSubject string `json:"new_subject"`
}
type ClosingIdentity struct {
	Generation   int    `json:"generation"`
	RecordSHA256 string `json:"record_sha256"`
}
type Closing struct {
	Generation   int              `json:"generation"`
	RecordSHA256 string           `json:"record_sha256"`
	At           time.Time        `json:"at"`
	Conflict     *ClosingIdentity `json:"conflict,omitempty"`
}

// RawClaim records a claim observed when a lease replaces it.
type RawClaim struct {
	At time.Time `json:"at"`
}

type TakeReason struct {
	Reason           string    `json:"reason"`
	At               time.Time `json:"at"`
	Generation       int       `json:"generation"`
	Via              string    `json:"via,omitempty"`
	RevokedTokenHash string    `json:"revoked_token_hash,omitempty"`
	Subject          string    `json:"subject,omitempty"`
}
type DecisionRecord struct {
	Options []string       `json:"options"`
	Cause   string         `json:"cause"`
	Gate    string         `json:"gate"`
	Slots   []DecisionSlot `json:"slots,omitempty"`
}
type DecisionSlot struct {
	Slot    string `json:"slot"`
	Attempt int    `json:"attempt"`
}
type DecisionAnswer struct {
	Option string    `json:"option"`
	At     time.Time `json:"at"`
}
type DeferRecord struct {
	Panel   string    `json:"panel,omitempty"`
	Round   int       `json:"round,omitempty"`
	Slot    string    `json:"slot,omitempty"`
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
	Judge string `json:"judge,omitempty"`
	// SlotCount is the count the judge authorized. It stays unchanged when the
	// subject moves. GateRound alone states the slots that run.
	SlotCount int `json:"slot_count"`
	// Paths is the scope the judge authorized. It stays unchanged when the subject moves.
	Paths []string `json:"paths"`
}
type CarryOver struct {
	Rebind     string   `json:"rebind,omitempty"`
	OldSubject string   `json:"old_subject"`
	NewSubject string   `json:"new_subject"`
	Judge      string   `json:"judge"`
	Receipt    *Receipt `json:"receipt,omitempty"`
}
type Receipt struct {
	Kind                 string          `json:"kind"`
	Path                 string          `json:"path,omitempty"`
	Commit               string          `json:"commit,omitempty"`
	Blob                 string          `json:"blob,omitempty"`
	Repaired             bool            `json:"repaired,omitempty"`
	Rebind               string          `json:"rebind,omitempty"`
	CarryOvers           []CarryOver     `json:"carry_overs,omitempty"`
	Subject              string          `json:"subject,omitempty"`
	HaltAnswer           string          `json:"halt_answer,omitempty"`
	Retally              string          `json:"retally,omitempty"`
	Answer               *DecisionAnswer `json:"answer,omitempty"`
	RecordSHA256         string          `json:"record_sha256,omitempty"`
	Generation           int             `json:"generation,omitempty"`
	SupersededGeneration int             `json:"superseded_generation,omitempty"`
	Via                  string          `json:"via,omitempty"`
	At                   string          `json:"at,omitempty"`
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
func ValidReceipt(root string, r Receipt, metadata map[string]any) error {
	switch r.Kind {
	case "passed", "refused", "superseded", "halted":
	default:
		return fmt.Errorf("unknown receipt kind")
	}
	tuple, _, _ := Decode[Tuple](metadata, KeyTuple)
	if r.RecordSHA256 != "" || (recordKind(tuple.Kind) && (r.Kind == "passed" || r.Kind == "refused")) {
		record, present, err := Decode[string](metadata, KeyRecord)
		if err != nil || !present || r.RecordSHA256 == "" || r.RecordSHA256 != recordDigest([]byte(record)) {
			return fmt.Errorf("receipt record_sha256 does not match mindspec_record")
		}
		if r.Path != "" || r.Commit != "" || r.Blob != "" {
			return fmt.Errorf("record receipt cannot name a path, commit or blob")
		}
		return nil
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

func recordKind(kind string) bool {
	return kind == "lens" || kind == "verify" || kind == "change-judge"
}
func recordDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func objectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
