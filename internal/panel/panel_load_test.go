package panel

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestS134PanelLoad_ParseErrorNamesDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "reviews", "bad\x1b[31m")
	writeFile(t, dir, FileName, "{bad")
	reg := load(dir)
	if reg.Err == nil || !strings.Contains(reg.Err.Error(), termsafe.Escape(dir)) || strings.Contains(reg.Err.Error(), "\x1b") {
		t.Fatalf("parse error must name the escaped directory: %v", reg.Err)
	}
}
