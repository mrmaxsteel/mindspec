package panel_test

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestVerdictV2_Accepts(t *testing.T) {
	for _, kind := range []string{"APPROVE", "REQUEST_CHANGES", "REJECT", "VOID"} {
		t.Run(kind, func(t *testing.T) {
			v := paneltest.Verdict(kind)
			if kind == "REQUEST_CHANGES" {
				zero := 0
				v.ConcreteChangesRequired = append(v.ConcreteChangesRequired, panel.ChangeRequired{Text: "executed finding", Basis: "executed", Command: &zero}, panel.ChangeRequired{Text: "untested finding", Basis: "hypothesis"})
				v.CommandsRun = []panel.CommandRun{{Cmd: "go test", Exit: "0", Tail: "PASS"}}
			}
			if err := panel.ValidateVerdictV2(paneltest.Bytes(t, v)); err != nil {
				t.Fatal(err)
			}
		})
	}
	v := paneltest.Verdict("REQUEST_CHANGES")
	v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "a", Basis: "traced"}, {Text: "b", Basis: "hypothesis"}}
	findings, err := panel.Findings(v.FindingVerdict())
	if err != nil || len(findings) != 2 || findings[1].ID != "ccr[1]" {
		t.Fatalf("projection: %v %v", findings, err)
	}
	slot, err := panel.SlotFinding(v.FindingVerdict())
	if err != nil || slot.SHA256 != "0473ef2dc0d324ab659d3580c1134e9d812035905c4781fdd6d529b0c6860e13" {
		t.Fatalf("slot projection: %+v %v", slot, err)
	}
}
func baseObject(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(paneltest.Bytes(t, paneltest.Verdict("APPROVE")), &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func encoded(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func refusalFixtures(t *testing.T) []struct {
	name, pointer string
	edit          func(map[string]any)
} {
	return []struct {
		name, pointer string
		edit          func(map[string]any)
	}{
		{"missing status", "/status", func(m map[string]any) { delete(m, "status") }},
		{"unknown verdict", "/verdict", func(m map[string]any) { m["verdict"] = "MAYBE" }},
		{"void reason missing", "/void_reason", func(m map[string]any) { m["verdict"] = "VOID" }},
		{"void reason forbidden", "/void_reason", func(m map[string]any) { m["void_reason"] = "x" }},
		{"unknown top", "/extra", func(m map[string]any) { m["extra"] = 1 }},
		{"unknown item", "/concrete_changes_required/0/extra", func(m map[string]any) {
			m["verdict"] = "REQUEST_CHANGES"
			m["concrete_changes_required"] = []any{map[string]any{"text": "x", "basis": "traced", "extra": 1}}
		}},
		{"executed no command", "/concrete_changes_required/0/command", func(m map[string]any) {
			m["verdict"] = "REQUEST_CHANGES"
			m["concrete_changes_required"] = []any{map[string]any{"text": "x", "basis": "executed"}}
		}},
		{"command out of range", "/concrete_changes_required/0/command", func(m map[string]any) {
			m["verdict"] = "REQUEST_CHANGES"
			m["concrete_changes_required"] = []any{map[string]any{"text": "x", "basis": "executed", "command": 5}}
			m["commands_run"] = []any{map[string]any{"cmd": "a", "exit": 0, "tail": ""}, map[string]any{"cmd": "b", "exit": 1, "tail": ""}}
		}},
		{"huge executed command", "/concrete_changes_required/0/command", func(m map[string]any) {
			m["verdict"] = "REQUEST_CHANGES"
			m["concrete_changes_required"] = []any{map[string]any{"text": "x", "basis": "executed", "command": json.Number(strings.Repeat("9", 40))}}
		}},
		{"traced command", "/concrete_changes_required/0/command", func(m map[string]any) {
			m["verdict"] = "REQUEST_CHANGES"
			m["concrete_changes_required"] = []any{map[string]any{"text": "x", "basis": "traced", "command": 0}}
		}},
		{"approve with item", "/concrete_changes_required", func(m map[string]any) {
			m["concrete_changes_required"] = []any{map[string]any{"text": "x", "basis": "traced"}}
		}},
		{"RC no items", "/concrete_changes_required", func(m map[string]any) { m["verdict"] = "REQUEST_CHANGES" }},
		{"unknown command field", "/commands_run/0/extra", func(m map[string]any) {
			m["commands_run"] = []any{map[string]any{"cmd": "a", "exit": 0, "tail": "", "extra": true}}
		}},
		{"empty rationale", "/rationale", func(m map[string]any) { m["rationale"] = "" }},
	}
}
func TestVerdictV2_Refusals(t *testing.T) {
	for _, tc := range refusalFixtures(t) {
		t.Run(tc.name, func(t *testing.T) {
			m := baseObject(t)
			tc.edit(m)
			err := panel.ValidateVerdictV2(encoded(t, m))
			if err == nil || !strings.HasPrefix(err.Error(), tc.pointer+":") {
				t.Fatalf("want pointer %s, got %v", tc.pointer, err)
			}
		})
	}
}
func TestVerdictV2_TransportLayer(t *testing.T) {
	b := paneltest.Bytes(t, paneltest.Verdict("APPROVE"))
	padding := append(append([]byte{}, b...), []byte(strings.Repeat(" ", panel.MaxVerdictBytes-len(b)+1))...)
	if err := panel.ValidateVerdictV2Schema(padding); err != nil {
		t.Fatal(err)
	}
	if err := panel.ValidateVerdictV2(padding); err == nil || !strings.HasPrefix(err.Error(), "/:") {
		t.Fatalf("padding beyond 1 MiB must name root pointer: %v", err)
	}
	m := baseObject(t)
	m["rationale"] = strings.Repeat("x", panel.MaxVerdictBytes-len(b)+len("reviewed")+1)
	compact := encoded(t, m)
	if len(compact) != panel.MaxVerdictBytes+1 {
		t.Fatalf("size %d", len(compact))
	}
	if err := panel.ValidateVerdictV2(compact); err == nil || !strings.HasPrefix(err.Error(), "/:") {
		t.Fatalf("oversized compact file must name root pointer: %v", err)
	}
	exact := padding[:panel.MaxVerdictBytes]
	if err := panel.ValidateVerdictV2(exact); err != nil {
		t.Fatal(err)
	}
	v := paneltest.Verdict("APPROVE")
	v.CommandsRun = []panel.CommandRun{{Cmd: "test", Exit: "0", Tail: strings.Repeat("é", 2048)}}
	if err := panel.ValidateVerdictV2(paneltest.Bytes(t, v)); err != nil {
		t.Fatal(err)
	}
	v.CommandsRun[0].Tail += "x"
	err := panel.ValidateVerdictV2(paneltest.Bytes(t, v))
	if err == nil || !strings.HasPrefix(err.Error(), "/commands_run/0/tail:") {
		t.Fatalf("4097 byte tail: %v", err)
	}
}

// schemaAccepts interprets the published JSON document independently. It does
// not call the Go schema validator and covers every keyword the document emits.
func schemaAccepts(s map[string]any, v any) bool {
	switch s["type"] {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return false
		}
		props := s["properties"].(map[string]any)
		for _, raw := range s["required"].([]any) {
			if _, ok := m[raw.(string)]; !ok {
				return false
			}
		}
		for k, x := range m {
			raw, ok := props[k]
			if !ok {
				if s["additionalProperties"] == false {
					return false
				}
				continue
			}
			if !schemaAccepts(raw.(map[string]any), x) {
				return false
			}
		}
	case "array":
		a, ok := v.([]any)
		if !ok {
			return false
		}
		if max, ok := s["maxItems"].(float64); ok && float64(len(a)) > max {
			return false
		}
		for _, x := range a {
			if !schemaAccepts(s["items"].(map[string]any), x) {
				return false
			}
		}
	case "string":
		x, ok := v.(string)
		if !ok {
			return false
		}
		if min, ok := s["minLength"].(float64); ok && float64(utf8.RuneCountInString(x)) < min {
			return false
		}
		if pattern, ok := s["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(x) {
			return false
		}
	case "integer":
		x, ok := v.(float64)
		if !ok || math.Trunc(x) != x {
			return false
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return false
		}
	default:
		panic(fmt.Sprintf("unsupported type %v", s["type"]))
	}
	if choices, ok := s["enum"].([]any); ok {
		found := false
		for _, x := range choices {
			if x == v {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func TestVerdictV2_SchemaMatchesValidator(t *testing.T) {
	raw, err := panel.VerdictV2Schema()
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"1 MiB", "4096", "void_reason", "executed", "cardinality"} {
		if !strings.Contains(schema["description"].(string), word) {
			t.Fatalf("missing relation description %s", word)
		}
	}
	check := func(t *testing.T, m map[string]any, want bool) {
		t.Helper()
		b := encoded(t, m)
		var decoded any
		_ = json.Unmarshal(b, &decoded)
		published := schemaAccepts(schema, decoded)
		native := panel.ValidateVerdictV2Schema(b) == nil
		if published != want || native != want {
			t.Fatalf("published=%v native=%v want=%v: %s", published, native, want, b)
		}
		if want && panel.ValidateVerdictV2(b) != nil {
			t.Fatalf("valid schema-layer fixture refused: %v", panel.ValidateVerdictV2(b))
		}
	}
	check(t, baseObject(t), true)
	for _, key := range []string{"schema", "slot", "round", "reviewed_sha", "status", "verdict", "model", "rationale", "concrete_changes_required", "commands_run"} {
		t.Run("required "+key, func(t *testing.T) { m := baseObject(t); delete(m, key); check(t, m, false) })
	}
	for key, value := range map[string]any{"schema": 1, "slot": 42, "round": 1.5, "reviewed_sha": strings.Repeat("A", 40), "status": "later", "verdict": "MAYBE", "model": "", "rationale": nil, "concrete_changes_required": nil, "commands_run": nil, "hard_block": "yes", "findings": []any{4}, "inputs_read": false, "extra": true} {
		t.Run("type or enum "+key, func(t *testing.T) { m := baseObject(t); m[key] = value; check(t, m, false) })
	}
	for _, tc := range refusalFixtures(t) {
		if tc.name != "missing status" && tc.name != "unknown verdict" && tc.name != "unknown top" && tc.name != "unknown item" && tc.name != "unknown command field" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) { m := baseObject(t); tc.edit(m); check(t, m, false) })
	}
	for _, n := range []int{100, 101} {
		t.Run(fmt.Sprintf("items %d", n), func(t *testing.T) {
			m := baseObject(t)
			m["verdict"] = "REQUEST_CHANGES"
			items := make([]any, n)
			for i := range items {
				items[i] = map[string]any{"text": "x", "basis": "traced"}
			}
			m["concrete_changes_required"] = items
			check(t, m, n == 100)
		})
	}
	for _, n := range []int{200, 201} {
		t.Run(fmt.Sprintf("commands %d", n), func(t *testing.T) {
			m := baseObject(t)
			items := make([]any, n)
			for i := range items {
				items[i] = map[string]any{"cmd": "x", "exit": 0, "tail": ""}
			}
			m["commands_run"] = items
			check(t, m, n == 200)
		})
	}
}
