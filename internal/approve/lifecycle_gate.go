package approve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/safefile"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Outcome holds the gate decision and the audit due after epic resolution.
// A disabled gate has no audit. A refusal must not be persisted as a pass.
type Outcome struct {
	panel.Decision
	Gate          string
	Skipped       bool
	Abandoned     bool
	Absent        bool
	AbandonReason string
	// LivePanelSlug names the first non-abandoned panel that passed all checks.
	LivePanelSlug string
}

var lifecycleExecutorFn = func(root string) executor.Executor { return executor.NewMindspecExecutor(root) }
var lifecycleMergeMetadataFn = bead.MergeMetadata

// SetLifecycleMergeMetadataForTest replaces the shared audit writer in tests.
func SetLifecycleMergeMetadataForTest(fn func(string, map[string]interface{}) error) func() {
	orig := lifecycleMergeMetadataFn
	lifecycleMergeMetadataFn = fn
	return func() { lifecycleMergeMetadataFn = orig }
}

// runLifecycleGate reads all registrations for an approval, without mutation.
func runLifecycleGate(root, specID, gate, worktree string, out io.Writer) (Outcome, error) {
	o := Outcome{Gate: gate}
	recovery := fmt.Sprintf("mindspec panel create <slug> --spec %s --gate %s --target spec/%s", idrender.Spec(specID), termsafe.Escape(gate), idrender.Spec(specID))
	refuse := func(slug, leg string, err error) (Outcome, error) {
		o.Action = panel.Block
		detail, _, _ := strings.Cut(err.Error(), "\nrecovery:")
		o.Message = fmt.Sprintf("gate %s panel %s: %s: %s\nrecovery: %s", termsafe.Escape(gate), termsafe.Escape(slug), leg, termsafe.Escape(detail), recovery)
		return o, fmt.Errorf("%s", o.Message)
	}
	warn := func(message string) {
		o.Action = panel.Warn
		o.Message = message
		if out != nil {
			fmt.Fprintf(out, "warning: %s\n", message)
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		return refuse("<slug>", "config", err)
	}
	if !cfg.Enforcement.PanelGate {
		return o, nil
	}
	if os.Getenv(panel.SkipPanelEnv) == "1" {
		o.Skipped = true
		warn(fmt.Sprintf("gate %s panel <slug>: panel gate skipped", termsafe.Escape(gate)))
		return o, nil
	}
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return refuse("<slug>", "subject", err)
	}
	exec := lifecycleExecutorFn(root)
	regs := lifecycleRegistrations(panel.Scan(complete.PanelGateRoots(root, worktree, specID)...), specID, gate)
	if len(regs) == 0 {
		if cfg.Enforcement.PanelPresence == "enforce" {
			return refuse("<slug>", "presence", fmt.Errorf("no registration"))
		}
		o.Absent = true
		warn(fmt.Sprintf("gate %s panel <slug>: no registration; recovery: %s", termsafe.Escape(gate), recovery))
		return o, nil
	}
	for _, reg := range regs {
		if reg.Err != nil {
			return refuse(reg.Slug(), "registration", reg.Err)
		}
		res, err := panel.Tally(reg.Dir)
		if err != nil {
			return refuse(reg.Slug(), "registration", err)
		}
		if res.IntegrityErr != nil {
			return refuse(reg.Slug(), "integrity", fmt.Errorf("panel.json changed outside the panel verbs (%v); recorded target %s", res.IntegrityErr, reg.Panel.Target))
		}
		// Integrity precedes abandonment, as in the shared decision engine.
		facts := panel.GateFacts{Reg: &reg, Res: res, WorktreeAbsent: true}
		if !reg.Panel.Abandoned {
			if err := panel.CheckLifecycleTarget(gate, specID, reg.Panel.Target); err != nil {
				return refuse(reg.Slug(), "target", err)
			}
			scanRoot := panel.PanelDirScanRoot(reg.Dir)
			facts.HeadSHA, err = exec.RevParseRef(scanRoot, "refs/heads/"+reg.Panel.Target)
			if err != nil {
				return refuse(reg.Slug(), "target", err)
			}
			facts.ArtifactDetails = panel.ArtifactPathDetails(res, scanRoot, exec.LsTree)
			reader := panel.DocumentSubjectTreeReader(gate, specDir, exec.LsTree, func(workdir, ref, name string) ([]byte, error) {
				return lifecycleExecutorFn(workdir).FileAtRef(ref, name)
			})
			facts.SubjectDigest, facts.SubjectErr = reader(scanRoot, facts.HeadSHA, panel.Subject(gate, specDir))
			if worktree != "" {
				facts.WorktreeAbsent = false
				facts.WorktreePath = worktree
				facts.UserDirt, err = lifecycleSubjectDirt(lifecycleExecutorFn(worktree), worktree, facts.HeadSHA, specDir, gate)
				if err != nil {
					return refuse(reg.Slug(), "dirty subject", err)
				}
			}
		}
		d := panel.PanelGateDecision(facts)
		if d.Action == panel.Block {
			if reg.Panel.Gate != gate {
				return refuse(reg.Slug(), "integrity", fmt.Errorf("%s", d.Message))
			}
			o.Decision = d
			return o, fmt.Errorf("%s", d.Message)
		}
		if d.Action == panel.Warn {
			if !reg.Panel.Abandoned {
				return refuse(reg.Slug(), "decision", fmt.Errorf("%s", d.Message))
			}
			o.Abandoned = true
			if o.AbandonReason != "" {
				o.AbandonReason += "; "
			}
			o.AbandonReason += reg.Panel.AbandonReason
			continue
		}
		if err := panel.CheckCore3(reg); err != nil {
			return refuse(reg.Slug(), "coverage", err)
		}
		if o.LivePanelSlug == "" {
			o.LivePanelSlug = reg.Slug()
		}
	}
	if o.Abandoned {
		warn(fmt.Sprintf("gate %s panel <slug>: abandoned: %s", termsafe.Escape(gate), termsafe.Escape(o.AbandonReason)))
	}
	return o, nil
}

