package panel

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// ArtifactPathDetails annotates unresolved findings against the round event's
// tree. It never changes a decision or reads the rebound snapshot's commit.
// Discharged findings need no tree read. A read failure retains the refusal
// and names the paths whose presence could not be checked.
func ArtifactPathDetails(r *Result, workdir string, lsTree func(string, string) (string, error)) string {
	var labels, paths []string
	for _, v := range r.UnresolvedVerdicts() {
		for _, id := range r.unresolvedFindings(v) {
			if name := v.ArtifactPaths[id]; name != "" {
				labels = append(labels, fmt.Sprintf("%s %s: %s", termsafe.Escape(v.Slot), termsafe.Escape(id), termsafe.Escape(name)))
				paths = append(paths, path.Clean(name))
			}
		}
	}
	if len(paths) == 0 {
		return ""
	}
	var listing string
	var err error
	if lsTree == nil || r.RoundReviewedHeadSHA == "" {
		err = fmt.Errorf("round tree reader or commit unavailable")
	} else {
		listing, err = lsTree(workdir, r.RoundReviewedHeadSHA)
	}
	present := map[string]bool{}
	if err == nil {
		for _, line := range strings.Split(listing, "\n") {
			if line == "" {
				continue
			}
			metadata, name, ok := strings.Cut(line, "\t")
			if !ok || len(strings.Fields(metadata)) != 3 || name == "" {
				err = fmt.Errorf("malformed round tree listing")
				break
			}
			if strings.HasPrefix(name, "\"") {
				name, err = strconv.Unquote(name)
				if err != nil {
					break
				}
			}
			for name != "." && name != "/" {
				present[name] = true
				name = path.Dir(name)
			}
			present["."] = true
		}
	}
	if err != nil {
		return "; artifact paths not verified in round tree: " + strings.Join(labels, ", ") + " (" + termsafe.Escape(err.Error()) + ")"
	}
	var absent []string
	for i, name := range paths {
		if !present[name] {
			absent = append(absent, labels[i])
		}
	}
	if len(absent) == 0 {
		return ""
	}
	return "; artifact paths absent from round tree: " + strings.Join(absent, ", ")
}
