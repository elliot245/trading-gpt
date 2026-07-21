package twitterapi

import (
	"github.com/yubing744/trading-gpt/pkg/types"
	"github.com/yubing744/trading-gpt/pkg/utils"
)

// TwitterAPIEvent represents an event that is specific to interactions with the Twitter API platform.
type TwitterAPIEvent struct {
	types.Event // Embed the base Event struct to reuse its implementation.
	title       string
	Content     string
}

// NewTwitterAPIEvent creates a new instance of TwitterAPIEvent with the given type and data.
func NewTwitterAPIEvent(name string, title string, content string) *TwitterAPIEvent {
	return &TwitterAPIEvent{
		Event:   *types.NewEvent(name, content),
		title:   title,
		Content: content,
	}
}

// ToPrompts wraps the external content in an explicit untrusted-data isolation
// block so any injected instruction inside tweets is treated as data, not a
// command, and the content cannot forge prompt structure. See utils.WrapUntrusted.
func (e *TwitterAPIEvent) ToPrompts() []string {
	return []string{utils.WrapUntrusted(e.title, e.Content, 4000)}
}