// lifecycleRegistrations also attributes a damaged snapshot by its latest
// decodable round. An unattributable broken registration fails closed.
func lifecycleRegistrations(regs []panel.Registration, specID, gate string) []panel.Registration {
	var result []panel.Registration
	for _, reg := range regs {
		matches := reg.Panel.Spec == specID && reg.Panel.Gate == gate
		if !matches {
			lines, _ := panel.ReadLedger(reg.Dir)
			for _, line := range lines {
				var r panel.RoundRecord
				if json.Unmarshal(line, &r) == nil && r.Record == panel.RecordRound {
					matches = r.Spec == specID && r.Gate == gate
				}
			}
		}
		if matches || (reg.Err != nil && reg.Panel.Spec == "") {
			result = append(result, reg)
		}
	}
	return result
}

func lifecycleSubjectDirt(exec executor.Executor, worktree, head, specDir, gate string) ([]string, error) {
	status, err := exec.Status(worktree)
	if err != nil {
		return nil, err
	}
	roots := panel.Subject(gate, specDir)
	relSpec := strings.TrimSuffix(panel.Subject("spec_approve", specDir).Roots[0], "/")
	var dirt []string
	var paths []string
	listing, err := exec.LsTree(worktree, head)
	if err != nil {
		return nil, err
	}
	modes := map[string]string{}
	for _, line := range strings.Split(listing, "\n") {
		metadata, name, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if strings.HasPrefix(name, "\"") {
			name, err = strconv.Unquote(name)
			if err != nil {
				return nil, err
			}
		}
		fields := strings.Fields(metadata)
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed subject tree listing")
		}
		modes[name] = fields[0]
	}
	for _, line := range strings.Split(status, "\n") {
		name, err := panel.PorcelainPath(line)
		if err != nil {
			return nil, err
		}
		if name == "" {
			continue
		}
		if strings.HasSuffix(name, "/") {
			err := filepath.WalkDir(filepath.Join(worktree, filepath.FromSlash(name)), func(full string, e os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if e.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(worktree, full)
				if err == nil {
					paths = append(paths, filepath.ToSlash(rel))
				}
				return err
			})
			if err != nil {
				return nil, err
			}
		} else {
			paths = append(paths, name)
		}
	}
	for _, name := range paths {
		if !panel.SubjectIncludes(name, roots) {
			continue
		}
		before, present, err := exec.FileAtRefOrAbsent(head, name)
		if err != nil {
			return nil, err
		}
		full := filepath.Join(worktree, filepath.FromSlash(name))
		info, err := os.Lstat(full)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		afterPresent := err == nil
		var after []byte
		afterMode := ""
		if afterPresent {
			mode := "100644"
			if info.Mode().Perm()&0111 != 0 {
				mode = "100755"
			}
			afterMode = mode
			if !info.Mode().IsRegular() || (present && modes[name] != mode) || (!present && mode != "100644") {
				dirt = append(dirt, name)
				continue
			}
			file, err := safefile.Open(full)
			if err != nil {
				return nil, err
			}
			after, err = io.ReadAll(file)
			closeErr := file.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
		b, bOmit, err := panel.DocumentSubjectContent(name, before, relSpec, gate)
		if err != nil {
			return nil, err
		}
		a, aOmit, err := panel.DocumentSubjectContent(name, after, relSpec, gate)
		if err != nil {
			return nil, err
		}
		if bOmit && modes[name] != "100644" {
			b, bOmit = before, false
		}
		if aOmit && afterMode != "100644" {
			a, aOmit = after, false
		}
		if !present {
			bOmit = true
		}
		if !afterPresent {
			aOmit = true
		}
		if bOmit != aOmit || (!bOmit && !bytes.Equal(a, b)) {
			dirt = append(dirt, name)
		}
	}
	return dirt, nil
}

// recordLifecycleGateAudit is the shared writer called after epic resolution.
// A nil deferral list leaves the previous deferral metadata untouched.
func recordLifecycleGateAudit(epicID string, outcome Outcome, deferralIDs []string) error {
	meta := map[string]interface{}{}
	now := time.Now().UTC().Format(time.RFC3339)
	if outcome.Skipped {
		meta["panel_gate_skipped"] = true
		meta["panel_gate_skipped_at"] = now
	}
	if outcome.Abandoned {
		meta["panel_abandoned"] = true
		meta["panel_abandoned_at"] = now
		meta["panel_abandoned_reason"] = outcome.AbandonReason
	}
	if outcome.Absent {
		meta["panel_absent"] = true
		meta["panel_absent_gate"] = outcome.Gate
		meta["panel_absent_at"] = now
	}
	if deferralIDs != nil {
		meta["plan_deferral_ids"] = deferralIDs
	}
	if len(meta) == 0 {
		return nil
	}
	if epicID == "" {
		return fmt.Errorf("cannot record panel gate audit without an epic: %v", meta)
	}
	return lifecycleMergeMetadataFn(epicID, meta)
}
