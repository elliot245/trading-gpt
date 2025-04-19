package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

// 定义触发器类型常量
const (
	TriggerTypeTradeComplete = "trade_complete"
	TriggerTypePeriodic      = "periodic"
	TriggerTypeManual        = "manual"
)

// TradeCompleteTrigger 交易完成触发器
type TradeCompleteTrigger struct{}

// NewTradeCompleteTrigger 创建一个新的交易完成触发器
func NewTradeCompleteTrigger() *TradeCompleteTrigger {
	return &TradeCompleteTrigger{}
}

// ShouldTrigger 判断是否应该触发记忆创建
func (t *TradeCompleteTrigger) ShouldTrigger(ctx context.Context, data interface{}) bool {
	// 交易完成时总是触发
	return true
}

// GetTriggerType 获取触发器类型
func (t *TradeCompleteTrigger) GetTriggerType() string {
	return TriggerTypeTradeComplete
}

// GetReflectionPrompt 获取反思提示
func (t *TradeCompleteTrigger) GetReflectionPrompt(ctx context.Context, data interface{}) (string, error) {
	// 构建反思提示
	prompt := `请对刚刚完成的交易进行反思，总结经验和教训。
请考虑以下方面：
1. 交易决策是否正确？为什么？
2. 交易执行是否顺利？有什么可以改进的地方？
3. 市场条件如何影响了这次交易？
4. 从这次交易中学到了什么重要经验？
5. 下次遇到类似情况应该如何应对？

请以以下格式回答：

标题：[简短的经验总结标题]

内容：
[详细的经验和反思内容，2-3段]

标签：[3-5个相关标签，用逗号分隔]

重要性：[1-10的数字，表示这个经验的重要程度]`

	return prompt, nil
}

// PeriodicTrigger 定期触发器
type PeriodicTrigger struct {
	interval time.Duration
	lastTime time.Time
}

// NewPeriodicTrigger 创建一个新的定期触发器
func NewPeriodicTrigger(interval time.Duration) *PeriodicTrigger {
	return &PeriodicTrigger{
		interval: interval,
		lastTime: time.Now(),
	}
}

// ShouldTrigger 判断是否应该触发记忆创建
func (t *PeriodicTrigger) ShouldTrigger(ctx context.Context, data interface{}) bool {
	now := time.Now()
	if now.Sub(t.lastTime) >= t.interval {
		t.lastTime = now
		return true
	}
	return false
}

// GetTriggerType 获取触发器类型
func (t *PeriodicTrigger) GetTriggerType() string {
	return TriggerTypePeriodic
}

// GetReflectionPrompt 获取反思提示
func (t *PeriodicTrigger) GetReflectionPrompt(ctx context.Context, data interface{}) (string, error) {
	// 构建定期反思提示
	prompt := fmt.Sprintf(`请对过去%s的交易活动进行总结和反思。
请考虑以下方面：
1. 过去这段时间的市场趋势是什么？
2. 你的交易策略表现如何？
3. 有哪些成功的交易？为什么成功？
4. 有哪些失败的交易？为什么失败？
5. 你从这段时间的交易中学到了什么？
6. 你的交易策略需要做哪些调整？

请以以下格式回答：

标题：[简短的阶段性总结标题]

内容：
[详细的总结和反思内容，3-5段]

标签：[3-5个相关标签，用逗号分隔]

重要性：[1-10的数字，表示这个总结的重要程度]`, formatDuration(t.interval))

	return prompt, nil
}

// formatDuration 格式化时间间隔为易读的字符串
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("%d天%d小时", days, hours)
		}
		return fmt.Sprintf("%d天", days)
	} else if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%d小时%d分钟", hours, minutes)
		}
		return fmt.Sprintf("%d小时", hours)
	} else {
		return fmt.Sprintf("%d分钟", minutes)
	}
}

// ManualTrigger 手动触发器
type ManualTrigger struct {
	prompt string
}

// NewManualTrigger 创建一个新的手动触发器
func NewManualTrigger(prompt string) *ManualTrigger {
	return &ManualTrigger{
		prompt: prompt,
	}
}

// ShouldTrigger 判断是否应该触发记忆创建
func (t *ManualTrigger) ShouldTrigger(ctx context.Context, data interface{}) bool {
	// 手动触发总是返回true
	return true
}

// GetTriggerType 获取触发器类型
func (t *ManualTrigger) GetTriggerType() string {
	return TriggerTypeManual
}

// GetReflectionPrompt 获取反思提示
func (t *ManualTrigger) GetReflectionPrompt(ctx context.Context, data interface{}) (string, error) {
	if t.prompt == "" {
		return "", errors.New("manual trigger prompt is empty")
	}
	return t.prompt, nil
}
