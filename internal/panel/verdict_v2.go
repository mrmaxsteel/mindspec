package panel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

const MaxVerdictBytes = 1 << 20
const MaxCommandTailBytes = 4096

type ChangeRequired struct {
	Text    string `json:"text"`
	Basis   string `json:"basis"`
	Command *int   `json:"command,omitempty"`
}
type CommandRun struct {
	Cmd  string      `json:"cmd"`
	Exit json.Number `json:"exit"`
	Tail string      `json:"tail"`
}
type VerdictV2 struct {
	Schema                  int              `json:"schema"`
	Slot                    string           `json:"slot"`
	Round                   int              `json:"round"`
	ReviewedSHA             string           `json:"reviewed_sha"`
	Status                  string           `json:"status"`
	Verdict                 string           `json:"verdict"`
	Model                   string           `json:"model"`
	Rationale               string           `json:"rationale"`
	ConcreteChangesRequired []ChangeRequired `json:"concrete_changes_required"`
	CommandsRun             []CommandRun     `json:"commands_run"`
	VoidReason              *string          `json:"void_reason,omitempty"`
	HardBlock               bool             `json:"hard_block,omitempty"`
	Findings                []string         `json:"findings,omitempty"`
	InputsRead              []string         `json:"inputs_read,omitempty"`
}

// verdictSchemaNode is the single definition of the schema layer. It is both
// executable and directly serializable as JSON Schema. Relations stay separate.
type verdictSchemaNode struct {
	Type                 string                        `json:"type"`
	Required             []string                      `json:"required,omitempty"`
	Properties           map[string]*verdictSchemaNode `json:"properties,omitempty"`
	AdditionalProperties *bool                         `json:"additionalProperties,omitempty"`
	Items                *verdictSchemaNode            `json:"items,omitempty"`
	MaxItems             int                           `json:"maxItems,omitempty"`
	MinLength            int                           `json:"minLength,omitempty"`
	Pattern              string                        `json:"pattern,omitempty"`
	Enum                 []any                         `json:"enum,omitempty"`
	Description          string                        `json:"description,omitempty"`
}

func objectSchema(required []string, properties map[string]*verdictSchemaNode) *verdictSchemaNode {
	no := false
	return &verdictSchemaNode{Type: "object", Required: required, Properties: properties, AdditionalProperties: &no}
}
func stringSchema(nonempty bool, choices ...string) *verdictSchemaNode {
	s := &verdictSchemaNode{Type: "string"}
	if nonempty {
		s.MinLength = 1
	}
	for _, c := range choices {
		s.Enum = append(s.Enum, c)
	}
	return s
}
func verdictV2Definition() *verdictSchemaNode {
	integer := func() *verdictSchemaNode { return &verdictSchemaNode{Type: "integer"} }
	stringsArray := func() *verdictSchemaNode { return &verdictSchemaNode{Type: "array", Items: stringSchema(false)} }
	schema := integer()
	schema.Enum = []any{2}
	sha := stringSchema(false)
	sha.Pattern = "^[0-9a-f]{40}$"
	s := objectSchema([]string{"schema", "slot", "round", "reviewed_sha", "status", "verdict", "model", "rationale", "concrete_changes_required", "commands_run"}, map[string]*verdictSchemaNode{
		"schema": schema, "slot": stringSchema(false), "round": integer(), "reviewed_sha": sha,
		"status": stringSchema(false, "draft", "final"), "verdict": stringSchema(false, "APPROVE", "REQUEST_CHANGES", "REJECT", "VOID"),
		"model": stringSchema(true), "rationale": stringSchema(false), "void_reason": stringSchema(false), "hard_block": {Type: "boolean"},
		"findings": stringsArray(), "inputs_read": stringsArray(),
		"concrete_changes_required": {Type: "array", MaxItems: 100, Items: objectSchema([]string{"text", "basis"}, map[string]*verdictSchemaNode{
			"text": stringSchema(true), "basis": stringSchema(false, "executed", "traced", "hypothesis"), "command": integer(),
		})},
		"commands_run": {Type: "array", MaxItems: 200, Items: objectSchema([]string{"cmd", "exit", "tail"}, map[string]*verdictSchemaNode{
			"cmd": stringSchema(false), "exit": integer(), "tail": stringSchema(false),
		})},
	})
	s.Description = "Transport and relation checks outside this schema: file size at most 1 MiB; UTF-8 tail size at most 4096 bytes; void_reason presence only for VOID; non-empty rationale unless VOID; executed command index binding; no command on other bases; verdict and concrete_changes_required cardinality."
	return s
}

// VerdictV2Schema returns the schema layer only, without transport or relations.
func VerdictV2Schema() ([]byte, error) { return json.MarshalIndent(verdictV2Definition(), "", "  ") }

// VerdictError retains the raw JSON pointer and detail for bounded retry prompts.
// The embedded failure supplies the terminal-safe message and recovery.
type VerdictError struct {
	*termsafe.Failure
	Pointer string
	Detail  string
}

// Unwrap exposes the embedded failure to errors.Is and errors.As.
func (e *VerdictError) Unwrap() error { return e.Failure }

