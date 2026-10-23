package setup

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

var s133GuidanceDigests = map[string]string{
	"ms-impl-approve.pre133-agents": "5033283a1fd1ebc433bd235732871d973c9e7f8502ae7c6cec703daa9973c2c7",
	"ms-plan-approve.pre133-agents": "3438c062ef3834cf57421eb2b412aac35a366d6e66ebc48ea6bf9b1aade697e4",
	"ms-spec-approve":               "0804274dd777d98e7e712141aeed122e1a492146a131e0e7568103678067d960",
	"ms-plan-approve":               "f4c3ad3bdd41c26918d03237a65e777e6f314bfb5dcb9e6db0ab2b65db9cff6d",
	"ms-impl-approve":               "03486b42534f422e8349ec0d4ecb7220dba921e8db36ad1ac492b36c2430a318",
	"ms-spec-final-review":          "848315196f383a5d722a810e80f4e0144bd100468bed3b2725ee1bf9e26229c5",
	"ms-bead-cycle":                 "6c5edba1ab69e35bf5576fb1ab545c71347c87b54c4cfae25d844065e0998d5f",
	"ms-panel-run":                  "46d3151480536e8976bb98ca8ea039665db780910a37907dde9c79f0e59ef16a",
	"ms-panel-tally":                "cf5b9b56c49aa32edaac51d9c89ed497c9168db47683be86f5a6eb26ac6c4865",
}

func TestS133Guidance_RefreshAndPreserve(t *testing.T) {
	canonical := skillFiles()
	for stem, digest := range s133GuidanceDigests {
		name := strings.Split(stem, ".")[0]
		if stem == name {
			stem += ".pre133"
		}
		t.Run(stem, func(t *testing.T) {
			_, prior := readPresurfaceSnapshot(t, stem)
			if got := fmt.Sprintf("%x", sha256.Sum256([]byte(prior))); got != digest {
				t.Fatalf("snapshot digest = %s, want %s", got, digest)
			}
			if prior == canonical[name] || canonical[name] == "" {
				t.Fatal("missing or unchanged canonical body")
			}
			for _, policy := range []string{"warn", "enforce"} {
				for _, agent := range []string{".agents", ".claude"} {
					for _, modified := range []bool{false, true} {
						t.Run(fmt.Sprintf("%s/%s/modified=%t", policy, agent, modified), func(t *testing.T) {
							root := t.TempDir()
							configDir := filepath.Join(root, ".mindspec")
							if err := os.MkdirAll(configDir, 0755); err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("enforcement:\n  panel_presence: "+policy+"\n"), 0644); err != nil {
								t.Fatal(err)
							}
							config.ResetCache()
							t.Cleanup(config.ResetCache)
							cfg, err := config.Load(root)
							if err != nil || cfg.Enforcement.PanelPresence != policy {
								t.Fatalf("policy fixture not loaded: %+v %v", cfg, err)
							}
							rel := filepath.Join(agent, "skills")
							dir := filepath.Join(root, rel)
							body := prior
							if modified {
								body += "\n<!-- local instruction -->\n"
							}
							writeExisting(t, dir, name, body)
							result := &Result{}
							if err := installSkills(dir, rel, canonical, false, result); err != nil {
								t.Fatal(err)
							}
							expected := canonical[name]
							if modified {
								expected = body
							}
							if got := readSkill(t, dir, name); got != expected {
								t.Fatal("installed body differs from expected")
							}
							if containsPath(result.Refreshed, filepath.Join(rel, name, "SKILL.md")) == modified {
								t.Fatalf("wrong refresh classification: %+v", result)
							}
							if modified && len(result.Notices) == 0 {
								t.Fatal("missing user-modified notice")
							}
						})
					}
				}
			}
		})
	}
}

