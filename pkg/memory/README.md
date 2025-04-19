# 记忆模块 (Memory Module)

记忆模块是Trading-GPT系统的核心组件之一，负责存储、管理和检索交易经验和反思。该模块专注于保存有价值的交易经验总结，而非详细的聊天历史，使交易代理能够从历史经验中学习，提高决策质量。

## 功能特点

- 使用单个Markdown文件存储所有记忆，便于人工查看和编辑
- 支持基于关键词和标签的基础检索
- 支持基于LLM的相关性检索，找出与当前情境最相关的记忆
- 提供多种记忆触发机制，包括交易完成触发、定期触发和手动触发
- 与交易代理系统无缝集成

## 使用方法

### 基本配置

```go
import (
    "github.com/yubing744/trading-gpt/pkg/memory"
)

// 创建记忆配置
memoryConfig := &memory.Config{
    Enabled:    true,                // 是否启用记忆功能
    FilePath:   "data/memories.md",  // 记忆文件路径
    MaxResults: 5,                   // 检索返回的最大记忆数量
    Periodic: memory.PeriodicConfig{
        Enabled:  true,              // 是否启用定期反思
        Interval: 24 * time.Hour,    // 反思间隔
    },
}
```

### 创建记忆管理器

```go
// 创建记忆管理器
manager := memory.NewMemoryManager(memoryConfig.FilePath)

// 初始化管理器
ctx := context.Background()
if err := manager.Initialize(ctx); err != nil {
    // 处理错误
}
```

### 添加记忆

```go
// 添加新记忆
memory, err := manager.AddMemory(
    ctx,
    "市场急跌时的应对策略",                                // 标题
    "在市场出现急跌时，应立即评估持仓风险，而不是盲目加仓...",  // 内容
    []string{"风险管理", "市场波动", "止损"},             // 标签
    "trade_reflection",                              // 来源
    8,                                               // 重要性(1-10)
)
```

### 检索记忆

```go
// 基本检索 - 基于关键词和标签
memories, err := manager.RetrieveMemories(
    ctx,
    "风险",                      // 查询关键词
    []string{"市场波动"},         // 标签
    5,                          // 最大结果数
)

// 创建记忆检索器
retriever := memory.NewMemoryRetriever(manager, llm)

// 基于LLM的相关性检索
situation := "市场正在快速下跌，我需要决定是否应该立即平仓或者设置更紧的止损。"
relevantMemories, err := retriever.RetrieveRelevantMemories(
    ctx,
    situation,  // 当前情境
    3,          // 最大结果数
)
```

### 使用触发器

```go
// 创建交易完成触发器
tradeTrigger := memory.NewTradeCompleteTrigger()

// 检查是否应该触发
if tradeTrigger.ShouldTrigger(ctx, tradeResult) {
    // 获取反思提示
    prompt, err := tradeTrigger.GetReflectionPrompt(ctx, tradeResult)
    
    // 使用提示生成反思...
}

// 创建定期触发器
periodicTrigger := memory.NewPeriodicTrigger(24 * time.Hour)

// 创建手动触发器
manualTrigger := memory.NewManualTrigger("请总结你对比特币市场的看法。")
```

### 与交易代理集成

请参考 `examples/trading_agent_integration.go` 文件，了解如何将记忆模块集成到交易代理中。

## 记忆文件格式

记忆以Markdown格式存储在单个文件中，格式如下：

```markdown
# Trading-GPT 记忆

## 记忆 1
- **ID**: 1234-5678-90ab-cdef
- **标题**: 市场急跌时的应对策略
- **标签**: 风险管理, 市场波动, 止损
- **创建时间**: 2023-06-15 14:30:00
- **来源**: trade_reflection
- **重要性**: 8

在市场出现急跌时，应立即评估持仓风险，而不是盲目加仓。

本次交易中，市场突然下跌5%，我选择了立即设置更紧的止损，而不是试图抄底，这避免了更大的损失。

经验表明，市场恐慌时通常会超调，等待企稳信号再入场通常是更好的策略。

## 记忆 2
...
```

## 运行测试

运行单元测试：

```bash
go test -v ./pkg/memory
```

运行集成测试：

```bash
RUN_INTEGRATION_TESTS=1 go test -v ./pkg/memory
```
