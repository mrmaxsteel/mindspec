package kernelid

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPanelOrchestrated_ParseErrors(t *testing.T) {
	for _, good := range []string{"", "a-good", "z-good"} {
		t.Run("good="+good, func(t *testing.T) {
			root := t.TempDir()
			write := func(dir, data string) string {
				path := filepath.Join(root, "reviews", dir, "panel.json")
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				return path
			}
			first := write("b-bad", `{not json`)
			write("c-bad", `{"orchestrated":"wrong type"}`)
			write("d-plain", `{"orchestrated":false}`)
			if good != "" {
				write(good, `{"orchestrated":true}`)
			}
			found, err := PanelOrchestrated(root)
			if good != "" {
				if !found || err != nil {
					t.Fatalf("valid witness: found=%v err=%v", found, err)
				}
				return
			}
			var syntax *json.SyntaxError
			if found || !errors.As(err, &syntax) || !strings.Contains(err.Error(), first) {
				t.Fatalf("want first parse error at %s: found=%v err=%v", first, found, err)
			}
		})
	}
}
