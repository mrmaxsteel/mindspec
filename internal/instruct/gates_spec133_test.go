package instruct

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/state"
)

func TestS133InstructGate_DocumentPanels(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, tc := range []struct{ mode, gate string }{{state.ModeSpec, "spec_approve"}, {state.ModePlan, "plan_approve"}} {
			t.Run(policy+"/"+tc.mode, func(t *testing.T) {
				root := t.TempDir()
				dir := filepath.Join(root, ".mindspec", "specs", "001-fixture")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("enforcement:\n  panel_presence: "+policy+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("---\nstatus: Approved\n---\n# Fixture\n"), 0644); err != nil {
					t.Fatal(err)
				}
				config.ResetCache()
				t.Cleanup(config.ResetCache)
				cfg, err := config.Load(root)
				if err != nil || cfg.Enforcement.PanelPresence != policy {
					t.Fatalf("policy fixture not loaded: %+v %v", cfg, err)
				}
				ctx := BuildContext(root, &state.Focus{Mode: tc.mode, ActiveSpec: "001-fixture"})
				markdown, err := Render(ctx)
				if err != nil {
					t.Fatal(err)
				}
				rawJSON, err := RenderJSON(ctx)
				if err != nil {
					t.Fatal(err)
				}
				var structured JSONOutput
				if err := json.Unmarshal([]byte(rawJSON), &structured); err != nil {
					t.Fatal(err)
				}
				for _, output := range []string{markdown, structured.Guidance} {
					command := "Approval needs a passing document panel: `mindspec panel create <slug> --spec 001-fixture --gate " + tc.gate + " --target spec/001-fixture`."
					if strings.Count(output, command) != 1 {
						t.Fatalf("missing or duplicate gate line:\n%s", output)
					}
					kinds := "Use reviewers from more than one family for a spec that changes a gate, a schema, a trust boundary or a merge path."
					if strings.Count(output, kinds) != 1 {
						t.Fatal("missing or duplicate change-kinds line")
					}
					other := "plan_approve"
					if tc.gate == other {
						other = "spec_approve"
					}
					if strings.Contains(output, "--gate "+other) {
						t.Fatal("wrong mode's gate rendered")
					}
				}
			})
		}
	}
}
