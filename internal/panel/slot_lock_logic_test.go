package panel

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type fakeLockFS struct {
	files        map[string]string
	live         map[int]bool
	beforeRename func()
	beforeLink   func()
	removed      []string
}

func (f *fakeLockFS) ops() staleLockOps {
	return staleLockOps{
		create: func(path string, pid int) error {
			if _, ok := f.files[path]; ok {
				return os.ErrExist
			}
			f.files[path] = strconv.Itoa(pid)
			return nil
		},
		read: func(path string) ([]byte, error) {
			raw, ok := f.files[path]
			if !ok {
				return nil, os.ErrNotExist
			}
			return []byte(raw), nil
		},
		rename: func(a, b string) error {
			if f.beforeRename != nil {
				f.beforeRename()
			}
			raw, ok := f.files[a]
			if !ok {
				return os.ErrNotExist
			}
			f.files[b] = raw
			delete(f.files, a)
			return nil
		},
		link: func(a, b string) error {
			if f.beforeLink != nil {
				f.beforeLink()
			}
			if _, ok := f.files[b]; ok {
				return os.ErrExist
			}
			raw, ok := f.files[a]
			if !ok {
				return os.ErrNotExist
			}
			f.files[b] = raw
			return nil
		},
		remove: func(path string) error {
			if raw, ok := f.files[path]; ok {
				pid, valid := lockPID([]byte(raw))
				if valid && f.live[pid] && f.files["lock"] != raw {
					return errors.New("deleted a live holder")
				}
			}
			f.removed = append(f.removed, path)
			delete(f.files, path)
			return nil
		},
		live: func(pid int) bool { return f.live[pid] }, unique: func(path string) string { return path + ".moved" },
	}
}
func TestSlotLock_StaleLogic(t *testing.T) {
	for _, name := range []string{"dead pid freed", "empty file held", "unparseable file held", "live pid never deleted", "rename reread new pid relinks", "relink race leaves one holder", "link fails moved pid live names both"} {
		t.Run(name, func(t *testing.T) {
			f := &fakeLockFS{files: map[string]string{"lock": "11"}, live: map[int]bool{22: true, 33: true, 44: true}}
			switch name {
			case "empty file held":
				f.files["lock"] = ""
			case "unparseable file held":
				f.files["lock"] = "bad"
			case "live pid never deleted":
				f.files["lock"] = "22"
			case "rename reread new pid relinks":
				f.beforeRename = func() { f.files["lock"] = "22" }
			case "relink race leaves one holder":
				f.beforeRename = func() { f.files["lock"] = "55" }
				f.beforeLink = func() { f.files["lock"] = "33" }
			case "link fails moved pid live names both":
				f.beforeRename = func() { f.files["lock"] = "22" }
				f.beforeLink = func() { f.files["lock"] = "33" }
			}
			acquired, err := decideStaleLock("lock", 44, f.ops())
			switch name {
			case "dead pid freed":
				if err != nil || !acquired || f.files["lock"] != "44" || len(f.files) != 1 {
					t.Fatalf("did not free dead pid: %v %v %+v", acquired, err, f.files)
				}
			case "empty file held", "unparseable file held", "live pid never deleted":
				if err != nil || acquired || len(f.removed) != 0 || len(f.files) != 1 {
					t.Fatalf("changed held lock: %v %v %+v", acquired, err, f)
				}
			case "rename reread new pid relinks":
				if err != nil || acquired || f.files["lock"] != "22" || len(f.files) != 1 {
					t.Fatalf("did not restore replacement: %v %v %+v", acquired, err, f.files)
				}
			case "relink race leaves one holder":
				if err != nil || acquired || f.files["lock"] != "33" || len(f.files) != 1 {
					t.Fatalf("race lost holder: %v %v %+v", acquired, err, f.files)
				}
			case "link fails moved pid live names both":
				if err == nil || acquired || !strings.Contains(err.Error(), "lock and lock.moved") || f.files["lock.moved"] != "22" || f.files["lock"] != "33" || len(f.removed) != 0 {
					t.Fatalf("lost live inode: %v %v %+v", acquired, err, f)
				}
			}
		})
	}
}
func missingStaleDecisionCall(path string) (bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return false, err
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if ok && id.Name == "decideStaleLock" {
			found = true
		}
		return true
	})
	return !found, nil
}
func TestSlotLock_WindowsUsesDecisionLogic(t *testing.T) {
	missing, err := missingStaleDecisionCall("slot_lock_windows.go")
	if err != nil || missing {
		t.Fatalf("Windows bypasses stale decision logic: %v", err)
	}
	t.Run("negative fixture", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "binding.go")
		if err := os.WriteFile(path, []byte("package panel\nfunc try() { println(\"decideStaleLock\") }\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		missing, err := missingStaleDecisionCall(path)
		if err != nil || !missing {
			t.Fatalf("detector missed bypass: %v %v", missing, err)
		}
	})
}
