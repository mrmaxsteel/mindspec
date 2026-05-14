package safeio

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAppendNoSymlink_RefusesSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	decoy := filepath.Join(root, "decoy")
	link := filepath.Join(root, "link")

	const original = "original-bytes\n"
	if err := os.WriteFile(decoy, []byte(original), 0o644); err != nil {
		t.Fatalf("writing decoy: %v", err)
	}
	if err := os.Symlink(decoy, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	f, err := OpenAppendNoSymlink(link, 0o644)
	if err == nil {
		f.Close()
		t.Fatal("OpenAppendNoSymlink(link) returned nil error; want ErrSymlinkRefused")
	}
	if !errors.Is(err, ErrSymlinkRefused) {
		t.Fatalf("OpenAppendNoSymlink(link) err = %v; want errors.Is(err, ErrSymlinkRefused)", err)
	}

	got, err := os.ReadFile(decoy)
	if err != nil {
		t.Fatalf("reading decoy: %v", err)
	}
	if string(got) != original {
		t.Errorf("decoy modified: got %q, want %q", string(got), original)
	}
}

func TestOpenAppendNoSymlink_AllowsRegularFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "regular.txt")
	if err := os.WriteFile(target, []byte("head\n"), 0o644); err != nil {
		t.Fatalf("seeding target: %v", err)
	}

	f, err := OpenAppendNoSymlink(target, 0o644)
	if err != nil {
		t.Fatalf("OpenAppendNoSymlink: %v", err)
	}
	if _, err := f.WriteString("tail\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(got) != "head\ntail\n" {
		t.Errorf("appended content = %q; want %q", string(got), "head\ntail\n")
	}
}

func TestWriteFileNoSymlink_RefusesSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	decoy := filepath.Join(root, "decoy")
	link := filepath.Join(root, "link")

	const original = "sensitive-bytes\n"
	if err := os.WriteFile(decoy, []byte(original), 0o644); err != nil {
		t.Fatalf("writing decoy: %v", err)
	}
	if err := os.Symlink(decoy, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	err := WriteFileNoSymlink(link, []byte("attacker-payload"), 0o644)
	if err == nil {
		t.Fatal("WriteFileNoSymlink(link) returned nil error; want ErrSymlinkRefused")
	}
	if !errors.Is(err, ErrSymlinkRefused) {
		t.Fatalf("WriteFileNoSymlink(link) err = %v; want errors.Is(err, ErrSymlinkRefused)", err)
	}

	got, err := os.ReadFile(decoy)
	if err != nil {
		t.Fatalf("reading decoy: %v", err)
	}
	if string(got) != original {
		t.Errorf("decoy modified: got %q, want %q", string(got), original)
	}

	// The symlink itself should remain a symlink (not silently replaced).
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("link is no longer a symlink (mode %v)", info.Mode())
	}
}

func TestWriteFileNoSymlink_CreatesNew(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "fresh.txt")

	if err := WriteFileNoSymlink(target, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFileNoSymlink: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(got) != "hello\n" {
		t.Errorf("content = %q; want %q", string(got), "hello\n")
	}
}

func TestWriteFileNoSymlink_OverwritesRegular(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "existing.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := WriteFileNoSymlink(target, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("WriteFileNoSymlink: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(got) != "new\n" {
		t.Errorf("content = %q; want %q", string(got), "new\n")
	}
}