func verdictError(pointer, message string) error {
	if pointer == "" {
		pointer = "/"
	}
	return &VerdictError{Failure: termsafe.NewFailure(fmt.Sprintf("%s: %s", pointer, message), "correct the verdict and submit it again"), Pointer: pointer, Detail: message}
}
func pointerChild(p, key string) string {
	return p + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
func integerValue(v any) (*big.Int, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(n.String())
	if !ok || !r.IsInt() {
		return nil, false
	}
	return r.Num(), true
}
func (s *verdictSchemaNode) validate(v any, p string) error {
	switch s.Type {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return verdictError(p, "must be an object")
		}
		for _, k := range s.Required {
			if _, ok := m[k]; !ok {
				return verdictError(pointerChild(p, k), "required field is missing")
			}
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child, ok := s.Properties[k]
			if !ok {
				return verdictError(pointerChild(p, k), "unknown field")
			}
			if err := child.validate(m[k], pointerChild(p, k)); err != nil {
				return err
			}
		}
	case "array":
		a, ok := v.([]any)
		if !ok {
			return verdictError(p, "must be an array")
		}
		if s.MaxItems > 0 && len(a) > s.MaxItems {
			return verdictError(p, fmt.Sprintf("at most %d entries are allowed", s.MaxItems))
		}
		for i, x := range a {
			if err := s.Items.validate(x, p+"/"+strconv.Itoa(i)); err != nil {
				return err
			}
		}
	case "string":
		x, ok := v.(string)
		if !ok {
			return verdictError(p, "must be a string")
		}
		if utf8.RuneCountInString(x) < s.MinLength {
			return verdictError(p, "must not be empty")
		}
		if s.Pattern != "" && !regexp.MustCompile(s.Pattern).MatchString(x) {
			return verdictError(p, "has an invalid format")
		}
	case "integer":
		if _, ok := integerValue(v); !ok {
			return verdictError(p, "must be an integer")
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return verdictError(p, "must be a boolean")
		}
	}
	if len(s.Enum) > 0 {
		found := false
		for _, x := range s.Enum {
			if fmt.Sprint(x) == fmt.Sprint(v) {
				found = true
			}
			if i, ok := x.(int); ok {
				n, valid := integerValue(v)
				if valid && n.Cmp(big.NewInt(int64(i))) == 0 {
					found = true
				}
			}
		}
		if !found {
			return verdictError(p, "value is not in the allowed set")
		}
	}
	return nil
}
func decodeVerdictObject(data []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, verdictError("", err.Error())
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, verdictError("", "expected one JSON object")
	}
	return v, nil
}

// ValidateVerdictV2Schema checks only the layer exported as JSON Schema.
func ValidateVerdictV2Schema(data []byte) error {
	v, err := decodeVerdictObject(data)
	if err != nil {
		return err
	}
	return verdictV2Definition().validate(v, "")
}

// normalizeIntegers preserves mathematical JSON integers such as 2.0 on decode.
func normalizeIntegers(v any) any {
	switch x := v.(type) {
	case json.Number:
		if n, ok := integerValue(x); ok {
			return json.Number(n.String())
		}
	case map[string]any:
		for k, e := range x {
			x[k] = normalizeIntegers(e)
		}
	case []any:
		for i, e := range x {
			x[i] = normalizeIntegers(e)
		}
	}
	return v
}
func ParseVerdictV2(data []byte) (VerdictV2, error) {
	var v VerdictV2
	if len(data) > MaxVerdictBytes {
		return v, verdictError("", "file exceeds 1 MiB")
	}
	if !utf8.Valid(data) {
		return v, verdictError("", "file is not UTF-8")
	}
	raw, err := decodeVerdictObject(data)
	if err != nil {
		return v, err
	}
	if err := verdictV2Definition().validate(raw, ""); err != nil {
		return v, err
	}
	// Check indices before decoding into machine integers, so even an enormous
	// invalid index reports the exact item pointer instead of a decode failure.
	object := raw.(map[string]any)
	commandCount := big.NewInt(int64(len(object["commands_run"].([]any))))
	for i, entry := range object["concrete_changes_required"].([]any) {
		item := entry.(map[string]any)
		pointer := fmt.Sprintf("/concrete_changes_required/%d/command", i)
		command, present := item["command"]
		if item["basis"] == "executed" {
			index, valid := integerValue(command)
			if !valid || index.Sign() < 0 || index.Cmp(commandCount) >= 0 {
				return v, verdictError(pointer, "executed item needs an existing command index")
			}
		} else if present {
			return v, verdictError(pointer, "allowed only for executed items")
		}
	}
	normalized, _ := json.Marshal(normalizeIntegers(raw))
	if err := json.Unmarshal(normalized, &v); err != nil {
		pointer := ""
		if typed, ok := err.(*json.UnmarshalTypeError); ok {
			pointer = "/" + strings.ReplaceAll(typed.Field, ".", "/")
		}
		return v, verdictError(pointer, err.Error())
	}
	if v.Verdict == "VOID" {
		if v.VoidReason == nil {
			return v, verdictError("/void_reason", "required for VOID")
		}
	} else {
		if v.VoidReason != nil {
			return v, verdictError("/void_reason", "allowed only for VOID")
		}
		if v.Rationale == "" {
			return v, verdictError("/rationale", "must not be empty")
		}
	}
	if (v.Verdict == "REQUEST_CHANGES") != (len(v.ConcreteChangesRequired) > 0) {
		return v, verdictError("/concrete_changes_required", "REQUEST_CHANGES needs items; other verdicts need an empty array")
	}

	for i, c := range v.CommandsRun {
		if len(c.Tail) > MaxCommandTailBytes {
			return v, verdictError(fmt.Sprintf("/commands_run/%d/tail", i), "tail exceeds 4096 UTF-8 bytes")
		}
	}
	return v, nil
}
func ValidateVerdictV2(data []byte) error { _, err := ParseVerdictV2(data); return err }

// FindingVerdict projects item texts before the existing positional hash rules.
func (v VerdictV2) FindingVerdict() FindingVerdict {
	texts := make([]string, len(v.ConcreteChangesRequired))
	for i, c := range v.ConcreteChangesRequired {
		texts[i] = c.Text
	}
	return FindingVerdict{Verdict: v.Verdict, HardBlock: v.HardBlock, Rationale: v.Rationale, ConcreteChangesRequired: texts}
}
