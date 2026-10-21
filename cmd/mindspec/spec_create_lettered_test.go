package main

import (
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"strings"
	"testing"
)

func TestSpecCreate_RefusesLetteredID(t *testing.T) {
	if err := idvalidate.SpecID("008b-human-gates"); err != nil {
		t.Fatal(err)
	}
	_, err := resolveSpecCreateID(t.TempDir(), "129b-x")
	if err == nil || !strings.Contains(err.Error(), "new spec IDs must use digits") || !strings.Contains(err.Error(), "recovery: mindspec spec create x") {
		t.Fatalf("lettered create: %v", err)
	}
	if id, err := resolveSpecCreateID(t.TempDir(), "129-x"); err != nil || id != "129-x" {
		t.Fatalf("plain create: %s %v", id, err)
	}
}
