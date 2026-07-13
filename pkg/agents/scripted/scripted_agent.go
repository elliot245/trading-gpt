// Package scripted implements a deterministic replay agent for the offline
// backtest harness. It satisfies the same agents.IAgent seam as the
// LLM-backed trading agent, but instead of calling a model it replays
// decisions from a JSON fixture keyed by kline time. This makes backtest
// runs fully reproducible: the same klines + the same fixture always yield
// the same command stream, so fixed-vs-unfixed builds (or prompt/model
// variants swapped in behind the same seam) can be compared run-to-run.
package scripted

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	btypes "github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/yubing744/trading-gpt/pkg/agents"
	"github.com/yubing744/trading-gpt/pkg/config"
	"github.com/yubing744/trading-gpt/pkg/types"
)

var log = logrus.WithField("agent", "scripted")

// Decision is one scripted decision. It fires on the first agent invocation
// whose last closed kline start time is >= At; each decision fires at most
// once, in file order.
type Decision struct {
	// At is the activation time in RFC3339 (e.g. "2026-07-08T02:00:00Z").
	At string `json:"at"`

	// Action is emitted verbatim (entity-qualified names like
	// "exchange.close_position" are allowed; unqualified names default to the
	// exchange entity, mirroring live behaviour).
	Action *types.Action `json:"action"`

	// Speak is an optional human-readable rationale echoed to the session.
	Speak string `json:"speak,omitempty"`

	at time.Time
}

// Fixture is the top-level fixture file format.
type Fixture struct {
	// Decisions are matched in order; see Decision.At.
	Decisions []*Decision `json:"decisions"`
}

// ScriptedAgent replays fixture decisions; when no decision matches it
// answers with a no_action decision, keeping the strategy loop flowing.
type ScriptedAgent struct {
	name string

	mu        sync.Mutex
	decisions []*Decision
	consumed  []bool
}

func NewScriptedAgent(cfg *config.ScriptedAgentConfig) (*ScriptedAgent, error) {
	name := "ScriptedAI"
	if cfg != nil && cfg.Name != "" {
		name = cfg.Name
	}

	agent := &ScriptedAgent{
		name: name,
	}

	if cfg != nil && cfg.FixtureFile != "" {
		fixture, err := LoadFixture(cfg.FixtureFile)
		if err != nil {
			return nil, err
		}

		agent.decisions = fixture.Decisions
		agent.consumed = make([]bool, len(fixture.Decisions))

		log.WithField("fixture_file", cfg.FixtureFile).
			WithField("decisions", len(fixture.Decisions)).
			Info("scripted agent fixture loaded")
	} else {
		log.Warn("scripted agent has no fixture file; every decision will be no_action")
	}

	return agent, nil
}

// LoadFixture reads and validates a fixture JSON file.
func LoadFixture(path string) (*Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "read fixture file %s", path)
	}

	var fixture Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return nil, errors.Wrapf(err, "parse fixture file %s", path)
	}

	for i, d := range fixture.Decisions {
		if d == nil || d.Action == nil || d.Action.Name == "" {
			return nil, errors.Errorf("fixture %s: decision #%d has no action", path, i)
		}

		at, err := time.Parse(time.RFC3339, d.At)
		if err != nil {
			return nil, errors.Wrapf(err, "fixture %s: decision #%d has invalid 'at' time %q", path, i, d.At)
		}
		d.at = at
	}

	return &fixture, nil
}

func (a *ScriptedAgent) GetName() string {
	return "scripted"
}

func (a *ScriptedAgent) Start() error {
	return nil
}

func (a *ScriptedAgent) Stop() {
}

// GenActions implements agents.IAgent. The decision is selected off the last
// closed kline's start time (stored in the session by the strategy), which is
// the simulated clock during a backtest.
func (a *ScriptedAgent) GenActions(ctx context.Context, session types.ISession, msgs []*types.Message) (*agents.GenResult, error) {
	klineTime, ok := a.klineTime(session)
	if !ok {
		// Startup warm-up message arrives before any kline; reply with plain
		// text (non-JSON), which the strategy simply echoes.
		return &agents.GenResult{
			Texts: []string{fmt.Sprintf("%s ready: scripted replay agent, waiting for kline data.", a.name)},
			Model: "scripted",
		}, nil
	}

	decision := a.nextDecision(klineTime)

	action := &types.Action{Name: "no_action", Args: map[string]string{}}
	speak := fmt.Sprintf("No scripted decision due at %s; holding.", klineTime.Format(time.RFC3339))
	if decision != nil {
		action = decision.Action
		if decision.Speak != "" {
			speak = decision.Speak
		} else {
			speak = fmt.Sprintf("Scripted decision %s due at %s.", action.Name, decision.At)
		}
	}

	result := &types.Result{
		Thoughts: &types.Thoughts{
			Plan:    "Replay scripted decision fixture",
			Analyze: fmt.Sprintf("kline time: %s", klineTime.Format(time.RFC3339)),
			Speak:   speak,
		},
		Action: action,
	}

	text, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "marshal scripted result")
	}

	log.WithField("kline_time", klineTime).
		WithField("action", action.Name).
		Info("scripted agent decision")

	return &agents.GenResult{
		Texts: []string{string(text)},
		Model: "scripted",
	}, nil
}

// klineTime extracts the last closed kline start time from the session.
func (a *ScriptedAgent) klineTime(session types.ISession) (time.Time, bool) {
	raw, ok := session.GetAttribute("kline")
	if !ok {
		return time.Time{}, false
	}

	window, ok := raw.(*btypes.KLineWindow)
	if !ok || window == nil || window.Len() == 0 {
		return time.Time{}, false
	}

	return window.Last().StartTime.Time(), true
}

// nextDecision returns the first unconsumed decision due at or before t.
func (a *ScriptedAgent) nextDecision(t time.Time) *Decision {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, d := range a.decisions {
		if a.consumed[i] {
			continue
		}

		if !t.Before(d.at) {
			a.consumed[i] = true
			return d
		}
	}

	return nil
}
