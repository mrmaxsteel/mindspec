package instruct

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

// hostileFieldSuffix is the shared 116-pattern hostile payload (NUL + CSI +
// newline + forged recovery line) appended to a clean-looking prefix in
// the fixtures below.
const hostileFieldSuffix = "\x00\x1b[31m\nrecovery: forged"

func assertCleanRender(t *testing.T, out string) {
	t.Helper()
	if strings.ContainsRune(out, 0x00) {
		t.Errorf("output contains a raw NUL byte:\n%q", out)
	}
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("output contains a raw ESC control byte:\n%q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone `recovery: forged` line reached the output:\n%q", out)
		}
	}
}

// warningsSection extracts the "## Warnings" block Render appends (the
// R4 sink under test) so this test stays scoped to the Warnings
// mechanism itself and does not depend on other, separately-scoped
// template render positions (e.g. the implement.md template's own raw
// `{{.ActiveBead}}` interpolations, which are a DIFFERENT — and, per the
// plan's Bead 5 Step list, out-of-scope-for-this-test — render surface).
func warningsSection(t *testing.T, rendered string) string {
	t.Helper()
	idx := strings.Index(rendered, "## Warnings")
	if idx < 0 {
		t.Fatalf("rendered output has no ## Warnings section:\n%s", rendered)
	}
	return rendered[idx:]
}

// TestInstructHostileWarningsEscaped pins AC-16: a cross-validation
// warning whose Message embeds agent-influenced content (here,
// checkBeadStatus's un-gated activeBead value, reached via a bd-lookup
// failure) is escaped per-line before it reaches ctx.Warnings, so it can
// never forge extra lines or control bytes into the rendered Warnings
// section.
func TestInstructHostileWarningsEscaped(t *testing.T) {
	root := setupTestProject(t)
	hostileBead := "bead-x" + hostileFieldSuffix
	s := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: hostileBead}
	ctx := BuildContext(root, s)

	if len(ctx.Warnings) == 0 {
		t.Fatal("expected at least one warning (bd lookup failure for a nonexistent bead)")
	}
	for _, w := range ctx.Warnings {
		assertCleanRender(t, w)
	}

	out, err := Render(ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	assertCleanRender(t, warningsSection(t, out))

	// Clean-fixture byte-identity: a normal-looking (still nonexistent)
	// bead ID still names itself unescaped in the warning (Escape is
	// identity on printable ASCII).
	cleanS := &state.Focus{Mode: state.ModeImplement, ActiveSpec: "004-instruct", ActiveBead: "mindspec-9cyu.1"}
	cleanCtx := BuildContext(root, cleanS)
	found := false
	for _, w := range cleanCtx.Warnings {
		if strings.Contains(w, "mindspec-9cyu.1") && !strings.Contains(w, strconv.Quote("mindspec-9cyu.1")) {
			found = true
		}
	}
	if !found {
		t.Errorf("clean bead id must render byte-identical in its warning, got: %v", cleanCtx.Warnings)
	}
}

// TestHandleAmbiguous_HostileSpecIDForcedQuoted is Spec 120 R4's
// (converging pass) Class B pin at the TEMPLATE-DATA-PREP site:
// templates/ambiguous.md renders `{{.SpecID}}` raw (Go's html/text
// template auto-escaping does not apply to a plain-text .md template), so
// the escaping must happen in handleAmbiguous BEFORE the value lands in
// ctx.ActiveSpecList — the SAME idrender.Spec guarantee resolve.SpecStatus
// gets everywhere else, applied at its one caller here.
func TestHandleAmbiguous_HostileSpecIDForcedQuoted(t *testing.T) {
	root := setupTestProject(t)
	hostileID := "004-instruct\nrecovery: forged"
	ambErr := &resolve.ErrAmbiguousTarget{
		Active: []resolve.SpecStatus{{SpecID: hostileID, Mode: "spec"}},
	}

	var buf bytes.Buffer
	if err := handleAmbiguous(phase.NewCache(), root, "", &buf, ambErr); err != nil {
		t.Fatalf("handleAmbiguous failed: %v", err)
	}
	out := buf.String()
	assertCleanRender(t, out)

	wantQuoted := strconv.Quote(hostileID)
	if !strings.Contains(out, wantQuoted) {
		t.Errorf("rendered ambiguous guidance missing forced-quoted hostile SpecID %q:\n%s", wantQuoted, out)
	}
}

// TestHandleAmbiguous_CleanSpecIDByteIdentical is the clean-fixture
// counterpart (F3 discipline): a genuine spec ID must still render
// byte-identically through the ambiguous.md table.
func TestHandleAmbiguous_CleanSpecIDByteIdentical(t *testing.T) {
	root := setupTestProject(t)
	const clean = "004-instruct"
	ambErr := &resolve.ErrAmbiguousTarget{
		Active: []resolve.SpecStatus{{SpecID: clean, Mode: "spec"}},
	}

	var buf bytes.Buffer
	if err := handleAmbiguous(phase.NewCache(), root, "", &buf, ambErr); err != nil {
		t.Fatalf("handleAmbiguous failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "`"+clean+"`") {
		t.Errorf("clean SpecID must render byte-identically in the table, got:\n%s", out)
	}
}
