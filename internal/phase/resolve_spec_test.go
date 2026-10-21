package phase

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
)

func TestResolveSpec_PlainAndLetteredBothPresent(t *testing.T) {
	epics := []EpicInfo{
		{ID: "epic-plain", Metadata: map[string]interface{}{"spec_id": "129-x"}},
		{ID: "epic-letter", Title: "[SPEC 129b-y] letter", Metadata: map[string]interface{}{"spec_id": "129b-y", "spec_num": 129, "spec_title": "x"}},
	}
	t.Cleanup(SetListJSONForTest(func(...string) ([]byte, error) { return json.Marshal(epics) }))
	for _, epic := range epics {
		id, err := ResolveSpec(epic)
		if err != nil || id != epic.Metadata["spec_id"] {
			t.Fatalf("%s: %s %v", epic.ID, id, err)
		}
		got, err := NewCache().FindEpicBySpecID(id)
		if err != nil || got != epic.ID {
			t.Fatalf("lookup %s: %s %v", id, got, err)
		}
	}
	if err := idvalidate.SpecID("008b-human-gates"); err != nil {
		t.Fatal(err)
	}
}

func TestResolveSpec_LegacyPairRepairResolves(t *testing.T) {
	e := EpicInfo{ID: "epic-letter", Title: "[SPEC 008b-human-gates] gates", Metadata: map[string]interface{}{"spec_num": 8, "spec_title": "human-gates"}}
	_, err := ResolveSpec(e)
	if err == nil || !strings.Contains(err.Error(), "recovery: bd update epic-letter --set-metadata spec_id=008b-human-gates") {
		t.Fatalf("repair: %v", err)
	}
	e.Metadata["spec_id"] = "008b-human-gates"
	id, err := ResolveSpec(e)
	if err != nil || id != "008b-human-gates" {
		t.Fatalf("repair failed: %s %v", id, err)
	}
}
func TestResolveSpec_TitleWithoutIDRefuses(t *testing.T) {
	_, err := ResolveSpec(EpicInfo{ID: "epic-old", Title: "[PLAN old] work"})
	if !errors.Is(err, ErrNoSpecID) || !strings.Contains(err.Error(), "use the spec directory name as the ID") || !strings.Contains(err.Error(), "recovery: bd update epic-old --set-metadata spec_id=") {
		t.Fatalf("missing ID: %v", err)
	}
}
func TestResolveSpec_SpecNumZeroAbsent(t *testing.T) {
	e := EpicInfo{ID: "epic-zero", Title: "[SPEC 008b-human-gates] gates", Metadata: map[string]interface{}{"spec_num": 0, "spec_title": "wrong"}}
	id, err := ResolveSpec(e)
	if err != nil || id != "008b-human-gates" {
		t.Fatalf("zero: %s %v", id, err)
	}
	e.Title = "no ID"
	if _, err := ResolveSpec(e); !errors.Is(err, ErrNoSpecID) {
		t.Fatalf("zero is not absent: %v", err)
	}
}
func TestResolveSpec_InvalidSpecIDRefuses(t *testing.T) {
	_, err := ResolveSpec(EpicInfo{ID: "epic-bad", Title: "[SPEC 008b-human-gates] gates", Metadata: map[string]interface{}{"spec_id": "../bad"}})
	if !errors.Is(err, ErrMalformedSpecMetadata) || !strings.Contains(err.Error(), "recovery:") {
		t.Fatalf("invalid ID fell through: %v", err)
	}
}
func TestResolveSpec_TwoEpicsScopedRefusal(t *testing.T) {
	epics := []EpicInfo{
		{ID: "epic-a", Status: "open", Metadata: map[string]interface{}{"spec_id": "129-x"}},
		{ID: "epic-b", Status: "closed", Title: "[SPEC 129-x] old"},
		{ID: "epic-good", Status: "open", Metadata: map[string]interface{}{"spec_id": "130-y", "mindspec_phase": "plan"}},
		{ID: "epic-historical", Status: "open", Title: "[PLAN 008b-human-gates] old"},
	}
	calls := 0
	t.Cleanup(SetListJSONForTest(func(...string) ([]byte, error) { calls++; return json.Marshal(epics) }))
	c := NewCache()
	for i := 0; i < 2; i++ {
		_, err := c.FindEpicBySpecID("129-x")
		if err == nil || !strings.Contains(err.Error(), "epic-a, epic-b") {
			t.Fatalf("duplicates: %v", err)
		}
	}
	if calls != 1 {
		t.Fatalf("duplicate refusal not cached: %d", calls)
	}
	if id, err := c.FindEpicBySpecID("130-y"); err != nil || id != "epic-good" {
		t.Fatalf("unrelated spec blocked: %s %v", id, err)
	}
	active, err := DiscoverActiveSpecsWithCache(c)
	if err != nil || len(active) != 1 || active[0].EpicID != "epic-good" {
		t.Fatalf("listing: %+v %v", active, err)
	}
}
func TestCacheFindEpicBySpecID_SpecIDOnlyLettered(t *testing.T) {
	t.Cleanup(SetListJSONForTest(func(...string) ([]byte, error) {
		return []byte(`[{"id":"epic-letter","metadata":{"spec_id":"008b-human-gates"}}]`), nil
	}))
	id, err := NewCache().FindEpicBySpecID("008b-human-gates")
	if err != nil || id != "epic-letter" {
		t.Fatalf("lettered: %s %v", id, err)
	}
}
func TestCacheFindEpicBySpecID_DuplicateWithClosedEpicRefuses(t *testing.T) {
	TestResolveSpec_TwoEpicsScopedRefusal(t)
}

func TestResolveSpec_TitleOnlyLetteredAndEmptyLegacyTitle(t *testing.T) {
	for _, meta := range []map[string]interface{}{nil, {"spec_num": 8, "spec_title": ""}} {
		id, err := ResolveSpec(EpicInfo{ID: "epic-letter", Title: "[SPEC 008b-human-gates] gates", Metadata: meta})
		if err != nil || id != "008b-human-gates" {
			t.Fatalf("title full ID: %s %v", id, err)
		}
	}
}

func TestResolveSpec_StoredFullIDDecides(t *testing.T) {
	id, err := ResolveSpec(EpicInfo{ID: "epic-letter", Title: "[SPEC 129-x] base", Metadata: map[string]interface{}{"spec_id": "129b-y", "spec_num": 129, "spec_title": "x"}})
	if err != nil || id != "129b-y" {
		t.Fatalf("stored full ID did not decide: %s %v", id, err)
	}
}

func TestCacheFindEpicBySpecID_LegacyConflictScoped(t *testing.T) {
	epics := []EpicInfo{
		{ID: "epic-old", Title: "[SPEC 008b-human-gates] gates", Metadata: map[string]interface{}{"spec_num": 8, "spec_title": "human-gates"}},
		{ID: "epic-good", Metadata: map[string]interface{}{"spec_id": "129-x"}},
	}
	t.Cleanup(SetListJSONForTest(func(...string) ([]byte, error) { return json.Marshal(epics) }))
	c := NewCache()
	if _, err := c.FindEpicBySpecID("008-human-gates"); err == nil || !strings.Contains(err.Error(), "--set-metadata spec_id=008b-human-gates") {
		t.Fatalf("plain ID selected through conflicting pair: %v", err)
	}
	if id, err := c.FindEpicBySpecID("129-x"); err != nil || id != "epic-good" {
		t.Fatalf("unrelated spec refused: %s %v", id, err)
	}
}
