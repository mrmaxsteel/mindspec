package bead

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkChunk represents a single work chunk from plan frontmatter.
type WorkChunk struct {
	ID        int      `yaml:"id"`
	Title     string   `yaml:"title"`
	Scope     string   `yaml:"scope"`
	Verify    []string `yaml:"verify"`
	DependsOn []int    `yaml:"depends_on"`
}

// Generated holds machine-written metadata in plan frontmatter.
type Generated struct {
	MolParentID string            `yaml:"mol_parent_id,omitempty"`
	BeadIDs     map[string]string `yaml:"bead_ids,omitempty"`
}

// PlanMeta represents the YAML frontmatter of a plan.md file.
type PlanMeta struct {
	Status     string      `yaml:"status"`
	SpecID     string      `yaml:"spec_id"`
	WorkChunks []WorkChunk `yaml:"work_chunks"`
	Generated  *Generated  `yaml:"generated,omitempty"`
}

// ParsePlanMeta extracts and parses YAML frontmatter from a plan file.
func ParsePlanMeta(planPath string) (*PlanMeta, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read plan: %v", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("no frontmatter found (expected leading ---)")
	}

	var fmLines []string
	found := false
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			found = true
			break
		}
		fmLines = append(fmLines, line)
	}

	if !found {
		return nil, fmt.Errorf("unclosed frontmatter (missing closing ---)")
	}

	// Filter out comment lines
	var activeFmLines []string
	for _, line := range fmLines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			activeFmLines = append(activeFmLines, line)
		}
	}

	fmContent := strings.Join(activeFmLines, "\n")

	var meta PlanMeta
	if err := yaml.Unmarshal([]byte(fmContent), &meta); err != nil {
		return nil, fmt.Errorf("parsing plan YAML: %w", err)
	}

	return &meta, nil
}

// PlanBeadResult holds the results of molecule-based plan decomposition.
type PlanBeadResult struct {
	MolParentID string         // molecule parent (epic) bead ID
	PlanGateID  string         // plan approval gate bead ID (empty if not created)
	ChunkBeads  map[int]string // chunk ID -> child bead ID
}

// CreatePlanBeads creates a molecule from an approved plan's work_chunks.
// The molecule parent is an epic with the spec bead as its parent.
// Work chunks become task children under the molecule parent.
// Dependencies are wired between children via DepAdd.
// Also creates a plan approval gate and wires impl beads to depend on it.
// Idempotent: searches for existing beads before creating.
// Requires spec gate to be resolved (if one exists).
func CreatePlanBeads(root, specID string) (*PlanBeadResult, error) {
	planPath := fmt.Sprintf("%s/docs/specs/%s/plan.md", root, specID)
	meta, err := ParsePlanMeta(planPath)
	if err != nil {
		return nil, err
	}

	// Validate approved
	if !strings.EqualFold(meta.Status, "approved") {
		return nil, fmt.Errorf("plan is not approved (status: %q)", meta.Status)
	}

	// Validate work_chunks present
	if len(meta.WorkChunks) == 0 {
		return nil, fmt.Errorf("plan has no work_chunks defined")
	}

	// Check spec gate is resolved (if one exists)
	specGateTitle := SpecGateTitle(specID)
	specGateResolved, _ := IsGateResolved(specGateTitle)
	if !specGateResolved {
		return nil, fmt.Errorf("spec gate is not resolved — run `mindspec approve spec %s` first", specID)
	}

	// Find spec gate ID for dependency wiring
	var specGateID string
	specGate, _ := FindGateAnyStatus(specGateTitle)
	if specGate != nil {
		specGateID = specGate.ID
	}

	// Find spec bead as grandparent
	specPrefix := fmt.Sprintf("[SPEC %s]", specID)
	var specBeadID string
	specBeads, err := Search(specPrefix)
	if err == nil && len(specBeads) > 0 {
		specBeadID = specBeads[0].ID
	}

	// Find or create molecule parent (epic)
	molPrefix := fmt.Sprintf("[PLAN %s]", specID)
	var molParentID string
	existing, err := Search(molPrefix)
	if err == nil && len(existing) > 0 {
		molParentID = existing[0].ID
	} else {
		molTitle := fmt.Sprintf("%s Plan decomposition", molPrefix)
		molDesc := fmt.Sprintf("Molecule parent for spec %s\nPlan: docs/specs/%s/plan.md", specID, specID)
		molBead, err := Create(molTitle, molDesc, "epic", 2, specBeadID)
		if err != nil {
			return nil, fmt.Errorf("creating molecule parent: %w", err)
		}
		molParentID = molBead.ID
	}

	// Create or find plan approval gate
	planGateTitle := PlanGateTitle(specID)
	var planGateID string
	planGate, err := FindOrCreateGate(planGateTitle, molParentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create plan gate: %v\n", err)
	} else {
		planGateID = planGate.ID
		// Wire plan gate to depend on spec gate (if spec gate exists)
		if specGateID != "" {
			_ = DepAdd(planGateID, specGateID) // best-effort
		}
	}

	// Create beads per chunk as molecule children (idempotent)
	mapping := make(map[int]string)
	for _, chunk := range meta.WorkChunks {
		implPrefix := fmt.Sprintf("[IMPL %s.%d]", specID, chunk.ID)

		// Check for existing
		existing, err := Search(implPrefix)
		if err == nil && len(existing) > 0 {
			mapping[chunk.ID] = existing[0].ID
			continue
		}

		// Build description (capped at 800 chars)
		desc := buildImplDescription(chunk, specID)

		title := fmt.Sprintf("%s %s", implPrefix, chunk.Title)
		bead, err := Create(title, desc, "task", 2, molParentID)
		if err != nil {
			return nil, fmt.Errorf("creating bead for chunk %d: %w", chunk.ID, err)
		}
		mapping[chunk.ID] = bead.ID

		// Wire impl bead to depend on plan gate
		if planGateID != "" {
			_ = DepAdd(bead.ID, planGateID) // best-effort
		}
	}

	// Wire dependencies between children
	for _, chunk := range meta.WorkChunks {
		for _, depID := range chunk.DependsOn {
			blockedBead, ok := mapping[chunk.ID]
			if !ok {
				continue
			}
			blockerBead, ok := mapping[depID]
			if !ok {
				continue
			}
			if err := DepAdd(blockedBead, blockerBead); err != nil {
				return nil, fmt.Errorf("wiring dep %d->%d: %w", chunk.ID, depID, err)
			}
		}
	}

	return &PlanBeadResult{
		MolParentID: molParentID,
		PlanGateID:  planGateID,
		ChunkBeads:  mapping,
	}, nil
}

