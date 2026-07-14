package coze

import (
	"github.com/yubing744/trading-gpt/pkg/types"
	"github.com/yubing744/trading-gpt/pkg/utils"
)

// CozeEvent represents an event that is specific to interactions with the Coze platform.
type CozeEvent struct {
	types.Event // Embed the base Event struct to reuse its implementation.
	title       string
	Content     string
}

// NewCozeEvent creates a new instance of CozeEvent with the given type and data.
func NewCozeEvent(name string, title string, content string) *CozeEvent {
	return &CozeEvent{
		Event:   *types.NewEvent(name, content),
		title:   title,
		Content: content,
	}
}

// ToPrompts wraps the external news content in an explicit untrusted-data
// isolation block so any injected instruction inside it is treated as data, not a
// command, and the content cannot forge prompt structure. See utils.WrapUntrusted.
func (e *CozeEvent) ToPrompts() []string {
	return []string{utils.WrapUntrusted(e.title, e.Content, 4000)}
}
