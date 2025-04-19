package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tmc/langchaingo/llms"
)

// 创建一个模拟的LLM实现
type MockLLM struct {
	mock.Mock
}

func (m *MockLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	args := m.Called(ctx, messages, options)
	return args.Get(0).(*llms.ContentResponse), args.Error(1)
}

func (m *MockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	args := m.Called(ctx, prompt, options)
	return args.Get(0).(string), args.Error(1)
}

func TestMemoryIntegration(t *testing.T) {
	// 跳过集成测试，除非明确指定运行
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=1 to run.")
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "memory-integration-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建测试文件路径
	testFilePath := filepath.Join(tempDir, "memories.md")

	// 创建记忆管理器
	manager := NewMemoryManager(testFilePath)

	// 初始化管理器
	ctx := context.Background()
	err = manager.Initialize(ctx)
	assert.NoError(t, err)

	// 添加测试记忆
	_, err = manager.AddMemory(ctx, "市场急跌时的应对策略", "在市场出现急跌时，应立即评估持仓风险，而不是盲目加仓。本次交易中，市场突然下跌5%，我选择了立即设置更紧的止损，而不是试图抄底，这避免了更大的损失。", []string{"风险管理", "市场波动", "止损"}, "trade_reflection", 8)
	assert.NoError(t, err)

	_, err = manager.AddMemory(ctx, "趋势确认的重要性", "在进行趋势交易时，等待趋势确认信号非常重要，不要仅凭价格突破就入场。本次交易中，我等待了移动平均线的交叉确认和成交量放大，才进行了顺势交易，最终获得了不错的收益。", []string{"趋势交易", "技术分析", "入场时机"}, "trade_reflection", 7)
	assert.NoError(t, err)

	// 创建模拟LLM
	mockLLM := new(MockLLM)

	// 设置模拟LLM的行为
	mockLLM.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "8\n5",
			},
		},
	}, nil)
	mockLLM.On("Call", mock.Anything, mock.Anything, mock.Anything).Return("", nil)

	// 创建记忆检索器
	retriever := NewMemoryRetriever(manager, mockLLM)

	// 测试记忆检索
	situation := "市场正在快速下跌，我需要决定是否应该立即平仓或者设置更紧的止损。"
	memories, err := retriever.RetrieveRelevantMemories(ctx, situation, 1)
	assert.NoError(t, err)
	assert.Len(t, memories, 1)
	assert.Equal(t, "市场急跌时的应对策略", memories[0].Title)

	// 测试交易完成触发器
	tradeTrigger := NewTradeCompleteTrigger()
	assert.True(t, tradeTrigger.ShouldTrigger(ctx, nil))
	assert.Equal(t, TriggerTypeTradeComplete, tradeTrigger.GetTriggerType())

	prompt, err := tradeTrigger.GetReflectionPrompt(ctx, nil)
	assert.NoError(t, err)
	assert.Contains(t, prompt, "请对刚刚完成的交易进行反思")

	// 测试定期触发器
	periodicTrigger := NewPeriodicTrigger(24 * time.Hour)
	assert.True(t, periodicTrigger.ShouldTrigger(ctx, nil))
	assert.Equal(t, TriggerTypePeriodic, periodicTrigger.GetTriggerType())

	// 第二次调用应该返回false，因为时间间隔未到
	assert.False(t, periodicTrigger.ShouldTrigger(ctx, nil))

	prompt, err = periodicTrigger.GetReflectionPrompt(ctx, nil)
	assert.NoError(t, err)
	assert.Contains(t, prompt, "请对过去1天的交易活动进行总结和反思")

	// 测试手动触发器
	manualPrompt := "请总结你对比特币市场的看法。"
	manualTrigger := NewManualTrigger(manualPrompt)
	assert.True(t, manualTrigger.ShouldTrigger(ctx, nil))
	assert.Equal(t, TriggerTypeManual, manualTrigger.GetTriggerType())

	prompt, err = manualTrigger.GetReflectionPrompt(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, manualPrompt, prompt)

	// 验证模拟对象的调用
	mockLLM.AssertExpectations(t)
}
