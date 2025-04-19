package memory

import (
	"context"
	"time"
)

// Memory 表示一条交易经验记忆
type Memory struct {
	ID         string    // 唯一标识符
	Title      string    // 记忆标题
	Content    string    // 记忆内容（经验总结）
	Tags       []string  // 标签，用于分类和检索
	CreatedAt  time.Time // 创建时间
	Source     string    // 记忆来源（如"交易反思"、"市场分析"等）
	Importance int       // 重要性评分（1-10）
}

// IMemoryManager 管理交易记忆
type IMemoryManager interface {
	// 初始化管理器，加载现有记忆
	Initialize(ctx context.Context) error

	// 添加新记忆
	AddMemory(ctx context.Context, title, content string, tags []string, source string, importance int) (*Memory, error)

	// 获取所有记忆
	GetAllMemories(ctx context.Context) ([]Memory, error)

	// 基本检索功能
	RetrieveMemories(ctx context.Context, query string, tags []string, limit int) ([]Memory, error)

	// 保存所有记忆到文件
	SaveMemories(ctx context.Context) error
}

// IMemoryRetriever 提供高级记忆检索功能
type IMemoryRetriever interface {
	// 使用LLM检索与当前情境最相关的记忆
	RetrieveRelevantMemories(ctx context.Context, situation string, maxResults int) ([]Memory, error)
}

// IMemoryTrigger 记忆触发器接口
type IMemoryTrigger interface {
	// 判断是否应该触发记忆创建
	ShouldTrigger(ctx context.Context, data interface{}) bool

	// 获取触发器类型
	GetTriggerType() string

	// 获取反思提示
	GetReflectionPrompt(ctx context.Context, data interface{}) (string, error)
}
