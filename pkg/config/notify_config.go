package config

type NotifyConfig struct {
	Feishu     *NotifyFeishuConfig     `json:"feishu"`
	FeishuHook *NotifyFeishuHookConfig `json:"feishu_hook"`

	// Console is a stdout/file notify channel for offline backtest runs; it
	// hosts the admin session that drives the agent decision loop.
	Console *NotifyConsoleConfig `json:"console"`
}
