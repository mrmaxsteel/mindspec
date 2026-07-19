// ratchet_template_test.go — spec 120 R6 scan (d): the instruct
// template classification (TestInstructTemplateFieldClassification).
//
// Every RENDERING interpolation in internal/instruct/templates/*.md —
// {{.Field}}, {{termsafe .Field}}, {{.Field | shellsafe}}, and the
// range-item {{termsafe .}} — is classified TWO-WAY into exactly one
// of four classes:
//
//	spine-gated ID      — the value is a validated spec/bead ID
//	                      (D1/D2/waist spine), rendered raw by class
//	                      contract.
//	emitter-gated path  — a composed path routed through the
//	                      shell-safe cd emitter (Bead 3's shellsafe).
//	termsafe free text  — agent-writable free text routed {{termsafe}}.
//	fenced payload      — labeled markdown payload (116's inherited
//	                      persuasion Non-Goal); .SpecGoal is PINNED
//	                      here.
//
// A new template field fails until classified; a stale classification
// entry fails when its interpolation disappears.
package lint

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

type tmplClass string

const (
	classSpineGatedID    tmplClass = "spine-gated ID"
	classEmitterGatedPth tmplClass = "emitter-gated path"
	classTermsafeText    tmplClass = "termsafe-routed free text"
	classFencedPayload   tmplClass = "fenced payload"
)

// templateFieldClassification is the audited two-way classification,
// keyed "<template>.md <action>".
var templateFieldClassification = map[string]tmplClass{
	// ambiguous.md — the active-spec table. SpecID comes from
	// DiscoverActiveSpecsWithCache, D1-gated at SpecIDFromMetadata
	// (malformed epics are skipped with an escaped warning).
	"ambiguous.md {{.SpecID}}": classSpineGatedID,
	// Mode is a framework-authored enum (state.Mode), not
	// agent-writable — classified with the spine (validated,
	// renders raw).
	"ambiguous.md {{.Mode}}": classSpineGatedID,

	// idle.md — lifecycle findings are agent-influenced free text,
	// routed through the termsafe template func per item.
	"idle.md {{termsafe .}}": classTermsafeText,

	// implement.md
	"implement.md {{.ActiveSpec}}": classSpineGatedID,
	"implement.md {{.ActiveBead}}": classSpineGatedID,
	// .SpecGoal: agent-writable markdown payload — PINNED as fenced
	// payload under 116's inherited persuasion Non-Goal (AC-14).
	"implement.md {{.SpecGoal}}": classFencedPayload,
	// ActiveWorktreeDisplay is the DISPLAY-only worktree line: it
	// routes termsafe.Escape in Go (Bead 5's display split —
	// instruct.go assigns `ActiveWorktreeDisplay:
	// termsafe.Escape(mc.ActiveWorktree)`), NOT the shellsafe cd
	// emitter — so its class is termsafe-routed free text (S2-F1).
	// The go-side routing claim is structurally verified by the
	// goside_termsafe_routing_verified subtest below.
	"implement.md {{.ActiveWorktreeDisplay}}": classTermsafeText,
	// The executable cd operand routes the shell-safe emitter.
	"implement.md {{.ActiveWorktree | shellsafe}}": classEmitterGatedPth,

	// plan.md / review.md / spec.md
	"plan.md {{.ActiveSpec}}":   classSpineGatedID,
	"plan.md {{.SpecGoal}}":     classFencedPayload,
	"review.md {{.ActiveSpec}}": classSpineGatedID,
	"review.md {{.SpecGoal}}":   classFencedPayload,
	"spec.md {{.ActiveSpec}}":   classSpineGatedID,
	"spec.md {{.SpecGoal}}":     classFencedPayload,
}

// goSideTermsafeRouted: interpolations whose termsafe routing happens
// in GO (the field value is termsafe.Escape'd before the template ever
// sees it) rather than via the {{termsafe}} template func. Each entry
// maps the interpolation key to the exact source shape that performs
// the routing in internal/instruct/instruct.go; the
// goside_termsafe_routing_verified subtest fails if the shape
// disappears, so the exemption cannot go stale.
var goSideTermsafeRouted = map[string]string{
	"implement.md {{.ActiveWorktreeDisplay}}": "ActiveWorktreeDisplay: termsafe.Escape(",
}

var tmplActionRe = regexp.MustCompile(`\{\{-?\s*[^}]*?\s*-?\}\}`)

// renderingActions extracts the RENDERING actions from template text:
// control-flow actions (if/else/end/range/with) are not renders and
// are skipped; everything else ({{.X}}, {{termsafe .X}}, {{.X |
// shellsafe}}, {{termsafe .}}) is.
func renderingActions(src string) []string {
	var out []string
	for _, raw := range tmplActionRe.FindAllString(src, -1) {
		inner := strings.TrimPrefix(raw, "{{")
		inner = strings.TrimSuffix(inner, "}}")
		inner = strings.Trim(inner, "- \t")
		if inner == "" {
			continue
		}
		word := strings.Fields(inner)[0]
		switch word {
		case "if", "else", "end", "range", "with", "template", "block", "define":
			continue
		}
		out = append(out, "{{"+inner+"}}")
	}
	return out
}

// scanTemplateActions returns every rendering action across the
// instruct templates, keyed "<file> <action>", with counts.
func scanTemplateActions(t *testing.T, dir string) map[string]int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read templates dir: %v", err)
	}
	found := map[string]int{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read template %s: %v", e.Name(), err)
		}
		for _, act := range renderingActions(string(src)) {
			found[e.Name()+" "+act]++
		}
	}
	return found
}