// buildImplDescription creates a structured description for an impl bead.
func buildImplDescription(chunk WorkChunk, specID string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Scope: %s", chunk.Scope)
	if len(chunk.Verify) > 0 {
		sb.WriteString("\nVerify:")
		for _, v := range chunk.Verify {
			fmt.Fprintf(&sb, "\n- %s", v)
		}
	}
	fmt.Fprintf(&sb, "\nPlan: docs/specs/%s/plan.md", specID)

	desc := sb.String()
	if len(desc) > 800 {
		desc = desc[:797] + "..."
	}
	return desc
}

// WriteGeneratedBeadIDs writes bead IDs into the plan frontmatter under generated.
// Includes mol_parent_id and per-chunk bead_ids.
// Preserves existing frontmatter fields via map[string]interface{} round-trip.
func WriteGeneratedBeadIDs(planPath string, result *PlanBeadResult) error {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("cannot read plan: %v", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find frontmatter boundaries
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fmt.Errorf("no frontmatter found")
	}

	fmEndIdx := -1
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			fmEndIdx = i + 1
			break
		}
	}
	if fmEndIdx == -1 {
		return fmt.Errorf("unclosed frontmatter")
	}

	// Extract frontmatter lines (including comments, which we'll filter for parsing)
	fmLines := lines[1:fmEndIdx]
	var activeFmLines []string
	for _, line := range fmLines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			activeFmLines = append(activeFmLines, line)
		}
	}

	// Parse into generic map to preserve all fields
	fmContent := strings.Join(activeFmLines, "\n")
	var fmMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmContent), &fmMap); err != nil {
		return fmt.Errorf("parsing frontmatter: %w", err)
	}
	if fmMap == nil {
		fmMap = make(map[string]interface{})
	}

	// Build bead_ids as map with string keys for YAML
	beadIDs := make(map[string]string)
	for chunkID, beadID := range result.ChunkBeads {
		beadIDs[fmt.Sprintf("%d", chunkID)] = beadID
	}

	// Set generated block: mol_parent_id + bead_ids
	gen, ok := fmMap["generated"].(map[string]interface{})
	if !ok {
		gen = make(map[string]interface{})
	}
	if result.MolParentID != "" {
		gen["mol_parent_id"] = result.MolParentID
	}
	gen["bead_ids"] = beadIDs
	fmMap["generated"] = gen

	// Re-marshal frontmatter
	newFm, err := yaml.Marshal(fmMap)
	if err != nil {
		return fmt.Errorf("marshaling frontmatter: %w", err)
	}

	// Splice back: new frontmatter + body after closing ---
	body := strings.Join(lines[fmEndIdx+1:], "\n")
	output := "---\n" + string(newFm) + "---\n" + body

	if err := os.WriteFile(planPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("writing plan: %w", err)
	}

	return nil
}
