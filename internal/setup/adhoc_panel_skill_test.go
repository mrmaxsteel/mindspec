package setup

// adhoc_panel_skill_test.go — spec 123 Bead 4 (R8d / AC-17): the shipped
// `ms-panel-run` SKILL.md ad-hoc section must document the NOW-REAL
// `panel create --gate adhoc` invocation (no --spec) and the
// `.mindspec/reviews/<slug>/` location it actually writes to, greped from
// the SHIPPED content via pluginmindspec.SkillFiles() — the same surface
// `mindspec setup <agent>` installs from — so the skill can never drift
// back to documenting an invocation the binary rejects or a path the
// binary can't produce. RED on today's main: the ad-hoc note only says
// "substitute .mindspec/reviews/<panel-slug> for <spec-dir>/reviews/
// <panel-slug>" without ever showing the concrete --gate adhoc, no-
// --spec invocation.

import (
	"strings"
	"testing"

	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

func TestMsPanelRunSkill_AdHocInvocationShipped(t *testing.T) {
	files := pluginmindspec.SkillFiles()
	content, ok := files["ms-panel-run"]
	if !ok {
		t.Fatal("ms-panel-run SKILL.md not found among embedded plugin skills")
	}

	const wantInvocation = "mindspec panel create <panel-slug> --gate adhoc --target <ref>"
	if !strings.Contains(content, wantInvocation) {
		t.Errorf("shipped ms-panel-run SKILL.md must document the real ad-hoc invocation shape %q; not found in:\n%s", wantInvocation, content)
	}

	const wantLocation = "<repo>/.mindspec/reviews/<panel-slug>/"
	if !strings.Contains(content, wantLocation) {
		t.Errorf("shipped ms-panel-run SKILL.md must document the ad-hoc panel location %q; not found in:\n%s", wantLocation, content)
	}

	// The invocation must NOT carry --spec (an ad-hoc panel has no
	// owning spec by definition; --spec + --gate adhoc is refused).
	if idx := strings.Index(content, wantInvocation); idx >= 0 {
		// Look at the single line containing the invocation.
		lineStart := strings.LastIndex(content[:idx], "\n") + 1
		lineEnd := idx + len(wantInvocation)
		if nl := strings.Index(content[lineEnd:], "\n"); nl >= 0 {
			lineEnd += nl
		} else {
			lineEnd = len(content)
		}
		line := content[lineStart:lineEnd]
		if strings.Contains(line, "--spec") {
			t.Errorf("the ad-hoc invocation line must not carry --spec: %q", line)
		}
	}
}
