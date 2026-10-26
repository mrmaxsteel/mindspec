package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS133ArtifactPath_PanelSubmitAndSchema(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			for _, name := range []string{"../x", "/x", "x\x01y", "evidence/result.txt"} {
				t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
					f := newCLIV2(t)
					if err := os.WriteFile(filepath.Join(f.root, ".mindspec", "config.yaml"), []byte("enforcement:\n  panel_presence: "+policy+"\n"), 0600); err != nil {
						t.Fatal(err)
					}
					config.ResetCache()
					v := paneltest.Verdict(panel.VerdictRequestChanges)
					v.ConcreteChangesRequired[0].ArtifactPath = name
					file := f.file(t, v)
					if name == "evidence/result.txt" {
						out, err := runPanelVerbCmd(f.args(file)...)
						if err != nil || !strings.Contains(out, "R1") {
							t.Fatalf("valid field not submitted: %v\n%s", err, out)
						}
					} else {
						cliV2Refusal(t, f, "/concrete_changes_required/0/artifact_path", f.args(file)...)
					}
					schema, err := runPanelVerbCmd("schema")
					if err != nil {
						t.Fatal(err)
					}
					var root struct {
						Properties map[string]struct {
							Items struct {
								Properties map[string]json.RawMessage `json:"properties"`
							} `json:"items"`
						} `json:"properties"`
					}
					if err := json.Unmarshal([]byte(schema), &root); err != nil {
						t.Fatal(err)
					}
					if _, ok := root.Properties["concrete_changes_required"].Items.Properties["artifact_path"]; !ok {
						t.Fatalf("item schema omits artifact_path: %s", schema)
					}
				})
			}
		})
	}
}
