package shellquote

import (
	"os/exec"
	"testing"
)

func TestQuoteLiteralArgument(t *testing.T) {
	for _, value := range []string{"", " ", "'", ";", "$(printf INJECTED)", "\n"} {
		t.Run(value, func(t *testing.T) {
			got, err := exec.Command("/bin/sh", "-c", `printf '%s\000' "$#" "$@"`, "_", value).CombinedOutput()
			if err != nil || string(got) != "1\x00"+value+"\x00" {
				t.Fatalf("printer failed: %q %v", got, err)
			}
			got, err = exec.Command("/bin/sh", "-c", `set -- `+Quote(value)+`; printf '%s\000' "$#" "$@"`).CombinedOutput()
			if err != nil || string(got) != "1\x00"+value+"\x00" {
				t.Fatalf("quoted=%q got=%q want=%q error=%v", Quote(value), got, "1\x00"+value+"\x00", err)
			}
		})
	}
}
