package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("module", "memory")

// MemoryManager 实现记忆管理功能
type MemoryManager struct {
	filePath    string
	memories    []Memory
	mutex       sync.RWMutex
	initialized bool
}

// NewMemoryManager 创建一个新的记忆管理器
func NewMemoryManager(filePath string) *MemoryManager {
	return &MemoryManager{
		filePath:    filePath,
		memories:    make([]Memory, 0),
		initialized: false,
	}
}

// Initialize 初始化记忆管理器，加载现有记忆
func (m *MemoryManager) Initialize(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.initialized {
		return nil
	}

	// 确保目录存在
	dir := filepath.Dir(m.filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrap(err, "failed to create memory directory")
		}
	}

	// 加载记忆文件
	if _, err := os.Stat(m.filePath); !os.IsNotExist(err) {
		if err := m.loadMemoriesFromFile(); err != nil {
			return errors.Wrap(err, "failed to load memories from file")
		}
	} else {
		// 如果文件不存在，创建一个空的记忆文件
		if err := m.SaveMemories(ctx); err != nil {
			return errors.Wrap(err, "failed to create initial memory file")
		}
	}

	m.initialized = true
	return nil
}

// AddMemory 添加新记忆
func (m *MemoryManager) AddMemory(ctx context.Context, title, content string, tags []string, source string, importance int) (*Memory, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.initialized {
		return nil, errors.New("memory manager not initialized")
	}

	memory := Memory{
		ID:         uuid.New().String(),
		Title:      title,
		Content:    content,
		Tags:       tags,
		CreatedAt:  time.Now(),
		Source:     source,
		Importance: importance,
	}

	m.memories = append(m.memories, memory)

	// 保存到文件
	if err := m.saveMemoriesToFile(); err != nil {
		return nil, errors.Wrap(err, "failed to save memory to file")
	}

	return &memory, nil
}

// GetAllMemories 获取所有记忆
func (m *MemoryManager) GetAllMemories(ctx context.Context) ([]Memory, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if !m.initialized {
		return nil, errors.New("memory manager not initialized")
	}

	// 返回记忆的副本，避免外部修改
	memories := make([]Memory, len(m.memories))
	copy(memories, m.memories)

	return memories, nil
}

