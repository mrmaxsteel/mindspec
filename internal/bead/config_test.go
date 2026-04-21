package bead

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func writeBeadsConfig(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCustomStatuses_SingleScalar(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `status.custom: "resolved"`+"\n")

	got := CustomStatuses(tmp)
	if !reflect.DeepEqual(got, []string{"resolved"}) {
		t.Errorf("got %v, want [resolved]", got)
	}
}

func TestCustomStatuses_CommaSeparated(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `status.custom: "resolved, paused, waiting"`+"\n")

	got := CustomStatuses(tmp)
	want := []string{"resolved", "paused", "waiting"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCustomStatuses_YAMLList(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, "status.custom:\n  - resolved\n  - paused\n")

	got := CustomStatuses(tmp)
	want := []string{"resolved", "paused"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCustomStatuses_MissingConfigIsEmpty(t *testing.T) {
	got := CustomStatuses(t.TempDir())
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestCustomStatuses_MalformedConfigIsEmpty(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, "not: yaml: [")

	got := CustomStatuses(tmp)
	if len(got) != 0 {
		t.Errorf("expected empty slice on malformed config, got %v", got)
	}
}

func TestAllStatuses_MergesBuiltinsAndCustom(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `status.custom: "resolved, paused"`+"\n")

	got := AllStatuses(tmp)
	want := []string{"open", "in_progress", "blocked", "closed", "resolved", "paused"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAllStatuses_DedupsAgainstBuiltins(t *testing.T) {
	tmp := t.TempDir()
	// Config redeclares `closed` as custom — must not appear twice.
	writeBeadsConfig(t, tmp, `status.custom: "closed, resolved"`+"\n")

	got := AllStatuses(tmp)
	want := []string{"open", "in_progress", "blocked", "closed", "resolved"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAllStatuses_NoConfigReturnsBuiltinsOnly(t *testing.T) {
	got := AllStatuses(t.TempDir())
	want := []string{"open", "in_progress", "blocked", "closed"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func readConfig(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".beads", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEnsureBeadsConfig_FreshFile(t *testing.T) {
	tmp := t.TempDir()
	res, err := EnsureBeadsConfig(tmp, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.CreatedFile {
		t.Errorf("expected CreatedFile=true on fresh file")
	}
	got := readConfig(t, tmp)
	wantKeys := []string{"issue-prefix", "types.custom", "status.custom", "export.git-add"}
	for _, k := range wantKeys {
		if !strings.Contains(got, k) {
			t.Errorf("fresh config missing key %q; got:\n%s", k, got)
		}
	}
	base := filepath.Base(tmp)
	if !strings.Contains(got, base) {
		t.Errorf("expected issue-prefix to default to %q; got:\n%s", base, got)
	}
	if !reflect.DeepEqual(sortedCopy(res.Added), sortedCopy(wantKeys)) {
		t.Errorf("Added = %v, want %v", res.Added, wantKeys)
	}
}

func TestEnsureBeadsConfig_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	if _, err := EnsureBeadsConfig(tmp, false); err != nil {
		t.Fatal(err)
	}
	first := readConfig(t, tmp)
	res, err := EnsureBeadsConfig(tmp, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.CreatedFile {
		t.Errorf("second call should not create file")
	}
	if len(res.Added) != 0 {
		t.Errorf("second call should add nothing; got %v", res.Added)
	}
	second := readConfig(t, tmp)
	if first != second {
		t.Errorf("non-idempotent write:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestEnsureBeadsConfig_MergesIntoPartialFile(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `issue-prefix: "myproj"
events-export: true
`)
	res, err := EnsureBeadsConfig(tmp, false)
	if err != nil {
		t.Fatal(err)
	}
	got := readConfig(t, tmp)
	if !strings.Contains(got, `issue-prefix: "myproj"`) {
		t.Errorf("existing issue-prefix should be preserved verbatim; got:\n%s", got)
	}
	if !strings.Contains(got, "events-export: true") {
		t.Errorf("non-mindspec key events-export must be preserved; got:\n%s", got)
	}
	for _, k := range []string{"types.custom", "status.custom", "export.git-add"} {
		if !strings.Contains(got, k) {
			t.Errorf("expected merged key %q; got:\n%s", k, got)
		}
	}
	if containsString(res.Added, "issue-prefix") {
		t.Errorf("issue-prefix already present; should NOT be re-added: %v", res.Added)
	}
}

func TestEnsureBeadsConfig_PreservesNonMindspecKeys(t *testing.T) {
	tmp := t.TempDir()
	body := `# user top-of-file comment
issue-prefix: "myproj"
sync-branch: "beads-sync"  # trailing comment
events-export: true
no-daemon: false
`
	writeBeadsConfig(t, tmp, body)
	if _, err := EnsureBeadsConfig(tmp, false); err != nil {
		t.Fatal(err)
	}
	got := readConfig(t, tmp)
	for _, fragment := range []string{
		"# user top-of-file comment",
		`issue-prefix: "myproj"`,
		"sync-branch",
		"# trailing comment",
		"events-export: true",
		"no-daemon: false",
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("fragment %q missing from merged output:\n%s", fragment, got)
		}
	}
}

func TestEnsureBeadsConfig_RecordsUserDriftWithoutForce(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `export.git-add: true
`)
	res, err := EnsureBeadsConfig(tmp, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UserAuthored) != 1 || res.UserAuthored[0].Key != "export.git-add" {
		t.Fatalf("expected UserAuthored=[export.git-add]; got %v", res.UserAuthored)
	}
	if res.UserAuthored[0].HaveRaw != "true" {
		t.Errorf("UserAuthored.HaveRaw = %q, want %q", res.UserAuthored[0].HaveRaw, "true")
	}
	got := readConfig(t, tmp)
	if !strings.Contains(got, "export.git-add: true") {
		t.Errorf("user value must be preserved without force; got:\n%s", got)
	}
	if containsString(res.Added, "export.git-add") {
		t.Errorf("export.git-add should not be in Added when drift detected without force")
	}
}

func TestEnsureBeadsConfig_ForceOverridesDrift(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `export.git-add: true
`)
	res, err := EnsureBeadsConfig(tmp, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UserAuthored) != 0 {
		t.Errorf("force=true should not report UserAuthored; got %v", res.UserAuthored)
	}
	if !containsString(res.Added, "export.git-add") {
		t.Errorf("force=true should report export.git-add in Added; got %v", res.Added)
	}
	got := readConfig(t, tmp)
	if !strings.Contains(got, "export.git-add: false") {
		t.Errorf("force=true should flip the value; got:\n%s", got)
	}
}

func TestEnsureBeadsConfig_NeverOverwritesIssuePrefix(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `issue-prefix: "custom-name"
`)
	res, err := EnsureBeadsConfig(tmp, true) // even with force
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UserAuthored) != 0 {
		t.Errorf("issue-prefix is never drift; got UserAuthored=%v", res.UserAuthored)
	}
	got := readConfig(t, tmp)
	if !strings.Contains(got, `issue-prefix: "custom-name"`) {
		t.Errorf("issue-prefix must not be overwritten, even with force; got:\n%s", got)
	}
}

func TestEnsureBeadsConfig_AlreadyCorrect(t *testing.T) {
	tmp := t.TempDir()
	body := `issue-prefix: "myproj"
types.custom: "gate"
status.custom: "resolved"
export.git-add: false
`
	writeBeadsConfig(t, tmp, body)
	res, err := EnsureBeadsConfig(tmp, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 0 {
		t.Errorf("nothing should be added; got %v", res.Added)
	}
	for _, k := range []string{"types.custom", "status.custom", "export.git-add"} {
		if !containsString(res.AlreadyCorrect, k) {
			t.Errorf("expected %q in AlreadyCorrect; got %v", k, res.AlreadyCorrect)
		}
	}
}

// --- YAML edge-case goldens ---------------------------------------------------

func TestEnsureBeadsConfig_Golden_TopOfFileComment(t *testing.T) {
	tmp := t.TempDir()
	body := `# Top-of-file banner
# Second header line
issue-prefix: "myproj"
types.custom: "gate"
status.custom: "resolved"
export.git-add: false
`
	writeBeadsConfig(t, tmp, body)
	if _, err := EnsureBeadsConfig(tmp, false); err != nil {
		t.Fatal(err)
	}
	got := readConfig(t, tmp)
	for _, line := range []string{"# Top-of-file banner", "# Second header line"} {
		if !strings.Contains(got, line) {
			t.Errorf("comment %q lost; got:\n%s", line, got)
		}
	}
}

func TestEnsureBeadsConfig_Golden_InterleavedComments(t *testing.T) {
	tmp := t.TempDir()
	body := `issue-prefix: "myproj"
# comment before types
types.custom: "gate"
# comment between types and status
status.custom: "resolved"
# comment before export
export.git-add: false
`
	writeBeadsConfig(t, tmp, body)
	if _, err := EnsureBeadsConfig(tmp, false); err != nil {
		t.Fatal(err)
	}
	got := readConfig(t, tmp)
	for _, line := range []string{
		"# comment before types",
		"# comment between types and status",
		"# comment before export",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("interleaved comment %q lost; got:\n%s", line, got)
		}
	}
}

func TestEnsureBeadsConfig_Golden_BlockStyleValue(t *testing.T) {
	tmp := t.TempDir()
	body := `issue-prefix: "myproj"
types.custom: "gate"
status.custom: "resolved"
export.git-add: false
description: |
  This is a multi-line
  block-style value that the
  user authored and expects
  to survive intact.
`
	writeBeadsConfig(t, tmp, body)
	if _, err := EnsureBeadsConfig(tmp, false); err != nil {
		t.Fatal(err)
	}
	got := readConfig(t, tmp)
	if !strings.Contains(got, "description: |") {
		t.Errorf("block-style marker lost; got:\n%s", got)
	}
	for _, line := range []string{
		"This is a multi-line",
		"block-style value that the",
		"user authored and expects",
		"to survive intact.",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("block-style line %q lost; got:\n%s", line, got)
		}
	}
}

func TestEnsureBeadsConfig_Golden_AnchorReference(t *testing.T) {
	tmp := t.TempDir()
	body := `defaults: &defaults
  owner: "team@example.com"
issue-prefix: "myproj"
types.custom: "gate"
status.custom: "resolved"
export.git-add: false
overrides: *defaults
`
	writeBeadsConfig(t, tmp, body)
	if _, err := EnsureBeadsConfig(tmp, false); err != nil {
		t.Fatal(err)
	}
	got := readConfig(t, tmp)
	if !strings.Contains(got, "&defaults") {
		t.Errorf("anchor definition lost; got:\n%s", got)
	}
	if !strings.Contains(got, "*defaults") {
		t.Errorf("anchor reference lost; got:\n%s", got)
	}
}

// --- Edge cases ---------------------------------------------------------------

func TestEnsureBeadsConfig_EmptyFileTreatedAsMissing(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, "")
	res, err := EnsureBeadsConfig(tmp, false)
	if err != nil {
		t.Fatalf("empty file should be handled without error, got: %v", err)
	}
	if !res.CreatedFile {
		t.Errorf("empty file should be treated like a missing file (CreatedFile=true)")
	}
	got := readConfig(t, tmp)
	for _, k := range []string{"issue-prefix", "types.custom", "status.custom", "export.git-add"} {
		if !strings.Contains(got, k) {
			t.Errorf("empty-file path should seed key %q; got:\n%s", k, got)
		}
	}
}

func TestEnsureBeadsConfig_WhitespaceOnlyFileTreatedAsMissing(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, "   \n\n\t\n")
	res, err := EnsureBeadsConfig(tmp, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.CreatedFile {
		t.Errorf("whitespace-only file should be treated like missing")
	}
}

func TestEnsureBeadsConfig_NonScalarDriftRenderedInHaveRaw(t *testing.T) {
	tmp := t.TempDir()
	// User wrote a list where mindspec expects a scalar.
	writeBeadsConfig(t, tmp, "types.custom:\n  - gate\n  - paused\n")
	res, err := EnsureBeadsConfig(tmp, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UserAuthored) != 1 || res.UserAuthored[0].Key != "types.custom" {
		t.Fatalf("expected UserAuthored=[types.custom]; got %v", res.UserAuthored)
	}
	have := res.UserAuthored[0].HaveRaw
	if !strings.Contains(have, "gate") || !strings.Contains(have, "paused") {
		t.Errorf("HaveRaw should render the user's sequence; got %q", have)
	}
}

func TestEnsureBeadsConfig_IdempotentAfterUserAuthoredMerge(t *testing.T) {
	tmp := t.TempDir()
	writeBeadsConfig(t, tmp, `# user header
issue-prefix: "myproj"
events-export: true
sync-branch: "beads-sync"
`)
	if _, err := EnsureBeadsConfig(tmp, false); err != nil {
		t.Fatal(err)
	}
	first := readConfig(t, tmp)
	if _, err := EnsureBeadsConfig(tmp, false); err != nil {
		t.Fatal(err)
	}
	second := readConfig(t, tmp)
	if first != second {
		t.Errorf("non-idempotent after user-authored merge:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// --- Atomic write -------------------------------------------------------------

func TestEnsureBeadsConfig_AtomicWrite_NoTempLeft(t *testing.T) {
	tmp := t.TempDir()
	if _, err := EnsureBeadsConfig(tmp, false); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(tmp, ".beads"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".config.yaml.") {
			t.Errorf("temp file left behind after atomic write: %s", e.Name())
		}
	}
}

// --- Test helpers -------------------------------------------------------------

func containsString(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func sortedCopy(ss []string) []string {
	out := make([]string, len(ss))
	copy(out, ss)
	sort.Strings(out)
	return out
}
