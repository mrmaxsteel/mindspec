package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// enrolStageHook observes completed write groups. Production leaves it unset.
var enrolStageHook func(stage string)

func enrolStage(stage string) {
	if enrolStageHook != nil {
		enrolStageHook(stage)
	}
}

func approveEnrolment(root, specID, approvedBy, specDir, planPath string, result *PlanResult, outcome Outcome, exec executor.Executor) (*PlanResult, error) {
	worktree := workspace.TreeRootForSpecDir(specDir)
	err := phase.WithPhaseLock(root, func() error {
		data, err := os.ReadFile(planPath)
		if err != nil {
			return err
		}
		content := string(data)
		fm, err := validate.ParsePlanFrontmatter(content)
		if err != nil {
			return err
		}
		sections := validate.ParseBeadSections(content)
		if err = validate.ValidateWorkChunkAlignment(fm.WorkChunks, len(sections)); err != nil {
			return err
		}
		if err = checkBeadSectionContents(content, sections); err != nil {
			return err
		}
		var metadata map[string]interface{}
		epic, err := resolveTargetEpic(specID, &metadata)
		if err != nil {
			return err
		}
		store := kernel.NewStore(root, epic, planRunBDFn)
		records, err := store.All()
		if err != nil {
			return err
		}
		enrolled, err := enrolWitness(specDir, epic, records)
		if err != nil {
			return err
		}
		_, runPresent := metadata[kernel.KeyRun]
		approved, _, err := kernel.Decode[kernel.PlanApproval](metadata, kernel.KeyPlanApproval)
		if err != nil {
			return err
		}
		if enrolled && (bool(approved) || strings.EqualFold(fm.Status, "Approved")) {
			return guard.NewFailure("plan approval already belongs to an enrolment", "mindspec next-action "+specID+" --apply")
		}
		vr := validate.ValidatePlan(root, specID)
		// Item existence is checked against this operation's fresh store snapshot.
		// A retry with no surviving recorded item discards stale recorded identities.
		issues := vr.Issues[:0]
		for _, issue := range vr.Issues {
			if issue.Name != "bead-id-missing" {
				issues = append(issues, issue)
			}
		}
		vr.Issues = issues
		if vr.HasFailures() {
			return planValidationFailure(specID, vr)
		}
		// An approval without any witness belongs to the earlier lifecycle.
		if !enrolled && (bool(approved) || strings.EqualFold(fm.Status, "Approved")) {
			result.BeadIDs = append([]string{}, fm.BeadIDs...)
			return nil
		}
		if err = kernel.RequireStepType(root); err != nil {
			return err
		}
		if err = checkPlanResolutions(worktree, specID, content, metadata, exec); err != nil {
			return err
		}
		ids, err := enrolCompare(fm, content, sections, records, metadata)
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		// Check existing gate identities before any enrolment write.
		for _, record := range records {
			for _, key := range []string{record.ID, record.Key()} {
				tuple, parseErr := kernelid.Parse(key)
				if parseErr == nil && tuple.Epic == epic && (tuple.Kind == "bead-gate" || key == epic+".final") {
					if err = store.Collision(key, tuple); err != nil {
						return err
					}
				}
			}
		}
		if err = enrolMarkEpic(store); err != nil {
			return err
		}
		enrolStage("epic")
		specContent := readFileOrEmpty(filepath.Join(specDir, "spec.md"))
		requirements := contextpack.ExtractSection(specContent, "Requirements")
		fallbackAC := contextpack.ExtractSection(specContent, "Acceptance Criteria")
		citationIDs := make([]string, 0, len(fm.ADRCitations))
		for _, citation := range fm.ADRCitations {
			citationIDs = append(citationIDs, citation.ID)
		}
		design := buildDesignField(specDir, requirements, citationIDs)
		bodies := extractBeadSectionContents(content)
		for i, section := range sections {
			if i < len(ids) && ids[i] != "" {
				continue
			}
			paths := enrolChunk(fm.WorkChunks, i+1).KeyFilePaths
			acceptance := section.AcceptanceCriteria
			if acceptance == "" {
				acceptance = fallbackAC
			}
			args := []string{"create", "--title", "[mindspec] [" + specID + "] " + section.Heading, "--type", "task", "--parent", epic, "--labels", "mindspec", "--description", bodies[section.Heading], "--acceptance", acceptance, "--design", design, "--metadata", buildBeadMetadata(specID, paths), "--json"}
			out, err := planRunBDFn(args...)
			if err != nil {
				return err
			}
			var record kernel.Record
			if err = json.Unmarshal(out, &record); err != nil {
				return err
			}
			if err = idvalidate.BeadID(record.ID); err != nil {
				return err
			}
			for len(ids) <= i {
				ids = append(ids, "")
			}
			ids[i] = record.ID
			if err = writeBeadIDsToFrontmatter(planPath, ids); err != nil {
				return err
			}
			enrolStage("item")
		}
		enrolStage("creates")
		links := enrolLinks(ids, fm.WorkChunks)
		if err = store.WriteMetadata(epic, kernel.Metadata(kernel.KeyLinks, links)); err != nil {
			return err
		}
		enrolStage("links-record")
		graph, err := enrolGraph(store, append([]string{epic + ".final"}, ids...))
		if err != nil {
			return err
		}
		// Check the entire proposed graph before writing the first plan link.
		proposed := enrolWaits{}
		for down, ups := range graph {
			proposed[down] = append([]string{}, ups...)
		}
		for _, link := range links {
			for _, upstream := range link.Upstream {
				if err = proposed.check(link.ID, upstream); err != nil && !runPresent {
					return err
				}
				if err == nil {
					proposed.add(link.ID, upstream)
				}
			}
		}
		for _, link := range links {
			for _, upstream := range link.Upstream {
				if err = enrolAddLink(store, graph, link.ID, upstream); err != nil {
					if runPresent {
						result.Warnings = append(result.Warnings, err.Error())
						continue
					}
					return err
				}
			}
		}
		enrolStage("links")
		if !runPresent {
			policy := kernel.Policy{RoundLimit: cfg.RoundLimit(), LeaseTTL: cfg.Panel.LeaseTTL, Rules: "kernel-1"}
			if err = store.WriteMetadata(epic, kernel.Metadata(kernel.KeyRun, policy)); err != nil {
				return err
			}
		}
		enrolStage("run")
		if err = store.WriteMetadata(epic, kernel.Metadata(kernel.KeyPlanApproval, kernel.PlanApproval(true))); err != nil {
			return err
		}
		if err = updatePlanApproval(planPath, approvedBy); err != nil {
			return err
		}
		enrolStage("approval")
		for _, item := range ids {
			if err = enrolGate(store, graph, item+".review", item); err != nil {
				return err
			}
			enrolStage("gate")
		}
		if err = enrolGate(store, graph, epic+".final", epic); err != nil {
			return err
		}
		enrolStage("final")
		for _, item := range ids {
			if err = enrolAddLink(store, graph, epic+".final", item); err != nil {
				result.Warnings = append(result.Warnings, err.Error())
			}
		}
		enrolStage("final-links")
		if err = recordLifecycleGateAudit(epic, outcome, nil); err != nil {
			return documentGateAuditFailure(err)
		}
		if err = exec.CommitAll(worktree, "chore: approve plan for "+specID); err != nil {
			return err
		}
		if err = exec.CommitAll(worktree, "chore: sync tracker state after plan approval"); err != nil {
			return err
		}
		if err = exec.IsTreeClean(worktree); err != nil {
			return err
		}
		result.BeadIDs = ids
		return exec.HandoffEpic(epic, specID, ids)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func enrolWitness(specDir, epic string, records []kernel.Record) (bool, error) {
	w := kernelid.Witnesses{Epic: epic}
	for _, r := range records {
		if r.ID == epic {
			_, w.RunPresent = r.Metadata[kernel.KeyRun]
		}
		w.IDs = append(w.IDs, r.ID)
		w.Keys = append(w.Keys, r.Key())
	}
	var err error
	w.Orchestrated, err = kernelid.PanelOrchestrated(specDir)
	enrolled, _ := kernelid.Enrolled(w)
	return enrolled, err
}

func enrolChunk(chunks []validate.WorkChunk, n int) validate.WorkChunk {
	for _, chunk := range chunks {
		if chunk.ID == n {
			return chunk
		}
	}
	return validate.WorkChunk{ID: n}
}

func enrolLinks(ids []string, chunks []validate.WorkChunk) kernel.Links {
	links := kernel.Links{}
	for i, id := range ids {
		link := kernel.PlanLink{ID: id, Upstream: []string{}}
		for _, dep := range enrolChunk(chunks, i+1).DependsOn {
			link.Upstream = append(link.Upstream, ids[dep-1])
		}
		links = append(links, link)
	}
	return links
}

// enrolCompare checks every durable item before the first write of a retry.
func enrolCompare(fm *validate.PlanFrontmatter, content string, sections []validate.BeadSection, records []kernel.Record, metadata map[string]interface{}) ([]string, error) {
	byID := map[string]kernel.Record{}
	for _, r := range records {
		byID[r.ID] = r
	}
	ids := append([]string{}, fm.BeadIDs...)
	anyRecorded := false
	for _, id := range ids {
		if _, ok := byID[id]; ok {
			anyRecorded = true
		}
	}
	if !anyRecorded {
		return nil, nil
	}
	if len(ids) > len(sections) {
		return nil, enrolEdit(fm.SpecID, "recorded item has no chunk: "+ids[len(sections)])
	}
	bodies := extractBeadSectionContents(content)
	for i, id := range ids {
		if id == "" {
			continue
		}
		r, ok := byID[id]
		if !ok {
			return nil, enrolEdit(fm.SpecID, "recorded item is missing: "+id)
		}
		section := sections[i]
		title := "[mindspec] [" + fm.SpecID + "] " + section.Heading
		if r.Title != title || strings.TrimSpace(r.Description) != bodies[section.Heading] {
			return nil, enrolEdit(fm.SpecID, "chunk text differs for "+id)
		}
		var paths struct {
			Paths []string `json:"file_paths"`
		}
		raw, err := json.Marshal(r.Metadata)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &paths); err != nil {
			return nil, err
		}
		if !slices.Equal(paths.Paths, enrolChunk(fm.WorkChunks, i+1).KeyFilePaths) {
			return nil, enrolEdit(fm.SpecID, "chunk paths differ for "+id)
		}
	}
	written, _, err := kernel.Decode[kernel.Links](metadata, kernel.KeyLinks)
	if err != nil {
		return nil, err
	}
	for _, link := range written {
		i := slices.Index(ids, link.ID)
		if i < 0 {
			return nil, enrolEdit(fm.SpecID, "recorded item has no chunk: "+link.ID)
		}
		want := []string{}
		for _, dep := range enrolChunk(fm.WorkChunks, i+1).DependsOn {
			if dep > len(ids) {
				return nil, enrolEdit(fm.SpecID, "upstream differs for "+link.ID)
			}
			want = append(want, ids[dep-1])
		}
		if !slices.Equal(link.Upstream, want) {
			return nil, enrolEdit(fm.SpecID, "plan-order upstream link differs for "+link.ID+": "+strings.Join(want, ","))
		}
	}
	return ids, nil
}

func enrolEdit(specID, detail string) error {
	return guard.NewFailure("plan changed: "+termsafe.Escape(detail)+"; restore the recorded plan before retrying approval", "mindspec plan approve "+idrender.Spec(specID))
}

func enrolMarkEpic(store *kernel.Store) error {
	epic, err := store.Lookup(store.Epic)
	if err != nil {
		return err
	}
	if epic == nil {
		return fmt.Errorf("epic is absent")
	}
	title := epic.Title
	if !strings.HasPrefix(title, "[mindspec]") {
		title = "[mindspec] " + title
	}
	if title == epic.Title && slices.Contains(epic.Labels, "mindspec") {
		return nil
	}
	_, err = store.Run("update", epic.ID, "--title", title, "--add-label", "mindspec")
	return err
}

func enrolGate(store *kernel.Store, graph enrolWaits, id, parent string) error {
	tuple, err := kernelid.Parse(id)
	if err != nil {
		return err
	}
	record, err := store.Create(kernel.CreateRecord{ID: id, Parent: parent, Type: "step", Title: tuple.Kind, Tuple: tuple})
	if err == nil {
		graph.add(record.Parent, record.ID)
	}
	return err
}

type enrolEdge struct{ down, up string }
type enrolWaits map[string][]string

func (g enrolWaits) add(down, up string) {
	if !slices.Contains(g[down], up) {
		g[down] = append(g[down], up)
	}
}
func (g enrolWaits) path(from, to string, seen map[string]bool) []enrolEdge {
	if seen[from] {
		return nil
	}
	seen[from] = true
	for _, next := range g[from] {
		edge := enrolEdge{from, next}
		if next == to {
			return []enrolEdge{edge}
		}
		if rest := g.path(next, to, seen); rest != nil {
			return append([]enrolEdge{edge}, rest...)
		}
	}
	return nil
}
func (g enrolWaits) check(down, up string) error {
	if slices.Contains(g[down], up) {
		return nil
	}
	if down == up {
		return fmt.Errorf("cycle at link %s -> %s", termsafe.Escape(down), termsafe.Escape(up))
	}
	chain := g.path(up, down, map[string]bool{})
	if len(chain) == 0 {
		return nil
	}
	// The first existing link from the proposed upstream starts the cycle.
	link := chain[0]
	return guard.NewFailure(fmt.Sprintf("foreign-blocker: link %s -> %s forms a cycle through %s -> %s", termsafe.Escape(down), termsafe.Escape(up), termsafe.Escape(link.down), termsafe.Escape(link.up)), fmt.Sprintf("bd dep remove %s %s", idrender.Bead(link.down), idrender.Bead(link.up)))
}
func enrolGraph(store *kernel.Store, starts []string) (enrolWaits, error) {
	records, err := store.All()
	if err != nil {
		return nil, err
	}
	graph := enrolWaits{}
	byID := map[string]kernel.Record{}
	for _, record := range records {
		byID[record.ID] = record
		byID[record.Key()] = record
		if record.Parent != "" {
			graph.add(record.Parent, record.ID)
		}
	}
	pending := append([]string{}, starts...)
	seen := map[string]bool{}
	for len(pending) > 0 {
		id := pending[0]
		pending = pending[1:]
		record, exists := byID[id]
		if !exists || seen[record.ID] {
			continue
		}
		seen[record.ID] = true
		out, err := store.Run("dep", "list", record.ID, "--json")
		if err != nil {
			return nil, err
		}
		var deps []struct {
			ID             string `json:"id"`
			Up             string `json:"depends_on_id"`
			Type           string `json:"type"`
			DependencyType string `json:"dependency_type"`
		}
		if err = json.Unmarshal(out, &deps); err != nil {
			return nil, err
		}
		for _, dep := range deps {
			up := dep.ID
			if dep.Up != "" {
				up = dep.Up
			}
			typ := dep.Type
			if typ == "" {
				typ = dep.DependencyType
			}
			switch typ {
			case "blocks":
				graph.add(record.ID, up)
			case "parent-child":
				graph.add(up, record.ID)
			}
		}
		pending = append(pending, graph[record.ID]...)
	}
	return graph, nil
}
func enrolAddLink(store *kernel.Store, graph enrolWaits, down, up string) error {
	d, err := store.Lookup(down)
	if err != nil {
		return err
	}
	u, err := store.Lookup(up)
	if err != nil {
		return err
	}
	if d == nil || u == nil {
		return fmt.Errorf("link endpoint is missing")
	}
	if slices.Contains(graph[d.ID], u.ID) {
		return nil
	}
	if err = graph.check(d.ID, u.ID); err != nil {
		return err
	}
	if err = store.AddLink(down, up); err != nil {
		return guard.NewFailure(fmt.Sprintf("link %s -> %s refused: %s", termsafe.Escape(down), termsafe.Escape(up), termsafe.Escape(err.Error())), fmt.Sprintf("bd dep list %s --json", idrender.Bead(down)))
	}
	graph.add(d.ID, u.ID)
	return nil
}
