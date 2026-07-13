package scripted

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	btypes "github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yubing744/trading-gpt/pkg/config"
	"github.com/yubing744/trading-gpt/pkg/types"
	"github.com/yubing744/trading-gpt/pkg/utils"
)

func writeFixture(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fixture.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func sessionWithKLine(t *testing.T, start time.Time) types.ISession {
	t.Helper()

	session := types.NewMockSession("test")
	window := &btypes.KLineWindow{}
	window.Add(btypes.KLine{
		Symbol:    "SUIUSDT",
		Interval:  btypes.Interval5m,
		StartTime: btypes.Time(start),
		EndTime:   btypes.Time(start.Add(5 * time.Minute)),
	})
	session.SetAttribute("kline", window)
	return session
}

const fixtureJSON = `{
  "decisions": [
    {
      "at": "2026-07-08T02:00:00Z",
      "action": {"name": "open_long_position", "args": {"stop_loss_trigger_price": "3%", "take_profit_trigger_price": "5%"}},
      "speak": "scripted long entry"
    },
    {
      "at": "2026-07-08T06:00:00Z",
      "action": {"name": "close_position", "args": {}}
    }
  ]
}`

func TestScriptedAgentReplaysFixtureInOrder(t *testing.T) {
	agent, err := NewScriptedAgent(&config.ScriptedAgentConfig{
		Enabled:     true,
		FixtureFile: writeFixture(t, fixtureJSON),
	})
	require.NoError(t, err)

	ctx := context.Background()
	msgs := []*types.Message{{Text: "Analyze data, generate trading cmd"}}

	// Before the first decision is due -> no_action
	resp, err := agent.GenActions(ctx, sessionWithKLine(t, time.Date(2026, 7, 8, 1, 55, 0, 0, time.UTC)), msgs)
	require.NoError(t, err)
	result, err := utils.ParseResult(resp.Texts[0])
	require.NoError(t, err)
	assert.Equal(t, "no_action", result.Action.Name)

	// First decision due -> long entry with args preserved
	resp, err = agent.GenActions(ctx, sessionWithKLine(t, time.Date(2026, 7, 8, 2, 0, 0, 0, time.UTC)), msgs)
	require.NoError(t, err)
	result, err = utils.ParseResult(resp.Texts[0])
	require.NoError(t, err)
	assert.Equal(t, "open_long_position", result.Action.Name)
	assert.Equal(t, "3%", result.Action.Args["stop_loss_trigger_price"])
	assert.Equal(t, "scripted", resp.Model)

	// Decision fires only once
	resp, err = agent.GenActions(ctx, sessionWithKLine(t, time.Date(2026, 7, 8, 2, 5, 0, 0, time.UTC)), msgs)
	require.NoError(t, err)
	result, err = utils.ParseResult(resp.Texts[0])
	require.NoError(t, err)
	assert.Equal(t, "no_action", result.Action.Name)

	// A later kline picks up the second decision (>= semantics)
	resp, err = agent.GenActions(ctx, sessionWithKLine(t, time.Date(2026, 7, 8, 6, 10, 0, 0, time.UTC)), msgs)
	require.NoError(t, err)
	result, err = utils.ParseResult(resp.Texts[0])
	require.NoError(t, err)
	assert.Equal(t, "close_position", result.Action.Name)
}

func TestScriptedAgentWithoutKLineRepliesPlainText(t *testing.T) {
	agent, err := NewScriptedAgent(&config.ScriptedAgentConfig{Enabled: true})
	require.NoError(t, err)

	resp, err := agent.GenActions(context.Background(), types.NewMockSession("test"), []*types.Message{{Text: "warm up"}})
	require.NoError(t, err)
	require.Len(t, resp.Texts, 1)

	_, err = utils.ParseResult(resp.Texts[0])
	assert.Error(t, err, "warm-up reply should be plain text, not a JSON decision")
}

func TestLoadFixtureRejectsInvalidEntries(t *testing.T) {
	_, err := LoadFixture(writeFixture(t, `{"decisions": [{"at": "not-a-time", "action": {"name": "no_action"}}]}`))
	assert.Error(t, err)

	_, err = LoadFixture(writeFixture(t, `{"decisions": [{"at": "2026-07-08T02:00:00Z"}]}`))
	assert.Error(t, err)
}
