package journal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

// helperEnv is the env var that turns the test binary into a single-append
// helper "process" (the standard Go re-exec-self pattern). When set, the
// helper appends ONE distinct event and exits before any test runs.
const helperEnv = "MINDSPEC_JOURNAL_APPEND_HELPER"

// TestMain dispatches to the append-helper mode when helperEnv is set so a
// parent test can spawn many real OS processes that each append to the same
// shared journal — the cross-process no-loss proof for the append-only
// O_APPEND design.
func TestMain(m *testing.M) {
	if idx := os.Getenv(helperEnv); idx != "" {
		os.Exit(runAppendHelper(idx))
	}
	os.Exit(m.Run())
}

// subTokens map a helper index to a DISTINCT closed-set Subcommand enum
// token, so each helper writes a distinct fingerprint — the case that
// exposed whole-entry CLOBBERING under the old read-modify-rewrite design.
var subTokens = []string{"approve", "impl", "phase"}

// runAppendHelper appends ONE event whose fingerprint is distinct per
// index, to the journal under MINDSPEC_STATE_DIR. Returns a process exit
// code.
func runAppendHelper(idxStr string) int {
	i, err := strconv.Atoi(idxStr)
	if err != nil {
		return 2
	}
	ev := Event{
		Argv0:       "mindspec",
		Command:     "impl",
		EscapeHatch: "override-adr",
		Subcommand:  subTokens[i%len(subTokens)],
		Version:     "1.0." + strconv.Itoa(i), // distinct per-event version
		OS:          runtime.GOOS,
	}
	if err := AppendSuccessEvent(ev); err != nil {
		return 3
	}
	return 0
}

// TestConcurrentProcessAppend_NoLoss spawns N separate OS processes that
// each append a DISTINCT event to ONE shared journal concurrently, then
// asserts ALL N lines survived (no lost-update / no clobbered entry) and
// the file is well-formed JSONL. This is the regression for the
// cross-process lost-update the panel found in the read-modify-rewrite
// design; under append-only O_APPEND each line is position-atomic.
func TestConcurrentProcessAppend_NoLoss(t *testing.T) {
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	const n = 24
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// The helper short-circuits in TestMain on helperEnv before any
			// test runs; -test.run=^$ keeps it from running the suite if the
			// env guard were ever absent.
			cmd := exec.Command(self, "-test.run", "^$")
			cmd.Env = append(os.Environ(),
				helperEnv+"="+strconv.Itoa(i),
				StateDirEnv+"="+dir,
			)
			errs[i] = cmd.Run()
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Fatalf("helper process %d failed: %v", i, e)
		}
	}

	// Read the shared journal directly and STRICTLY parse every line: the
	// file must be well-formed (no torn line) and carry all N appends.
	data, err := os.ReadFile(filepath.Join(dir, journalFileName))
	if err != nil {
		t.Fatalf("read shared journal: %v", err)
	}
	recs, perr := parseAllStrict(data)
	if perr != nil {
		t.Fatalf("journal is not well-formed JSONL after concurrent appends: %v", perr)
	}
	if len(recs) != n {
		t.Errorf("cross-process append lost entries: want %d well-formed lines, got %d", n, len(recs))
	}
	for _, r := range recs {
		if r.Fingerprint == "" || r.Command != "impl" {
			t.Errorf("malformed/clobbered record survived: %+v", r)
		}
	}
}

// parseAllStrict parses every non-empty line and FAILS on any malformed
// line (the well-formedness assertion — distinct from the lenient
// readRecords skip behaviour used in production reads).
func parseAllStrict(data []byte) ([]Record, error) {
	var recs []Record
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	return recs, nil
}
