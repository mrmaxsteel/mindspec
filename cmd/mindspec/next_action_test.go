package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

// The CLI uses a file-backed local fake. No call can reach a real tracker store.
const nextActionFake = `#!/usr/bin/env python3
import json, os, sys
args=sys.argv[1:]
if os.environ.get('S134_FAIL')=='1': sys.exit(1)
path=os.environ['S134_DATA']
with open(path) as f: rows=json.load(f)
def val(flag, default=''):
 return args[args.index(flag)+1] if flag in args else default
def save():
 with open(path,'w') as f: json.dump(rows,f,sort_keys=True)
def row(id):
 return next(r for r in rows if r['id']==id)
if args[0]=='list': print(json.dumps(rows,sort_keys=True));sys.exit(0)
if args[0]=='create':
 parent=val('--parent');n=1
 while any(r['id']==parent+'.'+str(n) for r in rows): n+=1
 r={'id':parent+'.'+str(n),'parent':parent,'title':args[1],'issue_type':val('--type'),'status':'open','metadata':json.loads(val('--metadata','{}')),'labels':['mindspec']}
 rows.append(r);save();print(json.dumps(r));sys.exit(0)
if args[0]=='update':
 r=row(args[1])
 if val('--if-status') and r['status']!=val('--if-status'):sys.exit(13)
 if val('--metadata'):r['metadata']=json.loads(val('--metadata'))
 if val('--status'):r['status']=val('--status')
 save();print(json.dumps(r));sys.exit(0)
if args[:2]==['dep','add']:
 r=row(args[2]);r.setdefault('dependencies',[]).append({'id':args[3],'type':val('--type')});save();print('{}');sys.exit(0)
sys.exit(1)
`

