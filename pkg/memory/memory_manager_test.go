package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryManager(t *testing.T) {
	t.Parallel()
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "memory-test")
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

	// 测试添加记忆
	memory, err := manager.AddMemory(ctx, "测试记忆", "这是一条测试记忆内容", []string{"测试", "记忆"}, "test", 5)
	assert.NoError(t, err)
	assert.NotNil(t, memory)
	assert.Equal(t, "测试记忆", memory.Title)
	assert.Equal(t, "这是一条测试记忆内容", memory.Content)
	assert.Equal(t, []string{"测试", "记忆"}, memory.Tags)
	assert.Equal(t, "test", memory.Source)
	assert.Equal(t, 5, memory.Importance)

	// 测试获取所有记忆
	memories, err := manager.GetAllMemories(ctx)
	assert.NoError(t, err)
	assert.Len(t, memories, 1)
	assert.Equal(t, memory.ID, memories[0].ID)

	// 测试基本检索
	results, err := manager.RetrieveMemories(ctx, "测试", nil, 0)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, memory.ID, results[0].ID)

	// 测试标签检索
	results, err = manager.RetrieveMemories(ctx, "", []string{"记忆"}, 0)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, memory.ID, results[0].ID)

	// 测试不匹配的检索
	results, err = manager.RetrieveMemories(ctx, "不存在", nil, 0)
	assert.NoError(t, err)
	assert.Len(t, results, 0)

	// 添加第二条记忆
	memory2, err := manager.AddMemory(ctx, "另一条记忆", "这是另一条测试记忆内容", []string{"测试", "重要"}, "test", 8)
	assert.NoError(t, err)
	assert.NotNil(t, memory2)

	// 测试获取所有记忆
	memories, err = manager.GetAllMemories(ctx)
	assert.NoError(t, err)
	assert.Len(t, memories, 2)

	// 测试限制结果数量
	results, err = manager.RetrieveMemories(ctx, "测试", nil, 1)
	assert.NoError(t, err)
	assert.Len(t, results, 1)

	// 测试从文件加载记忆
	newManager := NewMemoryManager(testFilePath)
	err = newManager.Initialize(ctx)
	assert.NoError(t, err)

	loadedMemories, err := newManager.GetAllMemories(ctx)
	assert.NoError(t, err)
	assert.Len(t, loadedMemories, 2)
	assert.Equal(t, memory.ID, loadedMemories[0].ID)
	assert.Equal(t, memory2.ID, loadedMemories[1].ID)
}

func TestParseMemoriesFromMarkdown(t *testing.T) {
	markdown := `# Trading-GPT 记忆

## 记忆 1
- **ID**: test-id-1
- **标题**: 测试记忆1
- **标签**: 测试, 记忆
- **创建时间**: 2023-06-15 14:30:00
- **来源**: test
- **重要性**: 5

这是测试记忆1的内容。

## 记忆 2
- **ID**: test-id-2
- **标题**: 测试记忆2
- **标签**: 测试, 重要
- **创建时间**: 2023-06-16 15:40:00
- **来源**: test
- **重要性**: 8

这是测试记忆2的内容。
这是第二行内容。
`

	memories, err := parseMemoriesFromMarkdown(markdown)
	assert.NoError(t, err)
	assert.Len(t, memories, 2)

	assert.Equal(t, "test-id-1", memories[0].ID)
	assert.Equal(t, "测试记忆1", memories[0].Title)
	assert.Equal(t, []string{"测试", "记忆"}, memories[0].Tags)
	assert.Equal(t, "test", memories[0].Source)
	assert.Equal(t, 5, memories[0].Importance)
	assert.Equal(t, "这是测试记忆1的内容。", memories[0].Content)

	assert.Equal(t, "test-id-2", memories[1].ID)
	assert.Equal(t, "测试记忆2", memories[1].Title)
	assert.Equal(t, []string{"测试", "重要"}, memories[1].Tags)
	assert.Equal(t, "test", memories[1].Source)
	assert.Equal(t, 8, memories[1].Importance)
	assert.Equal(t, "这是测试记忆2的内容。\n这是第二行内容。", memories[1].Content)

	// 测试时间解析
	expectedTime1, _ := time.Parse("2006-01-02 15:04:05", "2023-06-15 14:30:00")
	assert.Equal(t, expectedTime1, memories[0].CreatedAt)

	expectedTime2, _ := time.Parse("2006-01-02 15:04:05", "2023-06-16 15:40:00")
	assert.Equal(t, expectedTime2, memories[1].CreatedAt)
}

