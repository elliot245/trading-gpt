package config

type AgentConfig struct {
	Trading TradingAgentConfig `json:"trading"`
	Keeper  KeeperAgentConfig  `json:"keeper"`

	// Scripted selects the deterministic replay agent for offline backtests.
	// When enabled it takes precedence over the LLM-backed agents above.
	Scripted *ScriptedAgentConfig `json:"scripted"`
}
