package config

import "time"

// MemoryConfig 记忆模块配置
type MemoryConfig struct {
	Enabled     bool          `json:"enabled" yaml:"enabled"`         // 是否启用记忆功能
	FilePath    string        `json:"file_path" yaml:"file_path"`     // 记忆文件路径
	MaxResults  int           `json:"max_results" yaml:"max_results"` // 检索返回的最大记忆数量
	Periodic    PeriodicConfig `json:"periodic" yaml:"periodic"`       // 定期反思配置
}

// PeriodicConfig 定期反思配置
type PeriodicConfig struct {
	Enabled  bool          `json:"enabled" yaml:"enabled"`   // 是否启用定期反思
	Interval time.Duration `json:"interval" yaml:"interval"` // 反思间隔
}
