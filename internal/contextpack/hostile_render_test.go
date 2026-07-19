package contextpack

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
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

// TestRenderBeadContextHostileTitleEscaped pins AC-16: RenderBeadContext's
// single-line Title/ID/file_paths fields are escaped/idrender'd, while
// multi-line fenced payload (Description/AcceptanceCriteria/Design) is
// left byte-identical (it is not a single-line render position). The
// budgeter's renderHeader (BuildBead's entry point) shares the same
// discipline.
func TestRenderBeadContextHostileTitleEscaped(t *testing.T) {
	hostileTitle := "[074-test] Bead 1" + hostileFieldSuffix
	malformedID := "120-x;evil"

	t.Run("RenderBeadContext", func(t *testing.T) {
		restore := SetBeadShowForTest(func(args ...string) ([]byte, error) {
			entry := []beadShowEntry{{
				ID:    malformedID,
				Title: hostileTitle,
				Metadata: map[string]interface{}{
					"file_paths": []interface{}{"internal/widget" + hostileFieldSuffix + ".go"},
				},
			}}
			return json.Marshal(entry)
		})
		defer restore()

		rendered, err := RenderBeadContext(malformedID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertCleanRender(t, rendered)
		if !strings.Contains(rendered, strconv.Quote(malformedID)) {
			t.Errorf("expected the malformed bead id forced-quoted, got: %s", rendered)
		}
	})

	t.Run("RenderBeadContext clean-fixture byte-identity", func(t *testing.T) {
		restore := SetBeadShowForTest(func(args ...string) ([]byte, error) {
			entry := []beadShowEntry{{ID: "mindspec-9cyu.1", Title: "Clean title"}}
			return json.Marshal(entry)
		})
		defer restore()

		rendered, err := RenderBeadContext("mindspec-9cyu.1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(rendered, "**Bead**: mindspec-9cyu.1") {
			t.Errorf("clean bead id must render byte-identical, got: %s", rendered)
		}
		if !strings.Contains(rendered, "Clean title") {
			t.Errorf("clean title must render byte-identical, got: %s", rendered)
		}
	})

	t.Run("renderHeader (budgeter entry point)", func(t *testing.T) {
		out := renderHeader(hostileTitle, malformedID)
		assertCleanRender(t, out)
		if !strings.Contains(out, strconv.Quote(malformedID)) {
			t.Errorf("expected the malformed bead id forced-quoted, got: %s", out)
		}

		cleanOut := renderHeader("Clean title", "mindspec-9cyu.1")
		if cleanOut != "# Bead Context: Clean title\n**Bead**: mindspec-9cyu.1\n\n" {
			t.Errorf("clean renderHeader must be byte-identical, got: %q", cleanOut)
		}
	})
}
