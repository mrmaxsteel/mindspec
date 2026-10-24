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
	} {
		file := filepath.Join(t.TempDir(), "brief.md")
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if data, err := readBriefFile(file); err != nil || string(data) != body {
			t.Errorf("quoted syntax refused: %q %v", body, err)
		}
	}
}
