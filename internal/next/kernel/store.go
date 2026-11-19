package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

type Runner func(...string) ([]byte, error)
type Dependency struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DependsOnID string `json:"depends_on_id,omitempty"`
}
type Record struct {
	ID           string         `json:"id"`
	Parent       string         `json:"parent,omitempty"`
	Title        string         `json:"title"`
	Description  string         `json:"description,omitempty"`
	Type         string         `json:"issue_type"`
	Status       string         `json:"status"`
	Labels       []string       `json:"labels,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Dependencies []Dependency   `json:"dependencies,omitempty"`
}

func (r Record) Key() string {
	if key, ok := r.Metadata[KeyID].(string); ok {
		return key
	}
	return r.ID
}
func (r Record) Tuple() (Tuple, bool, error) { return Decode[Tuple](r.Metadata, KeyTuple) }

type Store struct {
	Root      string
	Epic      string
	Run       Runner
	NativeIDs bool
}

func NewStore(root, epic string, run Runner) *Store {
	if run == nil {
		run = func(args ...string) ([]byte, error) { return bead.RunBDAt(root, args...) }
	}
	return &Store{Root: root, Epic: epic, Run: run, NativeIDs: NativeIDs}
}

func (s *Store) All() ([]Record, error) {
	out, err := s.Run("list", "--all", "--json", "-n", "0")
	if err != nil {
		return nil, err
	}
	var records []Record
	if err = json.Unmarshal(out, &records); err != nil {
		return nil, fmt.Errorf("read store: %w", err)
	}
	return records, nil
}

// Lookup resolves computed keys in either identity mode. Duplicate keys refuse.
func (s *Store) Lookup(key string) (*Record, error) {
	records, err := s.All()
	if err != nil {
		return nil, err
	}
	var found *Record
	for _, r := range records {
		if r.ID == key || r.Key() == key {
			if found != nil {
				return nil, collisionError(key)
			}
			copy := r
			found = &copy
		}
	}
	return found, nil
}

func collisionError(key string) error {
	return fmt.Errorf("id-collision: %s; remove that bead with bd", termsafe.Escape(key))
}

// Collision refuses an identity occupied by a record without the matching tuple.
func (s *Store) Collision(key string, t Tuple) error {
	r, err := s.Lookup(key)
	if err != nil || r == nil {
		return err
	}
	have, ok, err := r.Tuple()
	if err != nil || !ok || !reflect.DeepEqual(have, t) {
		return collisionError(key)
	}
	return nil
}

type CreateRecord struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Parent   string         `json:"parent"`
	Title    string         `json:"title"`
	Tuple    Tuple          `json:"tuple"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

var ErrCASMismatch = errors.New("store status changed concurrently")

type exitCoder interface{ ExitCode() int }

// SetStatusIf makes one compare-and-set write and preserves the mismatch sentinel.
func (s *Store) SetStatusIf(key, from, to string) error {
	r, err := s.owned(key)
	if err != nil {
		return err
	}
	_, err = s.Run("update", r.ID, "--status", to, "--if-status", from)
	var code exitCoder
	if errors.As(err, &code) && code.ExitCode() == 13 {
		return ErrCASMismatch
	}
	return err
}

// WithCASRetry rereads and reruns once after a compare-and-set mismatch.
func WithCASRetry(fn func() error) error {
	err := fn()
	if !errors.Is(err, ErrCASMismatch) {
		return err
	}
	if err = fn(); errors.Is(err, ErrCASMismatch) {
		return fmt.Errorf("store changed twice; retry the command: %w", err)
	}
	return err
}

func (s *Store) owned(key string) (*Record, error) {
	r, err := s.Lookup(key)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("record is absent: %s", termsafe.Escape(key))
	}
	if key == s.Epic {
		return r, nil
	}
	if t, err := kernelid.Parse(key); err == nil && t.Epic == s.Epic {
		if err = s.Collision(key, t); err != nil {
			return nil, err
		}
		return r, nil
	}
	epic, err := s.Lookup(s.Epic)
	if err != nil {
		return nil, err
	}
	if epic != nil {
		links, _, err := Decode[Links](epic.Metadata, KeyLinks)
		if err != nil {
			return nil, err
		}
		for _, link := range links {
			if link.ID == key {
				return r, nil
			}
		}
	}
	return nil, fmt.Errorf("foreign record: %s", termsafe.Escape(key))
}

// WriteMetadata merges named records without replacing unrelated metadata.
func (s *Store) WriteMetadata(key string, updates map[string]any) error {
	r, err := s.owned(key)
	if err != nil {
		return err
	}
	metadata := map[string]any{}
	for k, v := range r.Metadata {
		metadata[k] = v
	}
	for k, v := range updates {
		metadata[k] = v
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = s.Run("update", r.ID, "--metadata", string(data))
	return err
}
func (s *Store) WriteReceipt(key string, receipt Receipt) error {
	return s.WriteMetadata(key, Metadata(KeyClose, receipt))
}

// AddLink writes only between the computed or recorded identities of this epic.
func (s *Store) AddLink(downstream, upstream string) error {
	down, err := s.owned(downstream)
	if err != nil {
		return err
	}
	up, err := s.owned(upstream)
	if err != nil {
		return err
	}
	_, err = s.Run("dep", "add", down.ID, up.ID, "--type", "blocks")
	return err
}

// RequireStepType refuses a store that has not declared the kernel step type.
func RequireStepType(root string) error {
	for _, value := range bead.CustomTypes(root) {
		if value == "step" {
			return nil
		}
	}
	return fmt.Errorf("custom issue type step is missing\nrecovery: mindspec setup")
}

func markedTitle(title string) string {
	if strings.HasPrefix(title, "[mindspec]") {
		return title
	}
	return "[mindspec] " + title
}

// Create creates or reuses one computed identity and never adopts a collision.
func (s *Store) Create(c CreateRecord) (*Record, error) {
	t, err := kernelid.Parse(c.ID)
	if err != nil || t != c.Tuple || t.Epic != s.Epic {
		return nil, fmt.Errorf("create needs a matching kernel tuple")
	}
	canonical, err := kernelid.Format(t)
	if err != nil || c.ID != canonical {
		return nil, fmt.Errorf("create needs a canonical kernel identity")
	}
	if c.Type != "step" && c.Type != "decision" {
		return nil, fmt.Errorf("kernel type must be step or decision")
	}
	if (t.Kind == "decision") != (c.Type == "decision") {
		return nil, fmt.Errorf("type does not match kernel kind")
	}
	if err = s.Collision(c.ID, c.Tuple); err != nil {
		return nil, err
	}
	existing, err := s.Lookup(c.ID)
	if err != nil || existing != nil {
		return existing, err
	}
	parent, err := s.owned(c.Parent)
	if err != nil {
		return nil, err
	}
	metadata := map[string]any{}
	for k, v := range c.Metadata {
		metadata[k] = v
	}
	metadata[KeyTuple] = c.Tuple
	metadata[KeyID] = ComputedKey(c.ID)
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	args := []string{"create", markedTitle(c.Title), "--type", c.Type, "--parent", parent.ID, "--labels", "mindspec", "--metadata", string(data), "--json"}
	if s.NativeIDs {
		args = append(args, "--id", c.ID)
	}
	out, err := s.Run(args...)
	if err != nil {
		return nil, err
	}
	var record Record
	if err = json.Unmarshal(out, &record); err != nil {
		return nil, err
	}
	if record.ID == "" {
		return nil, fmt.Errorf("create returned no identity")
	}
	return &record, nil
}
