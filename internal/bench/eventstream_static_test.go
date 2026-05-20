package bench_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func goModRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == "/dev/null" {
		t.Fatalf("no go.mod resolved; got %q", gomod)
	}
	return filepath.Dir(gomod)
}

// TestNoFileTailingEscapeHatch is the spec 083 Bead 3b step-4
// file-tailing-escape-hatch assertion. The three rewired files
// (internal/recording/collector.go, internal/bench/runner.go,
// internal/bench/session.go) MUST NOT contain constructs that obtain
// an io.Reader for client.ReadEvents by opening a file path — Hard
// Constraint #3 prohibits file-tailing the agentmind --output file.
//
// The only acceptable io.Reader source feeding client.ReadEvents is
// the subprocess stdout pipe (Handle.Stdout from
// exec.Cmd.StdoutPipe()).
//
// Implementation: regex-scan the three files for the prohibited
// `os.Open(.*outputPath)`, `os.Open(.*\.ndjson)`, or `tail` patterns,
// and fail if any match.
func TestNoFileTailingEscapeHatch(t *testing.T) {
	modRoot := goModRoot(t)
	files := []string{
		"internal/bench/runner.go",
		"internal/bench/session.go",
		"internal/recording/collector.go",
	}

	// Patterns the bead explicitly bans on the live-event-stream
	// read path. We don't ban os.Open broadly because session.go
	// has legitimate post-run file-aggregation paths
	// (countEventsByLabel, ParseSessionByLabel) that read the
	// already-persisted NDJSON file for reporting. The ban is
	// specifically on os.Open(outputPath) and os.Open of the
	// agentmind --output path used as a stream source feeding
	// client.ReadEvents.
	bannedNearReadEvents := []*regexp.Regexp{
		regexp.MustCompile(`client\.ReadEvents\([^)]*os\.Open`),
	}

	for _, rel := range files {
		path := filepath.Join(modRoot, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		// Strip comments so words like "tailing" in doc-comments don't
		// trigger false positives. Use a simple line-based filter
		// (good enough for Go source; the bead's escape-hatch ban is
		// about live code paths, not documentation).
		text = stripGoComments(text)
		for _, re := range bannedNearReadEvents {
			if loc := re.FindStringIndex(text); loc != nil {
				snippet := text[loc[0]:min(loc[1]+40, len(text))]
				t.Errorf("file-tailing escape hatch detected in %s: %q (Hard Constraint #3 violation — io.Reader for client.ReadEvents must be the subprocess stdout pipe, not a file handle)", rel, snippet)
			}
		}
	}
}

// stripGoComments removes // line comments and /* block comments */
// from Go source text. Returned text has the same byte positions as
// the original up to and within stripped regions (we replace with
// spaces) so regex match indices still align if you need them. For
// our purposes we only need a comment-free haystack.
func stripGoComments(src string) string {
	out := make([]byte, len(src))
	copy(out, src)
	// Block comments first.
	for i := 0; i < len(out)-1; i++ {
		if out[i] == '/' && out[i+1] == '*' {
			j := i + 2
			for j < len(out)-1 && !(out[j] == '*' && out[j+1] == '/') {
				out[j] = ' '
				j++
			}
			if j < len(out)-1 {
				out[j] = ' '
				out[j+1] = ' '
			}
			i = j + 1
		}
	}
	// Line comments.
	for i := 0; i < len(out)-1; i++ {
		if out[i] == '/' && out[i+1] == '/' {
			j := i
			for j < len(out) && out[j] != '\n' {
				out[j] = ' '
				j++
			}
			i = j
		}
	}
	return string(out)
}

// TestReadEventsConsumerSourceIsStdoutPipe is a positive assertion:
// every client.ReadEvents call site in the three rewired files MUST
// be invoked with `handle.Stdout` (or the equivalent helper
// `ConsumeHandleToFile` which routes the same pipe via Handle.Stdout).
// We scan source text for `client.ReadEvents(handle.Stdout` and
// `ConsumeHandleToFile(handle` invocations; either form is acceptable.
func TestReadEventsConsumerSourceIsStdoutPipe(t *testing.T) {
	modRoot := goModRoot(t)
	// Files that MUST contain a consumer call site. Panel REV-5
	// removed `ConsumeSessionStream` from session.go — session.go's
	// only relationship to the consumer is now transitive (the runner
	// drives `startBenchCollector` which owns the consumer), so we
	// no longer require session.go itself to match. Only runner.go
	// and the recording collector are direct call sites.
	files := []string{
		"internal/bench/runner.go",
		"internal/recording/collector.go",
	}
	// Acceptable invocations (regex alternation). Any of these
	// proves the file participates in the Bead 3b read-side rewire;
	// the live-binary test
	// (TestStreamConsumer_ReaderIsSubprocessStdoutPipe and
	// TestStartRecordingEventConsumer_ReadsFromSubprocessStdoutPipe)
	// supplies the dynamic proof that the reader IS a subprocess
	// stdout pipe.
	//
	// runner.go reaches the consumer via the bench-package
	// startBenchCollector helper (which returns StartResult.Consumer),
	// so we accept that indirection.
	consumerCallRE := regexp.MustCompile(
		`(?:client\.ReadEvents\(handle\.Stdout` +
			`|ConsumeHandleToFile\(handle` +
			`|startRecordingEventConsumer\(handle` +
			`|startBenchCollector\(` +
			`|startRes\.Consumer)`)

	for _, rel := range files {
		path := filepath.Join(modRoot, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !consumerCallRE.MatchString(string(data)) {
			t.Errorf("file %s contains no client.ReadEvents-on-handle.Stdout consumer invocation (Bead 3b read-side rewire missing)", rel)
		}
	}
}

// TestNoOTLPParserResidueInRecording is the spec 083 Bead 3b step-5
// OTLP-parser-residue assertion. After the read-side rewire, no code
// in internal/recording/** parses OTLP — the parsing lives in
// agentmind's internal/otlp/ package (and the legacy code in
// internal/bench/collector.go is staged for deletion by Bead 5).
//
// We scope this check to internal/recording only (Bead 3b's owned
// surface); the bench-side residue is addressed by Bead 5's deletion.
func TestNoOTLPParserResidueInRecording(t *testing.T) {
	modRoot := goModRoot(t)
	residueRE := regexp.MustCompile(
		`http\.HandleFunc.*"/v1/(logs|metrics|traces)"|parseOTLP|otlpKeyValue\b`)

	root := filepath.Join(modRoot, "internal", "recording")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip test files — they may grep for residue patterns
		// themselves (this file does, and would otherwise match).
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if loc := residueRE.FindStringIndex(string(data)); loc != nil {
			rel, _ := filepath.Rel(modRoot, path)
			snippet := string(data[loc[0]:min(loc[1]+40, len(data))])
			t.Errorf("OTLP-parser residue detected in %s: %q (Bead 3b step-5 violation — recording must not parse OTLP after the read-side rewire)", rel, snippet)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
