package config

// ScriptedAgentConfig configures the scripted (replay) agent used by the
// offline backtest harness. Instead of calling an LLM, the scripted agent
// replays deterministic decisions from a fixture file, which makes backtest
// runs reproducible and lets fixed-vs-unfixed behaviour be compared without
// LLM cost or nondeterminism.
//
// When enabled, the scripted agent takes precedence over the trading/keeper
// (LLM) agents. Point fixture_file at a JSON file, see
// pkg/agents/scripted for the format and fixtures/backtest for an example.
type ScriptedAgentConfig struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`

	// FixtureFile is the path (relative to the working directory) of the
	// decision fixture JSON. When empty, the agent always answers with the
	// default action (no_action).
	FixtureFile string `json:"fixture_file"`
}
