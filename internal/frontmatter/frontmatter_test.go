package frontmatter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatus_PortedFromState(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"draft", "---\nstatus: Draft\nspec_id: 004\n---\n# Plan\n", "Draft"},
		{"approved", "---\nstatus: Approved\napproved_at: 2026-02-12\n---\n# Plan\n", "Approved"},
		{"no frontmatter", "# Plan\n\nSome content\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Status([]byte(tt.content))
			if got != tt.want {
				t.Errorf("Status() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatus_PortedFromSpeclist(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"draft", "---\nstatus: Draft\n---\n# Spec", "Draft"},
		{"approved with quoted date", "---\nstatus: Approved\napproved_at: \"2026-01-01\"\n---\n# Spec", "Approved"},
		{"no frontmatter", "# Spec\nSome content", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Status([]byte(tt.content))
			if got != tt.want {
				t.Errorf("Status() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatus_Quoting(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"unquoted", "---\nstatus: Approved\n---\n", "Approved"},
		{"double quoted", "---\nstatus: \"Approved\"\n---\n", "Approved"},
		{"single quoted", "---\nstatus: 'Approved'\n---\n", "Approved"},
		{"lowercase", "---\nstatus: approved\n---\n", "approved"},
		{"uppercase", "---\nstatus: APPROVED\n---\n", "APPROVED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Status([]byte(tt.content))
			if got != tt.want {
				t.Errorf("Status() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatus_NoCloser(t *testing.T) {
	// Strict mode: missing closing fence is malformed, returns "".
	content := "---\nstatus: Approved\n# Plan\n"
	got := Status([]byte(content))
	if got != "" {
		t.Errorf("Status() = %q, want \"\" (no closer is malformed)", got)
	}
}

func TestStatus_MultiLineValue(t *testing.T) {
	// YAML folded scalar — trims whitespace including trailing newline.
	content := "---\nstatus: >\n  Approved\n---\n"
	got := Status([]byte(content))
	if got != "Approved" {
		t.Errorf("Status() = %q, want %q", got, "Approved")
	}
}

func TestStatus_WithInlineComment(t *testing.T) {
	content := "---\nstatus: Approved # set by approve cmd\n---\n"
	got := Status([]byte(content))
	if got != "Approved" {
		t.Errorf("Status() = %q, want %q", got, "Approved")
	}
}

func TestStatus_NoStatusField(t *testing.T) {
	content := "---\nspec_id: 004\n---\n# Plan\n"
	got := Status([]byte(content))
	if got != "" {
		t.Errorf("Status() = %q, want \"\"", got)
	}
}

func TestStatus_EmptyDocument(t *testing.T) {
	if got := Status([]byte("")); got != "" {
		t.Errorf("Status(\"\") = %q, want \"\"", got)
	}
}

func TestStatus_OnlyOpener(t *testing.T) {
	if got := Status([]byte("---\n")); got != "" {
		t.Errorf("Status(\"---\\n\") = %q, want \"\"", got)
	}
}

func TestStatus_LeadingBOM(t *testing.T) {
	// We don't strip BOMs; a BOM before --- means the opener line is not "---".
	bom := []byte{0xEF, 0xBB, 0xBF}
	content := append(bom, []byte("---\nstatus: Approved\n---\n")...)
	got := Status(content)
	if got != "" {
		t.Errorf("Status(BOM+...) = %q, want \"\" (BOM not stripped)", got)
	}
}

func TestField_Basic(t *testing.T) {
	content := "---\nstatus: Draft\nspec_id: \"004\"\n---\n# Plan\n"
	v, ok := Field([]byte(content), "spec_id")
	if !ok {
		t.Fatal("Field(spec_id) ok=false, want true")
	}
	if v != "004" {
		t.Errorf("Field(spec_id) = %q, want %q", v, "004")
	}
}

func TestField_NonStringScalar(t *testing.T) {
	// Non-string scalars (numbers, booleans) get stringified.
	content := "---\ncount: 42\n---\n"
	v, ok := Field([]byte(content), "count")
	if !ok {
		t.Fatal("Field(count) ok=false, want true")
	}
	if v != "42" {
		t.Errorf("Field(count) = %q, want %q", v, "42")
	}
}

func TestField_NonScalar(t *testing.T) {
	content := "---\nbead_ids:\n  - alpha\n  - beta\n---\n"
	_, ok := Field([]byte(content), "bead_ids")
	if ok {
		t.Error("Field(bead_ids) ok=true, want false for sequence value")
	}
}

func TestField_Missing(t *testing.T) {
	content := "---\nstatus: Draft\n---\n"
	if _, ok := Field([]byte(content), "missing_key"); ok {
		t.Error("Field(missing_key) ok=true, want false")
	}
}

func TestStatusFromPath_Missing(t *testing.T) {
	got := StatusFromPath("/nonexistent/path/should/not/exist.md")
	if got != "" {
		t.Errorf("StatusFromPath(missing) = %q, want \"\"", got)
	}
}

func TestStatusFromPath_Roundtrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "plan.md")
	content := "---\nstatus: Approved\n---\n# Plan\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if got := StatusFromPath(path); got != "Approved" {
		t.Errorf("StatusFromPath() = %q, want %q", got, "Approved")
	}
}

func TestParse_Basic(t *testing.T) {
	content := []byte("---\nstatus: Draft\n---\nbody\n")
	block, offset, ok := Parse(content)
	if !ok {
		t.Fatal("Parse ok=false, want true")
	}
	if string(block) != "status: Draft\n" {
		t.Errorf("Parse block = %q, want %q", string(block), "status: Draft\n")
	}
	// Body should begin after the closing fence line.
	if string(content[offset:]) != "body\n" {
		t.Errorf("Parse body = %q, want %q", string(content[offset:]), "body\n")
	}
}

func TestParse_NoOpener(t *testing.T) {
	if _, _, ok := Parse([]byte("# Just a title\n")); ok {
		t.Error("Parse ok=true, want false (no opener)")
	}
}

func TestParse_NoCloser(t *testing.T) {
	if _, _, ok := Parse([]byte("---\nstatus: Draft\nbody")); ok {
		t.Error("Parse ok=true, want false (no closer)")
	}
}