func nextActionFixture(t *testing.T) *kerneltest.Fixture {
	t.Helper()
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	if err := os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{".beads/config.yaml": "types.custom: gate,step\n", ".gitignore": ".mindspec/phase.lock\n"} {
		if err := os.WriteFile(filepath.Join(f.Root, path), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	data := filepath.Join(t.TempDir(), "store.json")
	t.Setenv("S134_DATA", data)
	installFakeBD(t, nextActionFake)
	nextActionSave(t, f)
	out, err := f.Command("git", "branch", "spec/"+f.SpecID)
	if err != nil {
		t.Fatalf("branch: %s %v", out, err)
	}
	for _, item := range f.Items {
		_, err = f.Command("git", "branch", "bead/"+item)
		f.Must(err)
	}
	return f
}
func nextActionSave(t *testing.T, f *kerneltest.Fixture) {
	t.Helper()
	rows, err := f.Store.All()
	f.Must(err)
	data, err := json.Marshal(rows)
	f.Must(err)
	f.Must(os.WriteFile(os.Getenv("S134_DATA"), data, 0644))
}
func nextActionRun(t *testing.T, f *kerneltest.Fixture, bin string, args ...string) ([]byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, append([]string{"next-action", "x"}, args...)...)
	cmd.Dir = f.Root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false", "BEADS_DIR="+filepath.Join(f.Root, ".beads"))
	// Keep terminal diagnostics separate from the action JSON document.
	out, err := cmd.Output()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	if err == nil {
		return out, 0
	}
	var e *exec.ExitError
	if !errors.As(err, &e) {
		t.Fatal(err)
	}
	return append(out, e.Stderr...), e.ExitCode()
}
func TestS134NextAction_ReadOnlyApplyAndRefusals(t *testing.T) {
	bin := buildMindspecBinary(t)
	t.Run("read-only-apply", func(t *testing.T) {
		f := nextActionFixture(t)
		before, err := os.ReadFile(os.Getenv("S134_DATA"))
		f.Must(err)
		snapshot := kerneltest.Snapshot(t, f)
		a, code := nextActionRun(t, f, bin, "--json")
		b, again := nextActionRun(t, f, bin, "--json")
		after, err := os.ReadFile(os.Getenv("S134_DATA"))
		f.Must(err)
		var out kernel.Output
		f.Must(json.Unmarshal(a, &out))
		if code != 0 || again != 0 || string(a) != string(b) || string(before) != string(after) || kerneltest.Snapshot(t, f) != snapshot || len(out.Actions) != 1 || out.Actions[0].Exists || out.Actions[0].Kind != "implement" {
			t.Fatalf("read-only CLI: codes %d %d %s", code, again, a)
		}
		a, code = nextActionRun(t, f, bin, "--apply", "--json")
		f.Must(json.Unmarshal(a, &out))
		revision := out.Revision
		if code != 0 || len(out.Actions) != 1 || !out.Actions[0].Exists {
			t.Fatalf("CLI apply: %d %s", code, a)
		}
		before, err = os.ReadFile(os.Getenv("S134_DATA"))
		f.Must(err)
		a, code = nextActionRun(t, f, bin, "--apply", "--json")
		f.Must(json.Unmarshal(a, &out))
		after, err = os.ReadFile(os.Getenv("S134_DATA"))
		f.Must(err)
		if code != 0 || out.Revision != revision || string(before) != string(after) {
			t.Fatalf("CLI replay changed: %d %s", code, a)
		}
	})
	for _, mode := range []string{"silent", "missing-step", "unapproved"} {
		t.Run(mode, func(t *testing.T) {
			f := nextActionFixture(t)
			if mode == "missing-step" {
				f.Must(os.WriteFile(filepath.Join(f.Root, ".beads", "config.yaml"), []byte("types.custom: gate\n"), 0644))
			}
			if mode == "unapproved" {
				r := f.Fake.Records[f.Epic]
				delete(r.Metadata, kernel.KeyPlanApproval)
				f.Fake.Records[f.Epic] = r
				nextActionSave(t, f)
			}
			before, err := os.ReadFile(os.Getenv("S134_DATA"))
			f.Must(err)
			snapshot := kerneltest.Snapshot(t, f)
			if mode == "silent" {
				t.Setenv("S134_FAIL", "1")
			}
			out, code := nextActionRun(t, f, bin, "--apply", "--json")
			after, err := os.ReadFile(os.Getenv("S134_DATA"))
			f.Must(err)
			if code == 0 || string(before) != string(after) || snapshot != kerneltest.Snapshot(t, f) {
				t.Fatalf("CLI refusal wrote: %d %s", code, out)
			}
			if mode == "missing-step" && !strings.Contains(string(out), "mindspec setup") {
				t.Fatalf("missing recovery: %s", out)
			}
		})
	}
}
func TestS134NextAction_StatusExits(t *testing.T) {
	bin := buildMindspecBinary(t)
	t.Run("done", func(t *testing.T) {
		f := nextActionFixture(t)
		head, err := f.Command("git", "rev-parse", "HEAD")
		f.Must(err)
		accepted := strings.TrimSpace(string(head))
		for _, item := range f.Items {
			f.Must(f.Store.WriteMetadata(item, map[string]any{kernel.KeyCompletion: accepted}))
			kerneltest.PassGate(f, item+".review", accepted)
			f.Must(f.Store.SetStatusIf(item, "open", "closed"))
		}
		kerneltest.PassGate(f, f.Epic+".final", accepted)
		f.Must(f.Store.SetStatusIf(f.Epic, "open", "closed"))
		nextActionSave(t, f)
		out, code := nextActionRun(t, f, bin)
		if code != 5 || !strings.Contains(string(out), `"status":"done"`) {
			t.Fatalf("done exit %d: %s", code, out)
		}
	})
	t.Run("waiting", func(t *testing.T) {
		f := nextActionFixture(t)
		id := f.Items[0] + ".impl.a1"
		f.Create(id, f.Items[0])
		expires := time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC)
		f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, kernel.LeaseRecord{Generation: 2, Expires: expires})))
		nextActionSave(t, f)
		out, code := nextActionRun(t, f, bin, "--json")
		if code != 3 || !strings.Contains(string(out), `"status":"wait"`) || !strings.Contains(string(out), expires.Format(time.RFC3339)) {
			t.Fatalf("wait exit %d: %s", code, out)
		}
	})
}

func TestS134NextAction_MalformedLedgerRecovery(t *testing.T) {
	bin := buildMindspecBinary(t)
	f := nextActionFixture(t)
	dir := filepath.Join(f.Root, ".mindspec", "reviews", "broken")
	f.Must(os.MkdirAll(dir, 0755))
	resolvedDir, err := filepath.EvalSymlinks(dir)
	f.Must(err)
	f.Must(os.WriteFile(filepath.Join(dir, "dispositions.jsonl"), []byte("{not json\n"), 0644))
	for _, args := range [][]string{{"--json"}, {"--apply", "--json"}} {
		out, code := nextActionRun(t, f, bin, args...)
		if code == 0 || !strings.Contains(string(out), dir) || !strings.Contains(string(out), "\nrecovery: in "+resolvedDir+", repair the ledger or move it out of the review root, then rerun the command") {
			t.Fatalf("ledger recovery lacks repair: exit %d: %s", code, out)
		}
	}
}
