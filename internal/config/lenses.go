package config

import "gopkg.in/yaml.v3"

// Acceptance is the gate question and its advisory exclusions.
type Acceptance struct {
	Question    string   `yaml:"question" json:"question"`
	NotBlocking []string `yaml:"not_blocking" json:"not_blocking"`
}

// The default tables are data. Slot expansion reads the selected gate's list.
const lensDefaults = `
lenses:
  author-of-record: Does the change do what its plan says, and nothing else?
  empirical-prober: Run the tests and validators. Do the claims hold?
  codebase-pin: Do the named files, symbols and tests exist and pass?
  adversarial: Which input, order or failure makes this change wrong?
  contract-stability: Do types, schemas, outputs and messages stay compatible?
  integration: Can the next unit of work use this change without rework?
  falsifiability: Is every acceptance criterion concrete and testable?
  scope-honesty: Do the scope and impacted domains match what the document asks for?
  contradiction: Do two requirements, or a requirement and a non-goal, conflict?
  feasibility: Is the approach sound and in proportion to its risk?
gates:
  spec_approve:
    lenses: &document [falsifiability, scope-honesty, contradiction, feasibility]
    acceptance:
      question: Can a planner write a plan from this spec without inventing a requirement?
      not_blocking: [wording style, a scope item that the spec names as another spec's]
  plan_approve:
    lenses: *document
    acceptance:
      question: Does every bead trace to a requirement, and can each bead be built and tested alone?
      not_blocking: [estimates, names]
  bead:
    lenses: &implementation [author-of-record, empirical-prober, codebase-pin, adversarial, contract-stability, integration]
    acceptance:
      question: Does the change meet the bead's acceptance criteria with passing tests, and break nothing else?
      not_blocking: [style the linter accepts, defects outside the diff (file a follow-up)]
  final_review:
    lenses: *implementation
    acceptance:
      question: Does the branch meet every acceptance criterion, and does the project's CI command pass?
      not_blocking: [style already reviewed per bead]
  adhoc:
    lenses: *implementation
    acceptance:
      question: Is the change correct, and does it do what its description says?
      not_blocking: [style]
`

func defaultLensData() Panel {
	var data Panel
	if err := yaml.Unmarshal([]byte(lensDefaults), &data); err != nil {
		panic(err)
	}
	return data
}

// GateLenses returns the ordered lens list for gate.
func (c *Config) GateLenses(gate string) []string {
	if gp, ok := c.Panel.Gates[gate]; ok && len(gp.Lenses) > 0 {
		return gp.Lenses
	}
	return defaultLensData().Gates[gate].Lenses
}

// GateAcceptance returns the gate question and exclusions.
func (c *Config) GateAcceptance(gate string) Acceptance {
	if gp, ok := c.Panel.Gates[gate]; ok && gp.Acceptance != nil {
		return *gp.Acceptance
	}
	if gp, ok := defaultLensData().Gates[gate]; ok {
		return *gp.Acceptance
	}
	return Acceptance{}
}
