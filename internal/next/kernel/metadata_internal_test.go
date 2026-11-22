package kernel

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestS134Store_SweepContinues(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", t.TempDir())
	dir, err := KernelTempDir(root)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	for _, suffix := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("metadata-%d-%s", pid, suffix)), []byte("metadata"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	denied := errors.New("cannot remove first file")
	var visited []string
	err = sweepMetadata(root, func(path string) error {
		visited = append(visited, filepath.Base(path))
		if strings.HasSuffix(path, "-a") {
			return denied
		}
		return os.Remove(path)
	})
	if !errors.Is(err, denied) || len(visited) != 2 {
		t.Fatalf("sweep stopped after an individual failure: visited=%v err=%v", visited, err)
	}
	if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("metadata-%d-b", pid))); !os.IsNotExist(err) {
		t.Fatalf("later stale file remains: %v", err)
	}
}

type countedRecordReader struct{ remaining, read int }

func (r *countedRecordReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	for i := 0; i < n; i++ {
		p[i] = ' '
	}
	r.remaining -= n
	r.read += n
	return n, nil
}

func TestS134Submit_RecordGrowingReadBound(t *testing.T) {
	reader := &countedRecordReader{remaining: MetadataLimit + 100}
	_, err := readRecordBytes("test-1.1.review.r1.verify", reader)
	if err == nil || !strings.Contains(err.Error(), "metadata-too-large") || reader.read != MetadataLimit+1 {
		t.Fatalf("record reader exceeded bounded read: bytes=%d err=%v", reader.read, err)
	}
}

func TestS134Submit_EvidenceIdentity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(root, "proof.txt")
	if err := os.WriteFile(evidence, []byte("proof"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(root), "REPO")
	t.Run("case-alias", func(t *testing.T) {
		if _, err := os.Stat(alias); os.IsNotExist(err) {
			t.Skip("requires a case-insensitive filesystem")
		}
		if !recordEvidenceResolves(alias, evidence) {
			t.Fatal("repository evidence through an identity alias did not resolve")
		}
	})
	outside := root + "-outside"
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(outside, "proof.txt")
	if err := os.WriteFile(path, []byte("proof"), 0600); err != nil {
		t.Fatal(err)
	}
	if recordEvidenceResolves(root, path) {
		t.Fatal("name prefix accepted as repository evidence")
	}
	child := filepath.Join(outside, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(child, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if recordEvidenceResolves(root, "alias"+string(filepath.Separator)+".."+string(filepath.Separator)+"proof.txt") {
		t.Fatal("cleaned symlink parent accepted external evidence")
	}
}
