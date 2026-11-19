package kerneltest

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

type ExitError int

func (e ExitError) Error() string { return fmt.Sprintf("bd exited %d", e) }
func (e ExitError) ExitCode() int { return int(e) }

// FakeBD models the local store and records argv. Hook can inject write faults.
type FakeBD struct {
	Records map[string]kernel.Record
	Calls   [][]string
	Hook    func([]string) error
	next    int
}

func NewFakeBD() *FakeBD { return &FakeBD{Records: map[string]kernel.Record{}} }
func (f *FakeBD) Run(args ...string) ([]byte, error) {
	f.Calls = append(f.Calls, append([]string{}, args...))
	if f.Hook != nil {
		if err := f.Hook(args); err != nil {
			return nil, err
		}
	}
	value := func(flag string) string {
		for i, a := range args {
			if a == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	switch args[0] {
	case "list":
		records := []kernel.Record{}
		for _, r := range f.Records {
			if p := value("--parent"); p != "" && r.Parent != p {
				continue
			}
			records = append(records, r)
		}
		sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
		return json.Marshal(records)
	case "show":
		r, ok := f.Records[args[1]]
		if !ok {
			return nil, ExitError(1)
		}
		return json.Marshal([]kernel.Record{r})
	case "create":
		id := value("--id")
		if id == "" {
			f.next++
			id = "test-" + strconv.Itoa(f.next)
			if parent := value("--parent"); parent != "" {
				for n := 1; ; n++ {
					candidate := parent + "." + strconv.Itoa(n)
					if _, exists := f.Records[candidate]; !exists {
						id = candidate
						break
					}
				}
			}
		}
		if _, ok := f.Records[id]; ok {
			return nil, ExitError(1)
		}
		title := value("--title")
		if title == "" && len(args) > 1 && !strings.HasPrefix(args[1], "--") {
			title = args[1]
		}
		r := kernel.Record{ID: id, Parent: value("--parent"), Title: title, Description: value("--description"), Type: value("--type"), Status: "open", Metadata: map[string]any{}}
		if m := value("--metadata"); m != "" {
			if err := json.Unmarshal([]byte(m), &r.Metadata); err != nil {
				return nil, err
			}
		}
		if labels := value("--labels"); labels != "" {
			r.Labels = strings.Split(labels, ",")
		}
		f.Records[id] = r
		return json.Marshal(r)
	case "update":
		r, ok := f.Records[args[1]]
		if !ok {
			return nil, ExitError(1)
		}
		if expected := value("--if-status"); expected != "" && r.Status != expected {
			return nil, ExitError(13)
		}
		if title := value("--title"); title != "" {
			r.Title = title
		}
		if label := value("--add-label"); label != "" {
			found := false
			for _, old := range r.Labels {
				found = found || old == label
			}
			if !found {
				r.Labels = append(r.Labels, label)
			}
		}
		if typ := value("--type"); typ != "" {
			r.Type = typ
		}
		if status := value("--status"); status != "" {
			r.Status = status
		}
		for _, arg := range args {
			if arg == "--claim" {
				r.Status = "in_progress"
			}
		}
		if m := value("--metadata"); m != "" {
			if err := json.Unmarshal([]byte(m), &r.Metadata); err != nil {
				return nil, err
			}
		}
		f.Records[r.ID] = r
		return json.Marshal(r)
	case "close":
		r, ok := f.Records[args[1]]
		if !ok {
			return nil, ExitError(1)
		}
		if !slices.Contains(args, "--force") {
			for _, child := range f.Records {
				if child.Parent == r.ID && child.Status != "closed" {
					return nil, fmt.Errorf("open child %s", child.ID)
				}
			}
		}
		r.Status = "closed"
		f.Records[r.ID] = r
		return json.Marshal(r)
	case "dep":
		if args[1] == "list" {
			r, ok := f.Records[args[2]]
			if !ok {
				return nil, ExitError(1)
			}
			return json.Marshal(r.Dependencies)
		}
		if args[1] != "add" {
			return nil, fmt.Errorf("unsupported dependency operation")
		}
		r, ok := f.Records[args[2]]
		if !ok {
			return nil, ExitError(1)
		}
		for _, d := range r.Dependencies {
			if d.ID == args[3] && d.Type == value("--type") {
				return []byte(`{}`), nil
			}
		}
		r.Dependencies = append(r.Dependencies, kernel.Dependency{ID: args[3], Type: value("--type")})
		f.Records[r.ID] = r
		return []byte(`{}`), nil
	}
	return nil, fmt.Errorf("unsupported fake bd command %v", args)
}
