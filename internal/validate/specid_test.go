package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpecID(t *testing.T) {
	valid := []string{
		"001-init",
		"033-security-hardening",
		"033-security-hardening-sast-findings",
		"999-foo",
		"0001-four-digits",
	}
	for _, id := range valid {
		if err := SpecID(id); err != nil {
			t.Errorf("SpecID(%q) unexpected error: %v", id, err)
		}
	}

	invalid := []string{
		"",
		".",
		"..",
		"../etc/passwd",
		"foo/bar",
		"foo\\bar",
		"33-too-short",   // only 2 digits
		"no-digits",      // missing leading digits
		"033",            // no slug
		"033-",           // trailing hyphen
		"033-UPPERCASE",  // uppercase
		"033-has spaces", // spaces
		"033-has_under",  // underscores
	}
	for _, id := range invalid {
		if err := SpecID(id); err == nil {
			t.Errorf("SpecID(%q) expected error, got nil", id)
		}
	}
}

func TestSafePath(t *testing.T) {
	// Create a temp dir structure for testing.
	root := t.TempDir()
	sub := filepath.Join(root, "sub", "dir")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	// Path within root → ok.
	if err := SafePath(root, sub); err != nil {
		t.Errorf("SafePath(%q, %q) unexpected error: %v", root, sub, err)
	}

	// Root itself → ok.
	if err := SafePath(root, root); err != nil {
		t.Errorf("SafePath(%q, %q) unexpected error: %v", root, root, err)
	}

	// Path outside root → error.
	outside := filepath.Dir(root)
	if err := SafePath(root, outside); err == nil {
		t.Errorf("SafePath(%q, %q) expected error, got nil", root, outside)
	}

	// Symlink escape → error.
	outsideDir := t.TempDir()
	symlink := filepath.Join(root, "escape")
	if err := os.Symlink(outsideDir, symlink); err != nil {
		t.Skip("symlinks not supported")
	}
	if err := SafePath(root, symlink); err == nil {
		t.Errorf("SafePath(%q, %q) expected error for symlink escape, got nil", root, symlink)
	}
}