func s133CycleOrderError(body string) error {
	sequenceStart := strings.Index(body, "## Sequence\n")
	if sequenceStart < 0 {
		return fmt.Errorf("missing sequence")
	}
	sequenceEnd := strings.Index(body[sequenceStart:], "\n```\n\n")
	if sequenceEnd < 0 {
		return fmt.Errorf("missing sequence end")
	}
	sequence := body[sequenceStart : sequenceStart+sequenceEnd]
	// Both the initial implementation and each fix must be checked before review.
	rounds := strings.Split(sequence, "/ms-panel-run")
	if len(rounds) != 3 {
		return fmt.Errorf("expected initial and fix panel rounds")
	}
	for _, round := range rounds[:2] {
		commit := strings.LastIndex(round, "implementation commit SHA")
		check := strings.LastIndex(round, "mindspec complete <bead> --check")
		if commit < 0 || check <= commit {
			return fmt.Errorf("content check must follow commit and precede panel")
		}
	}
	for _, required := range []string{"send all\nfailing gates and recovery lines to the implementer or fixer", "panel reviews the fixed diff"} {
		if !strings.Contains(body, required) {
			return fmt.Errorf("missing failure routing: %s", required)
		}
	}
	return nil
}

func TestS133Guidance_ContentCheckOrder(t *testing.T) {
	body := skillFiles()["ms-bead-cycle"]
	if err := s133CycleOrderError(body); err != nil {
		t.Fatal(err)
	}
	// Keep --check present, but put it after the panel. A presence-only pin misses this.
	wrong := strings.Replace(body, "mindspec complete <bead> --check\n  ↓ content gates PASS\n/ms-panel-run", "/ms-panel-run\nmindspec complete <bead> --check", 1)
	if wrong == body || !strings.Contains(wrong, "--check") {
		t.Fatal("order mutation did not fire")
	}
	if err := s133CycleOrderError(wrong); err == nil {
		t.Fatal("check-after-panel passed")
	}
	wrong = strings.Replace(body, "  ↓ new implementation commit SHA + flagged deviations\nmindspec complete <bead> --check\n  ↓ content gates PASS", "  ↓ new implementation commit SHA + flagged deviations", 1)
	if wrong == body || s133CycleOrderError(wrong) == nil {
		t.Fatal("unchecked fix round passed")
	}
}

func TestS133Guidance_ApprovalContracts(t *testing.T) {
	files := skillFiles()
	for _, tc := range []struct{ name, gate, verb string }{
		{"ms-spec-approve", "spec_approve", "spec"}, {"ms-plan-approve", "plan_approve", "plan"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := documentGuidanceError(files[tc.name], tc.gate, tc.verb); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, name := range []string{"ms-impl-approve", "ms-spec-final-review"} {
		t.Run(name, func(t *testing.T) {
			body := files[name]
			required := "`mindspec impl approve` refuses without a passing final review for every spec, whatever the number of beads."
			if !strings.Contains(body, required) {
				t.Fatal("missing universal final-review refusal")
			}
			if !strings.Contains(body, "a gate, a schema, a trust boundary or a merge path") || !strings.Contains(body, "mindspec config resolve --gate final_review") {
				t.Fatal("missing change kinds or family preview")
			}
			wrong := strings.Replace(body, "for every spec, whatever the number of beads", "only for specs with multiple beads", 1)
			if wrong == body || strings.Contains(wrong, required) {
				t.Fatal("small-spec exception mutation passed")
			}
		})
	}
	for _, name := range []string{"ms-panel-run", "ms-panel-tally"} {
		body := files[name]
		if !strings.Contains(body, "For `spec_approve`, `plan_approve` and `final_review`,") || !strings.Contains(body, "--target spec/<id>") {
			t.Fatalf("%s lacks lifecycle gate targets", name)
		}
	}
	for _, required := range []string{"older live registration with another target blocks", "new round", "abandoned with a reason", "older binary"} {
		if !strings.Contains(files["ms-panel-run"], required) {
			t.Fatalf("missing target recovery %q", required)
		}
	}
	tally := files["ms-panel-tally"]
	if strings.Contains(tally, "Spec approval and adhoc panels cannot defer") || !strings.Contains(tally, "--to-plan") || !strings.Contains(tally, "## Resolutions") {
		t.Fatal("stale spec deferral guidance")
	}
}

func TestS133Guidance_NoInternalReferences(t *testing.T) {
	for stem := range s133GuidanceDigests {
		name := strings.Split(stem, ".")[0]
		if hits := surfaceRefScanContent("skill:"+name, skillFiles()[name], true); len(hits) != 0 {
			t.Fatalf("%s: %+v", name, hits)
		}
	}
}