func TestGenerateMemoriesMarkdown(t *testing.T) {
	time1, _ := time.Parse("2006-01-02 15:04:05", "2023-06-15 14:30:00")
	time2, _ := time.Parse("2006-01-02 15:04:05", "2023-06-16 15:40:00")

	memories := []Memory{
		{
			ID:         "test-id-1",
			Title:      "测试记忆1",
			Content:    "这是测试记忆1的内容。",
			Tags:       []string{"测试", "记忆"},
			CreatedAt:  time1,
			Source:     "test",
			Importance: 5,
		},
		{
			ID:         "test-id-2",
			Title:      "测试记忆2",
			Content:    "这是测试记忆2的内容。\n这是第二行内容。",
			Tags:       []string{"测试", "重要"},
			CreatedAt:  time2,
			Source:     "test",
			Importance: 8,
		},
	}

	markdown := generateMemoriesMarkdown(memories)

	// 验证生成的Markdown包含预期的内容
	assert.Contains(t, markdown, "# Trading-GPT 记忆")
	assert.Contains(t, markdown, "## 记忆 1")
	assert.Contains(t, markdown, "- **ID**: test-id-1")
	assert.Contains(t, markdown, "- **标题**: 测试记忆1")
	assert.Contains(t, markdown, "- **标签**: 测试, 记忆")
	assert.Contains(t, markdown, "- **创建时间**: 2023-06-15 14:30:00")
	assert.Contains(t, markdown, "- **来源**: test")
	assert.Contains(t, markdown, "- **重要性**: 5")
	assert.Contains(t, markdown, "这是测试记忆1的内容。")

	assert.Contains(t, markdown, "## 记忆 2")
	assert.Contains(t, markdown, "- **ID**: test-id-2")
	assert.Contains(t, markdown, "- **标题**: 测试记忆2")
	assert.Contains(t, markdown, "- **标签**: 测试, 重要")
	assert.Contains(t, markdown, "- **创建时间**: 2023-06-16 15:40:00")
	assert.Contains(t, markdown, "- **来源**: test")
	assert.Contains(t, markdown, "- **重要性**: 8")
	assert.Contains(t, markdown, "这是测试记忆2的内容。\n这是第二行内容。")

	// 测试解析和生成的一致性
	parsedMemories, err := parseMemoriesFromMarkdown(markdown)
	assert.NoError(t, err)
	assert.Len(t, parsedMemories, 2)
	assert.Equal(t, memories[0].ID, parsedMemories[0].ID)
	assert.Equal(t, memories[0].Title, parsedMemories[0].Title)
	assert.Equal(t, memories[0].Content, parsedMemories[0].Content)
	assert.Equal(t, memories[0].Tags, parsedMemories[0].Tags)
	assert.Equal(t, memories[0].Source, parsedMemories[0].Source)
	assert.Equal(t, memories[0].Importance, parsedMemories[0].Importance)
	assert.Equal(t, memories[0].CreatedAt.Format(time.RFC3339), parsedMemories[0].CreatedAt.Format(time.RFC3339))

	assert.Equal(t, memories[1].ID, parsedMemories[1].ID)
	assert.Equal(t, memories[1].Title, parsedMemories[1].Title)
	assert.Equal(t, memories[1].Content, parsedMemories[1].Content)
	assert.Equal(t, memories[1].Tags, parsedMemories[1].Tags)
	assert.Equal(t, memories[1].Source, parsedMemories[1].Source)
	assert.Equal(t, memories[1].Importance, parsedMemories[1].Importance)
	assert.Equal(t, memories[1].CreatedAt.Format(time.RFC3339), parsedMemories[1].CreatedAt.Format(time.RFC3339))
}
