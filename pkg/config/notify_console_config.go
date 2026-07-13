package config

// NotifyConsoleConfig configures the console notify channel used by the
// offline backtest harness. The console channel plays the role that the
// Feishu channels play in live mode: it owns the admin chat session that
// drives the strategy's agent decision loop, and it prints every agent
// reply to stdout (and optionally appends it to a log file) instead of
// calling any external service.
type NotifyConsoleConfig struct {
	Enabled bool `json:"enabled"`

	// LogFile, when non-empty, appends every replied message (with a wall
	// clock timestamp) to the given file so the decision stream of a backtest
	// run can be diffed between runs / variants.
	LogFile string `json:"log_file"`
}
