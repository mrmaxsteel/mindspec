package kernel

import (
	"fmt"
	"sync"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

var ErrPreKernel error = termsafe.NewFailure("this spec uses the existing lifecycle; use mindspec next, mindspec complete and mindspec impl approve", "mindspec next")

type Subject struct {
	Commit string `json:"commit"`
}
type Action struct {
	Step    string   `json:"step"`
	Kind    string   `json:"kind"`
	Exists  bool     `json:"exists"`
	Round   int      `json:"round"`
	Subject Subject  `json:"subject"`
	Claim   []string `json:"claim"`
	Brief   []string `json:"brief"`
	Submit  []string `json:"submit"`
}

// LeaseView contains the public lease fields.
type LeaseView struct {
	Generation int       `json:"generation"`
	Expires    time.Time `json:"expires"`
}

// PolicyView contains the public policy fields.
type PolicyView struct {
	RoundLimit int    `json:"round_limit"`
	LeaseTTL   string `json:"lease_ttl"`
}
type Waiting struct {
	Step  string    `json:"step"`
	Lease LeaseView `json:"lease"`
}
type Decision struct {
	Step    string   `json:"step"`
	Options []string `json:"options"`
	Submit  []string `json:"submit"`
}
type Run struct {
	Actor string   `json:"actor"`
	Argv  []string `json:"argv"`
	Paths []string `json:"paths,omitempty"`
}
type Flag struct {
	Code    string   `json:"code"`
	Step    string   `json:"step,omitempty"`
	Message string   `json:"message,omitempty"`
	Repair  []string `json:"repair"`
}
type Output struct {
	Schema    string     `json:"schema"`
	Status    string     `json:"status"`
	Applied   bool       `json:"applied"`
	Revision  string     `json:"revision"`
	RetryAt   string     `json:"retry_at"`
	Policy    PolicyView `json:"policy"`
	Actions   []Action   `json:"actions"`
	Waiting   []Waiting  `json:"waiting"`
	Decisions []Decision `json:"decisions"`
	Run       []Run      `json:"run"`
	Flags     []Flag     `json:"flags"`
}

type Mode string

const (
	Ordinary Mode = "ordinary"
	Halt     Mode = "halt"
	FlagOnly Mode = "flag"
)

type RuleSpec struct {
	Name string `json:"name"`
	Mode Mode   `json:"mode"`
}

var RuleContracts = []RuleSpec{
	{"recreate", Ordinary}, {"missing-link", Ordinary}, {"judge-apply", Ordinary}, {"tally", Ordinary},
	{"refused-round", Ordinary}, {"verify-apply", Ordinary}, {"final-round", Ordinary},
	{"judge-supersede", Ordinary}, {"judge-create", Ordinary}, {"round-open", Ordinary},
	{"lens-create", Ordinary}, {"slots", Ordinary}, {"implement", Ordinary},
	{"retry-attempt", Ordinary}, {"complete-entry", Ordinary}, {"halt", Halt},
	{"run-marker-missing", FlagOnly}, {"foreign-child-open", FlagOnly}, {"foreign-blocker", FlagOnly}, {"id-collision", FlagOnly},
	{"unreceipted-close", FlagOnly}, {"raw-close", FlagOnly}, {"forced-close", FlagOnly}, {"raw-claim", FlagOnly},
}

type Rule func(State) (Changes, error)
type registered struct {
	mode Mode
	fn   Rule
}
type Registry struct {
	names   []string
	entries map[string]registered
}

func NewRegistry(names ...string) *Registry {
	return &Registry{names: append([]string{}, names...), entries: map[string]registered{}}
}
func (r *Registry) Register(name string, mode Mode, fn Rule) error {
	allowed := false
	for _, n := range r.names {
		if n == name {
			allowed = true
		}
	}
	if !allowed {
		return fmt.Errorf("unlisted rule %q", name)
	}
	if _, ok := r.entries[name]; ok {
		return fmt.Errorf("duplicate rule %q", name)
	}
	if mode != Ordinary && mode != Halt && mode != FlagOnly {
		return fmt.Errorf("unknown rule mode %q", mode)
	}
	if fn == nil {
		return fmt.Errorf("nil rule %q", name)
	}
	r.entries[name] = registered{mode, fn}
	return nil
}

var DefaultRegistry = func() *Registry {
	var names []string
	for _, s := range RuleContracts {
		names = append(names, s.Name)
	}
	return NewRegistry(names...)
}()
var rulesMu sync.RWMutex
var rulesOverride *Registry

func Rules() *Registry {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	if rulesOverride != nil {
		return rulesOverride
	}
	return DefaultRegistry
}
func RegisterRule(name string, mode Mode, fn Rule) error {
	for _, s := range RuleContracts {
		if name == s.Name {
			if mode != s.Mode {
				return fmt.Errorf("wrong mode for rule %q", name)
			}
			return DefaultRegistry.Register(name, mode, fn)
		}
	}
	return fmt.Errorf("unlisted rule %q", name)
}

// ReplaceRulesForTest installs a temporary registry. Tests must not run in parallel.
func ReplaceRulesForTest(reg *Registry) func() {
	rulesMu.Lock()
	old := rulesOverride
	rulesOverride = reg
	rulesMu.Unlock()
	return func() { rulesMu.Lock(); rulesOverride = old; rulesMu.Unlock() }
}

type Write struct {
	Kind     string         `json:"kind"`
	ID       string         `json:"id"`
	Create   *CreateRecord  `json:"create,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Receipt  *Receipt       `json:"receipt,omitempty"`
	Upstream string         `json:"upstream,omitempty"`
}
type Changes struct {
	Writes    []Write
	Flags     []Flag
	Actions   []Action
	Waiting   []Waiting
	Decisions []Decision
	Run       []Run
}
type PassResult struct {
	Output Output
	Writes []Write
	State  State
}
