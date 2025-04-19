package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/tmc/langchaingo/llms"
)

// MemoryRetriever 实现基于LLM的记忆检索
type MemoryRetriever struct {
	manager *MemoryManager
	llm     llms.Model
}

// NewMemoryRetriever 创建一个新的记忆检索器
func NewMemoryRetriever(manager *MemoryManager, llm llms.Model) *MemoryRetriever {
	return &MemoryRetriever{
		manager: manager,
		llm:     llm,
	}
}

// RetrieveRelevantMemories 使用LLM检索与当前情境最相关的记忆
func (r *MemoryRetriever) RetrieveRelevantMemories(ctx context.Context, situation string, maxResults int) ([]Memory, error) {
	// 获取所有记忆
	memories, err := r.manager.GetAllMemories(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all memories")
	}

	if len(memories) == 0 {
		return []Memory{}, nil
	}

	// 评估记忆相关性
	relevanceScores, err := r.evaluateRelevance(ctx, situation, memories)
	if err != nil {
		return nil, errors.Wrap(err, "failed to evaluate memory relevance")
	}

	// 创建带有相关性分数的记忆列表
	type ScoredMemory struct {
		Memory Memory
		Score  int
	}

	scoredMemories := make([]ScoredMemory, len(memories))
	for i, memory := range memories {
		scoredMemories[i] = ScoredMemory{
			Memory: memory,
			Score:  relevanceScores[i],
		}
	}

	// 按相关性分数排序
	sort.Slice(scoredMemories, func(i, j int) bool {
		return scoredMemories[i].Score > scoredMemories[j].Score
	})

	// 限制结果数量
	resultCount := len(scoredMemories)
	if maxResults > 0 && resultCount > maxResults {
		resultCount = maxResults
	}

	// 提取排序后的记忆
	results := make([]Memory, resultCount)
	for i := 0; i < resultCount; i++ {
		results[i] = scoredMemories[i].Memory
	}

	return results, nil
}

// evaluateRelevance 评估记忆与当前情境的相关性
func (r *MemoryRetriever) evaluateRelevance(ctx context.Context, situation string, memories []Memory) ([]int, error) {
	// 构建提示
	prompt := fmt.Sprintf(`你是一个交易记忆检索系统。你的任务是评估以下交易记忆与当前情境的相关性。
当前情境：%s

请为每条记忆评分，范围从0到10，其中0表示完全不相关，10表示非常相关。
只返回评分数字，每行一个数字，不要有任何其他文字。

记忆列表：
`, situation)

	for i, memory := range memories {
		prompt += fmt.Sprintf(`
记忆 %d:
标题: %s
标签: %s
内容: %s

`, i+1, memory.Title, strings.Join(memory.Tags, ", "), memory.Content)
	}

	// 调用LLM评估相关性
	callOpts := []llms.CallOption{
		llms.WithTemperature(0.0), // 使用低温度以获得确定性结果
	}

	// 创建LLM消息
	messages := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{
				llms.TextContent{
					Text: "你是一个交易记忆检索系统，负责评估记忆与当前情境的相关性。",
				},
			},
		},
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextContent{
					Text: prompt,
				},
			},
		},
	}

	resp, err := r.llm.GenerateContent(ctx, messages, callOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call LLM for relevance evaluation")
	}

	if resp == nil || len(resp.Choices) == 0 || resp.Choices[0].Content == "" {
		return nil, errors.New("empty response from LLM")
	}

	// 解析LLM响应
	responseText := resp.Choices[0].Content
	lines := strings.Split(responseText, "\n")

	scores := make([]int, len(memories))
	for i := 0; i < len(memories) && i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var score int
		_, err := fmt.Sscanf(line, "%d", &score)
		if err != nil {
			log.Warnf("Failed to parse score from line: %s, error: %v", line, err)
			score = 0
		}

		// 确保分数在0-10范围内
		if score < 0 {
			score = 0
		} else if score > 10 {
			score = 10
		}

		scores[i] = score
	}

	// 如果解析的分数数量不足，用0填充
	for i := len(lines); i < len(memories); i++ {
		scores[i] = 0
	}

	return scores, nil
}
