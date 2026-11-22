package kernel

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestS134Judge_MissingArtifactRefusal(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "init", "-q", root)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init: %v %s", err, out)
	}
	_, err := readJudgement(Record{})
	if err == nil || !strings.Contains(err.Error(), "mindspec_record is absent or invalid\nrecovery: mindspec next-action") {
		t.Fatalf("missing artifact refusal: %v", err)
	}
}
