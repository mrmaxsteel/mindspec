package panel

import (
	"os"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/adr"
)

func TestS133Digest_ActualPlaceholderTemplate(t *testing.T) {
	for _, domains := range [][]string{nil, {"workflow"}, {"workflow", "core"}} {
		root := t.TempDir()
		name, err := adr.CreateWithID(root, "ADR-0099", "Placeholder for ADR-0099", adr.CreateOpts{Domains: domains})
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !isPlaceholder(data) {
			t.Fatal("digest template differs from the actual writer")
		}
		if isPlaceholder(append(data, []byte("body edit\n")...)) {
			t.Fatal("edited placeholder excluded")
		}
		if isPlaceholder([]byte(strings.Replace(string(data), "**Status**: Proposed", "**Status**: Accepted", 1))) {
			t.Fatal("accepted ADR excluded")
		}
	}
}