// classifyTemplates performs the two-way comparison; extracted so the
// unclassified-field fixture can drive it directly.
func classifyTemplates(found map[string]int, classified map[string]tmplClass) []string {
	var problems []string
	for k := range found {
		if _, ok := classified[k]; !ok {
			problems = append(problems, "UNCLASSIFIED template interpolation (classify it into {spine-gated ID, emitter-gated path, termsafe free text, fenced payload}): "+k)
		}
	}
	for k := range classified {
		if _, ok := found[k]; !ok {
			problems = append(problems, "STALE classification entry (interpolation no longer present): "+k)
		}
	}
	// Class-consistency checks: a termsafe-classified action must
	// actually route termsafe; a shellsafe-suffixed action must be
	// emitter-gated.
	for k, class := range classified {
		if _, ok := found[k]; !ok {
			continue
		}
		hasTermsafe := strings.Contains(k, "termsafe ")
		hasShellsafe := strings.Contains(k, "| shellsafe")
		if class == classTermsafeText && !hasTermsafe && goSideTermsafeRouted[k] == "" {
			problems = append(problems, "MISROUTED: classified termsafe free text but routes neither {{termsafe}} nor an audited go-side termsafe.Escape (goSideTermsafeRouted): "+k)
		}
		if hasTermsafe && class != classTermsafeText {
			problems = append(problems, "MISROUTED: routes {{termsafe}} but classified "+string(class)+": "+k)
		}
		if hasShellsafe && class != classEmitterGatedPth {
			problems = append(problems, "MISROUTED: routes shellsafe but classified "+string(class)+": "+k)
		}
	}
	return problems
}

// TestInstructTemplateFieldClassification is spec 120 R6 scan (d)
// (AC-14): every instruct-template interpolation classified two-way;
// .SpecGoal pinned as fenced payload.
func TestInstructTemplateFieldClassification(t *testing.T) {
	root := repoRootDir(t)
	dir := filepath.Join(root, "internal", "instruct", "templates")
	found := scanTemplateActions(t, dir)
	if len(found) == 0 {
		t.Fatal("no template interpolations found — the scanner has regressed")
	}
	// Census pin (S2-F2): 14 audited rendering interpolations across
	// the instruct templates (incl. ambiguous.md's two). Update BOTH
	// the classification map and this count when templates change.
	if len(templateFieldClassification) != 14 {
		t.Errorf("template classification census drift: map has %d entries, the audited census is 14", len(templateFieldClassification))
	}
	failOnProblems(t, "scan (d) template classification",
		classifyTemplates(found, templateFieldClassification))

	t.Run("goside_termsafe_routing_verified", func(t *testing.T) {
		// Structural verification of the go-side termsafe exemptions:
		// each goSideTermsafeRouted entry must (a) classify as
		// termsafe-routed free text and (b) still have its named
		// termsafe.Escape assignment shape in
		// internal/instruct/instruct.go — deleting the Escape routing
		// turns this RED.
		src, err := os.ReadFile(filepath.Join(root, "internal", "instruct", "instruct.go"))
		if err != nil {
			t.Fatalf("read instruct.go: %v", err)
		}
		for k, shape := range goSideTermsafeRouted {
			if templateFieldClassification[k] != classTermsafeText {
				t.Errorf("goSideTermsafeRouted entry %q must classify as termsafe-routed free text, got %q", k, templateFieldClassification[k])
			}
			if !strings.Contains(string(src), shape) {
				t.Errorf("go-side termsafe routing shape %q for %q no longer present in internal/instruct/instruct.go — reroute or reclassify", shape, k)
			}
		}
	})

	t.Run("specgoal_pinned_fenced", func(t *testing.T) {
		// The .SpecGoal pin (116's inherited persuasion Non-Goal):
		// every template that renders .SpecGoal classifies it as
		// fenced payload, and at least one such interpolation exists.
		n := 0
		for k, class := range templateFieldClassification {
			if strings.Contains(k, "{{.SpecGoal}}") {
				n++
				if class != classFencedPayload {
					t.Errorf(".SpecGoal must be fenced payload, got %s for %s", class, k)
				}
			}
		}
		if n == 0 {
			t.Error("no .SpecGoal interpolation classified — the pin is vacuous")
		}
	})

	t.Run("fixture_unclassified_field_flagged", func(t *testing.T) {
		// Negative fixture: a template carrying a brand-new field
		// must surface as UNCLASSIFIED.
		_, thisFile, _, _ := runtime.Caller(0)
		td := filepath.Join(filepath.Dir(thisFile), "testdata")
		src, err := os.ReadFile(filepath.Join(td, "ratchet_template_newfield.md.txt"))
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		ffound := map[string]int{}
		for _, act := range renderingActions(string(src)) {
			ffound["newfield.md "+act]++
		}
		problems := classifyTemplates(ffound, templateFieldClassification)
		assertProblemPresent(t, problems, "UNCLASSIFIED", "{{.BrandNewField}}")
	})

	t.Run("fixture_stale_classification_flagged", func(t *testing.T) {
		synth := map[string]tmplClass{"gone.md {{.Deleted}}": classTermsafeText}
		problems := classifyTemplates(map[string]int{}, synth)
		assertProblemPresent(t, problems, "STALE", "gone.md")
	})
}
