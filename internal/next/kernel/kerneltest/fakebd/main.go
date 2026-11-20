// Command fakebd serves the serialized fake tracker for binary tests.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func main() {
	if os.Getenv("MINDSPEC_TEST_FULL_BD") == "1" && len(os.Args) > 2 && os.Args[1] == "worktree" && os.Args[2] == "list" {
		data, err := exec.Command("git", "worktree", "list", "--porcelain").Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		rows := []map[string]any{}
		for _, block := range strings.Split(strings.TrimSpace(string(data)), "\n\n") {
			row := map[string]any{}
			for _, line := range strings.Split(block, "\n") {
				key, value, _ := strings.Cut(line, " ")
				if key == "worktree" {
					row["path"] = value
					row["name"] = filepath.Base(value)
				}
				if key == "branch" {
					row["branch"] = strings.TrimPrefix(value, "refs/heads/")
				}
			}
			rows = append(rows, row)
		}
		if err := json.NewEncoder(os.Stdout).Encode(rows); err != nil {
			os.Exit(2)
		}
		return
	}

	path := os.Getenv("MINDSPEC_TEST_FAKE_STORE")
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fake := kerneltest.NewFakeBD()
	if err = json.Unmarshal(data, &fake.Records); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var out []byte
	if os.Getenv("MINDSPEC_TEST_FULL_BD") == "1" && len(os.Args) > 1 && os.Args[1] == "export" {
		ids := make([]string, 0, len(fake.Records))
		for id := range fake.Records {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			row, e := json.Marshal(fake.Records[id])
			if e != nil {
				err = e
				break
			}
			out = append(out, row...)
			out = append(out, '\n')
		}
		for i, arg := range os.Args {
			if arg == "-o" && i+1 < len(os.Args) && err == nil {
				err = os.WriteFile(os.Args[i+1], out, 0600)
				out = nil
				break
			}
		}
	} else if os.Getenv("MINDSPEC_TEST_FULL_BD") == "1" && len(os.Args) > 2 && os.Args[1] == "dolt" && os.Args[2] == "commit" {
		// The serialized store is already durable after each successful write.
		out = []byte("{}")
	} else {
		out, err = fake.Run(os.Args[1:]...)
	}
	if os.Getenv("MINDSPEC_TEST_FULL_BD") == "1" && len(os.Args) > 1 && os.Args[1] == "list" && err == nil {
		typ := ""
		for i, arg := range os.Args[2:] {
			if strings.HasPrefix(arg, "--type=") {
				typ = strings.TrimPrefix(arg, "--type=")
			}
			if arg == "--type" && i+3 < len(os.Args) {
				typ = os.Args[i+3]
			}
		}
		if typ != "" {
			var rows []kernel.Record
			if e := json.Unmarshal(out, &rows); e != nil {
				err = e
			} else {
				filtered := []kernel.Record{}
				for _, row := range rows {
					if row.Type == typ {
						filtered = append(filtered, row)
					}
				}
				out, err = json.Marshal(filtered)
			}
		}
	}
	if os.Getenv("MINDSPEC_TEST_FULL_BD") == "1" && err == nil && len(os.Args) > 1 && (os.Args[1] == "show" || os.Args[1] == "list") {
		var rows []map[string]any
		if e := json.Unmarshal(out, &rows); e != nil {
			err = e
		} else {
			for _, row := range rows {
				parent, _ := row["parent"].(string)
				if p, exists := fake.Records[parent]; exists {
					deps, _ := row["dependencies"].([]any)
					deps = append(deps, map[string]any{"id": parent, "issue_type": p.Type, "dependency_type": "parent-child", "type": "parent-child"})
					row["dependencies"] = deps
				}
			}
			out, err = json.Marshal(rows)
		}
	}
	if err != nil {
		var code interface{ ExitCode() int }
		fmt.Fprintln(os.Stderr, err)
		if errors.As(err, &code) {
			os.Exit(code.ExitCode())
		}
		os.Exit(1)
	}
	if data, err = json.Marshal(fake.Records); err == nil {
		err = os.WriteFile(path, data, 0600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if _, err = os.Stdout.Write(out); err != nil {
		os.Exit(2)
	}
}
