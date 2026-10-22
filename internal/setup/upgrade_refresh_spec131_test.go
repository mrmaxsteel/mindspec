package setup

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Pins are from git show v0.17.0:plugins/mindspec/skills/<name>/SKILL.md.
var spec131SkillDigests = map[string]string{
	"ms-panel-run":         "f84762a328e4e3f9e2ec8dd75a14eb801a49ae4fda9abd44787457e99ca0d8ca",
	"ms-panel-tally":       "7fdc52ed89d0ccffe093c1ef0e66b157cd32f9dacbeb65cf914d319056f1e0d4",
	"ms-bead-fix":          "c525fd6b6d59d8a54d09b6405f75d899877604ce4426e06294abe2c1fd871b7b",
	"ms-bead-cycle":        "81c991b65bd544c9695449a626fb01ebf4eef4cfe6ba1186fc4d1ef92df416ff",
	"ms-spec-final-review": "0d615aff52a5e85b914420df0e88349a5ccac843dbbe4dd88246760990c8690b",
	"ms-bead-impl":         "db650865ef20c0890b07c06c22b5c176315239afad5256283d047d32525d9b33",
}

func TestSetup_RefreshesSpec130Skills(t *testing.T) {
	for name, digest := range spec131SkillDigests {
		t.Run(name, func(t *testing.T) {
			_, prior := readPresurfaceSnapshot(t, name+".pre131")
			if got := fmt.Sprintf("%x", sha256.Sum256([]byte(prior))); got != digest {
				t.Fatalf("v0.17.0 snapshot digest = %s, want %s", got, digest)
			}
			want, ok := skillFiles()[name]
			if !ok || prior == want {
				t.Fatal("missing or unchanged canonical skill")
			}
			for _, modified := range []bool{false, true} {
				t.Run(fmt.Sprintf("user_modified=%t", modified), func(t *testing.T) {
					root := t.TempDir()
					dir := filepath.Join(root, ".claude", "skills")
					body := prior
					if modified {
						body += "\n<!-- local operator instruction -->\n"
					}
					writeExisting(t, dir, name, body)
					result, err := RunClaude(root, false)
					if err != nil {
						t.Fatal(err)
					}
					rel := filepath.Join(".claude", "skills", name, "SKILL.md")
					expected := want
					if modified {
						expected = body
					}
					if got := readSkill(t, dir, name); got != expected {
						t.Fatal("setup wrote the wrong skill body")
					}
					if containsPath(result.Refreshed, rel) == modified {
						t.Fatalf("wrong refresh classification: %+v", result)
					}
					if modified {
						found := false
						for _, n := range result.Notices {
							if strings.HasPrefix(n, rel) && strings.Contains(n, "user-modified") {
								found = true
							}
						}
						if !found {
							t.Fatal("missing user-modified notice")
						}
					}
				})
			}
		})
	}
}
