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

func TestS134Submit_OpenedRecordIdentity(t *testing.T) {
	for _, mode := range []string{"same", "replaced", "directory"} {
		t.Run(mode, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "record.json")
			if err := os.WriteFile(name, []byte("record bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			checked, err := os.Stat(name)
			if err != nil {
				t.Fatal(err)
			}
			want := "changed after its path was checked"
			switch mode {
			case "replaced":
				if err := os.Rename(name, name+"-checked"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, []byte("other record"), 0600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				name = filepath.Dir(name)
				checked, err = os.Stat(name)
				if err != nil {
					t.Fatal(err)
				}
				want = "opened record is not a regular file"
			}
			text, err := readCheckedRecord("test-1.1.review.r1.lens", name, checked)
			if mode == "same" {
				if err != nil || text != "record bytes" {
					t.Fatalf("same opened file failed: %q %v", text, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), want) || text != "" {
				t.Fatalf("opened file check failed: %q %v", text, err)
			}
			if mode == "replaced" {
				want := "record changed after its path was checked\nrecovery: rerun the submit with a record file that is not being changed"
				if err.Error() != want {
					t.Fatalf("changed record recovery: got %q want %q", err.Error(), want)
				}
			}
		})
	}
}