// RetrieveMemories 基本检索功能
func (m *MemoryManager) RetrieveMemories(ctx context.Context, query string, tags []string, limit int) ([]Memory, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if !m.initialized {
		return nil, errors.New("memory manager not initialized")
	}

	// 如果没有查询条件，返回所有记忆
	if query == "" && len(tags) == 0 {
		memories := make([]Memory, len(m.memories))
		copy(memories, m.memories)
		return memories, nil
	}

	// 基于关键词和标签进行简单过滤
	var results []Memory
	for _, memory := range m.memories {
		// 检查标题和内容是否包含查询关键词
		if query != "" && !strings.Contains(strings.ToLower(memory.Title), strings.ToLower(query)) && !strings.Contains(strings.ToLower(memory.Content), strings.ToLower(query)) {
			continue
		}

		// 检查标签是否匹配
		if len(tags) > 0 {
			matched := false
			for _, tag := range tags {
				for _, memoryTag := range memory.Tags {
					if strings.ToLower(tag) == strings.ToLower(memoryTag) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		results = append(results, memory)
	}

	// 限制结果数量
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// SaveMemories 保存所有记忆到文件
func (m *MemoryManager) SaveMemories(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.saveMemoriesToFile()
}

// loadMemoriesFromFile 从文件加载记忆
func (m *MemoryManager) loadMemoriesFromFile() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return errors.Wrap(err, "failed to read memory file")
	}

	content := string(data)
	if content == "" {
		// 文件为空，初始化为空记忆列表
		m.memories = make([]Memory, 0)
		return nil
	}

	// 解析Markdown文件
	memories, err := parseMemoriesFromMarkdown(content)
	if err != nil {
		return errors.Wrap(err, "failed to parse memories from markdown")
	}

	m.memories = memories
	return nil
}

// saveMemoriesToFile 将记忆保存到文件
func (m *MemoryManager) saveMemoriesToFile() error {
	content := generateMemoriesMarkdown(m.memories)

	// 确保目录存在
	dir := filepath.Dir(m.filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrap(err, "failed to create memory directory")
		}
	}

	// 写入文件
	if err := os.WriteFile(m.filePath, []byte(content), 0644); err != nil {
		return errors.Wrap(err, "failed to write memory file")
	}

	return nil
}

// parseMemoriesFromMarkdown 从Markdown内容解析记忆
func parseMemoriesFromMarkdown(content string) ([]Memory, error) {
	var memories []Memory

	// 按照记忆分隔符分割内容
	sections := strings.Split(content, "## 记忆 ")
	if len(sections) <= 1 {
		// 没有找到记忆部分
		return memories, nil
	}

	// 跳过第一部分（标题部分）
	for i := 1; i < len(sections); i++ {
		section := sections[i]
		lines := strings.Split(section, "\n")
		if len(lines) < 6 {
			// 记忆格式不正确，跳过
			continue
		}

		var memory Memory
		var contentLines []string

		// 解析元数据
		for j, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				// 元数据结束，剩余部分为内容
				contentLines = lines[j+1:]
				break
			}

			if strings.HasPrefix(line, "- **ID**:") {
				memory.ID = strings.TrimSpace(strings.TrimPrefix(line, "- **ID**:"))
			} else if strings.HasPrefix(line, "- **标题**:") {
				memory.Title = strings.TrimSpace(strings.TrimPrefix(line, "- **标题**:"))
			} else if strings.HasPrefix(line, "- **标签**:") {
				tagStr := strings.TrimSpace(strings.TrimPrefix(line, "- **标签**:"))
				tags := strings.Split(tagStr, ",")
				for k, tag := range tags {
					tags[k] = strings.TrimSpace(tag)
				}
				memory.Tags = tags
			} else if strings.HasPrefix(line, "- **创建时间**:") {
				timeStr := strings.TrimSpace(strings.TrimPrefix(line, "- **创建时间**:"))
				t, err := time.Parse("2006-01-02 15:04:05", timeStr)
				if err != nil {
					// 尝试其他时间格式
					t, err = time.Parse(time.RFC3339, timeStr)
					if err != nil {
						log.Warnf("Failed to parse time: %s, error: %v", timeStr, err)
						// 使用当前时间作为默认值
						t = time.Now()
					}
				}
				memory.CreatedAt = t
			} else if strings.HasPrefix(line, "- **来源**:") {
				memory.Source = strings.TrimSpace(strings.TrimPrefix(line, "- **来源**:"))
			} else if strings.HasPrefix(line, "- **重要性**:") {
				importanceStr := strings.TrimSpace(strings.TrimPrefix(line, "- **重要性**:"))
				fmt.Sscanf(importanceStr, "%d", &memory.Importance)
			}
		}

		// 解析内容
		memory.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))

		// 如果ID为空，生成一个新的
		if memory.ID == "" {
			memory.ID = uuid.New().String()
		}

		memories = append(memories, memory)
	}

	return memories, nil
}

// generateMemoriesMarkdown 生成记忆的Markdown内容
func generateMemoriesMarkdown(memories []Memory) string {
	var builder strings.Builder

	builder.WriteString("# Trading-GPT 记忆\n\n")

	for i, memory := range memories {
		builder.WriteString(fmt.Sprintf("## 记忆 %d\n", i+1))
		builder.WriteString(fmt.Sprintf("- **ID**: %s\n", memory.ID))
		builder.WriteString(fmt.Sprintf("- **标题**: %s\n", memory.Title))
		builder.WriteString(fmt.Sprintf("- **标签**: %s\n", strings.Join(memory.Tags, ", ")))
		builder.WriteString(fmt.Sprintf("- **创建时间**: %s\n", memory.CreatedAt.Format("2006-01-02 15:04:05")))
		builder.WriteString(fmt.Sprintf("- **来源**: %s\n", memory.Source))
		builder.WriteString(fmt.Sprintf("- **重要性**: %d\n\n", memory.Importance))
		builder.WriteString(memory.Content)
		builder.WriteString("\n\n")
	}

	return builder.String()
}
