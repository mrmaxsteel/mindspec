package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

func TestS134SubmitCmd_SafeRecordRefusal(t *testing.T) {
	bin := buildMindspecBinary(t)
	for _, mode := range []string{"hostile", "nested-root"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := actorCmdFixture(t)
			gate := f.Items[0] + ".review"
			id := gate + ".r1.lens"
			subject := actorCmdGit(t, f, "rev-parse", "HEAD")
			f.Create(id, gate)
			// An authorised round names its open lens step. The seeder for
			// authorisation closes the lens, so this test writes the round itself.
			round := kernel.GateRound{Round: 1, Subject: subject, SlotCount: 2, Slots: []string{gate + ".r1.R1", gate + ".r1.R2"}, Lens: id}
			f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRound, round)))
			f.Must(f.Store.WriteMetadata(id, map[string]any{kernel.KeySubject: kernel.Subject{Commit: subject}, kernel.KeyRound: kernel.LensAuthorization(round)}))
			store := actorCmdStore(t, f)
			token := judgeCmdLease(t, f, bin, id)
			cwd := f.Root
			name := filepath.Join(t.TempDir(), "missing\x1b[31m\nrecovery: injected")
			want := `\x1b`
			if mode == "nested-root" {
				cwd = filepath.Join(f.Root, "project")
				f.Must(os.Mkdir(cwd, 0700))
				f.Must(os.Rename(filepath.Join(f.Root, ".mindspec"), filepath.Join(cwd, ".mindspec")))
				name = filepath.Join(f.Root, "record.json")
				data := fmt.Sprintf(`{"step":%q,"lenses":[{"slot":"R1","lens":"paths"},{"slot":"R2","lens":"tests"}]}`, id)
				f.Must(os.WriteFile(name, []byte(data), 0600))
				want = "outside every worktree"
			}
			before, err := json.Marshal(actorCmdRead(t, store, id))
			f.Must(err)
			out, err := f.BinaryCommandIn(cwd, bin, "submit", id, "--lease", token, "--record", name)
			var exit *exec.ExitError
			if err == nil || !errors.As(err, &exit) || exit.ExitCode() == 0 {
				t.Fatalf("unsafe record accepted: %v %s", err, out)
			}
			text := string(out)
			if !strings.Contains(text, want) {
				t.Fatalf("wrong refusal, wanted %q: %s", want, text)
			}
			recoveries := 0
			for _, line := range strings.Split(text, "\n") {
				if strings.HasPrefix(line, "recovery:") {
					recoveries++
				}
				for _, r := range line {
					if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
						t.Fatalf("raw control in CLI refusal: %q", text)
					}
				}
			}
			if recoveries != 1 {
				t.Fatalf("wanted exactly one recovery line: %q", text)
			}
			if strings.Contains(text, `\\x1b`) {
				t.Fatalf("control byte escaped twice: %q", text)
			}
			after, err := json.Marshal(actorCmdRead(t, store, id))
			f.Must(err)
			if string(before) != string(after) {
				t.Fatal("refusal changed the step")
			}
		})
	}
}
