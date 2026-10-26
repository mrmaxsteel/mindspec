package panel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSuppliedBriefEmpty(t *testing.T) {
	for _, body := range []string{"", " \n\t\r\n"} {
		file := filepath.Join(t.TempDir(), "brief.md")
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readBriefFile(file); err == nil || !strings.Contains(err.Error(), "fill") {
			t.Fatalf("accepted empty body or missing recovery: %v", err)
		}
	}
}
func TestSuppliedBriefQuotedSyntax(t *testing.T) {
	for _, body := range []string{
		"Quoted:\n```md\n<!-- TODO(skill) fill -->\n```\n",
		"The guard refuses `<!-- TODO(skill)` comments.\n",
		"Quoted:\n~~~md\n<!-- mindspec:panel-header -->\n~~~\n",
		"The marker is `<!-- mindspec:panel-header -->`.\n",
		"Quoted ``<!-- TODO(skill) ` example -->``.\n",
		"Plain prose mentions TODO(skill) as the stub label.\n",
		"The half-edited form is `TODO(skill): fill -->`.\n",
	} {
		file := filepath.Join(t.TempDir(), "brief.md")
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if data, err := readBriefFile(file); err != nil || string(data) != body {
			t.Errorf("quoted syntax refused: %q %v", body, err)
		}
	}
	for _, body := range []string{
		"## Scope\n```md\ncode\n<!-- TODO(skill): fill -->\n",
		"## Scope\n    ```md\n<!-- TODO(skill): fill -->\n",
		"## Scope\n\t```md\n<!-- TODO(skill): fill -->\n",
		"## Scope\nTODO(skill): fill -->\n",
		"<!-- note\nTODO(skill): fill\n-->\n",
		"<!-- note\nTODO (Skill): fill\n",
		"## Scope\n<!-- TODO (skill): fill -->\n",
	} {
		file := filepath.Join(t.TempDir(), "brief.md")
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readBriefFile(file); err == nil || !strings.Contains(err.Error(), "TODO") {
			t.Errorf("supplied stub accepted: %q %v", body, err)
		}
		if _, err := ReadFilledBrief(file); err == nil || !strings.Contains(err.Error(), "TODO") {
			t.Errorf("dispatch stub accepted: %q %v", body, err)
		}
	}
}

func TestSuppliedBriefUnclosedFenceHeader(t *testing.T) {
	for _, marker := range []string{"<!-- mindspec:panel-header -->", "<!-- /mindspec:panel-header -->"} {
		file := filepath.Join(t.TempDir(), "brief.md")
		if err := os.WriteFile(file, []byte("Instructions\n```md\n"+marker+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readBriefFile(file); err == nil || !strings.Contains(err.Error(), "managed-header") {
			t.Fatalf("accepted marker after unclosed fence: %s: %v", marker, err)
		}
	}
}

func TestSuppliedBriefFenceScanDisagreement(t *testing.T) {
	for _, marker := range []string{
		briefHeaderOpen,
		briefHeaderClose,
		"<!--  mindspec:panel-header -->",
		"<!-- / mindspec:panel-header -->",
		"<!-- MINDSPEC:panel-header -->",
	} {
		t.Run(marker, func(t *testing.T) {
			body := "## Summary\n\nok\n\n    ```\n```\n" + marker + "\n```\n\nend\n"
			file := filepath.Join(t.TempDir(), "brief.md")
			if err := os.WriteFile(file, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readBriefFile(file); err == nil || !strings.Contains(err.Error(), "managed-header") {
				t.Fatalf("accepted marker exposed by the header parser: %q: %v", marker, err)
			}
		})
	}
}
