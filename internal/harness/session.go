package harness

import (
	"context"
	"fmt"
	"time"
)

// RunSession executes an agent session for a given scenario in a sandbox.
// It runs the scenario's setup, invokes the agent, collects recorded events,
// and returns a SessionResult with timing and event data.
func RunSession(ctx context.Context, agent Agent, scenario Scenario, sandbox *Sandbox) (*SessionResult, error) {
	// Run scenario setup to prepare sandbox state
	if scenario.Setup != nil {
		if err := scenario.Setup(sandbox); err != nil {
			return nil, fmt.Errorf("scenario setup: %w", err)
		}
	}

	// Build run options from scenario
	opts := RunOpts{
		MaxTurns: scenario.MaxTurns,
		Model:    scenario.Model,
	}

	start := time.Now()

	result, err := agent.Run(ctx, sandbox, scenario.Prompt, opts)
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}

	result.Duration = time.Since(start)

	return result, nil
}
