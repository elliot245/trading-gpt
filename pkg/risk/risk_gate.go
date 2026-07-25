package risk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	bbgotypes "github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

const (
	defaultStatePath                    = "memory-bank/risk-gate-state.json"
	defaultNormalBaseQuantity           = 20
	defaultReducedBaseQuantity          = 10
	defaultReduceAfterConsecutiveLosses = 2
	defaultPauseAfterConsecutiveLosses  = 4
	defaultPauseDuration                = 24 * time.Hour
	defaultCooldownDuration             = 12 * time.Hour
	defaultMaxDailyLoss                 = 5
)

type Config struct {
	Enabled *bool `json:"enabled,omitempty"`

	StatePath string `json:"state_path"`

	NormalBaseQuantity  fixedpoint.Value `json:"normal_base_quantity"`
	ReducedBaseQuantity fixedpoint.Value `json:"reduced_base_quantity"`

	ReduceAfterConsecutiveLosses int `json:"reduce_after_consecutive_losses"`
	PauseAfterConsecutiveLosses  int `json:"pause_after_consecutive_losses"`

	PauseDuration    bbgotypes.Duration `json:"pause_duration"`
	CooldownDuration bbgotypes.Duration `json:"cooldown_duration"`

	MaxDailyLoss fixedpoint.Value `json:"max_daily_loss"`

	OneTradePerUTCDay *bool `json:"one_trade_per_utc_day,omitempty"`
}

type Gate struct {
	cfg     Config
	state   State
	loadErr error
}

type State struct {
	Version int `json:"version"`

	ConsecutiveLosses int `json:"consecutive_losses"`

	LastOpenAtUTC   string `json:"last_open_at_utc,omitempty"`
	LastTradeDayUTC string `json:"last_trade_day_utc,omitempty"`

	CooldownUntilUTC string `json:"cooldown_until_utc,omitempty"`
	PauseUntilUTC    string `json:"pause_until_utc,omitempty"`

	DailyRealizedPnL map[string]string `json:"daily_realized_pnl"`
}

func NewGate(cfg Config) *Gate {
	cfg = cfg.withDefaults()
	state, err := loadState(cfg.StatePath)
	return &Gate{
		cfg:     cfg,
		state:   state,
		loadErr: err,
	}
}

func (c Config) EnabledValue() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

func (c Config) OneTradePerUTCDayValue() bool {
	if c.OneTradePerUTCDay == nil {
		return true
	}
	return *c.OneTradePerUTCDay
}

func (c Config) withDefaults() Config {
	if c.StatePath == "" {
		c.StatePath = defaultStatePath
	}
	if c.NormalBaseQuantity.IsZero() {
		c.NormalBaseQuantity = fixedpoint.NewFromInt(defaultNormalBaseQuantity)
	}
	if c.ReducedBaseQuantity.IsZero() {
		c.ReducedBaseQuantity = fixedpoint.NewFromInt(defaultReducedBaseQuantity)
	}
	if c.ReduceAfterConsecutiveLosses == 0 {
		c.ReduceAfterConsecutiveLosses = defaultReduceAfterConsecutiveLosses
	}
	if c.PauseAfterConsecutiveLosses == 0 {
		c.PauseAfterConsecutiveLosses = defaultPauseAfterConsecutiveLosses
	}
	if c.PauseDuration == 0 {
		c.PauseDuration = bbgotypes.Duration(defaultPauseDuration)
	}
	if c.CooldownDuration == 0 {
		c.CooldownDuration = bbgotypes.Duration(defaultCooldownDuration)
	}
	if c.MaxDailyLoss.IsZero() {
		c.MaxDailyLoss = fixedpoint.NewFromInt(defaultMaxDailyLoss)
	}
	return c
}

func (g *Gate) Config() Config {
	return g.cfg
}

func (g *Gate) State() State {
	return g.state
}

func (g *Gate) NextBaseQuantity(now time.Time) (fixedpoint.Value, error) {
	if !g.cfg.EnabledValue() {
		return g.cfg.NormalBaseQuantity, nil
	}
	if err := g.validateOpen(now); err != nil {
		return fixedpoint.Zero, err
	}
	if g.state.ConsecutiveLosses >= g.cfg.ReduceAfterConsecutiveLosses {
		return g.cfg.ReducedBaseQuantity, nil
	}
	return g.cfg.NormalBaseQuantity, nil
}

func (g *Gate) ReserveOpen(now time.Time) (fixedpoint.Value, error) {
	qty, err := g.NextBaseQuantity(now)
	if err != nil {
		return fixedpoint.Zero, err
	}
	if !g.cfg.EnabledValue() {
		return qty, nil
	}

	now = now.UTC()
	g.state.LastOpenAtUTC = now.Format(time.RFC3339)
	g.state.LastTradeDayUTC = utcDay(now)
	if err := g.save(); err != nil {
		g.loadErr = err
		return fixedpoint.Zero, err
	}
	return qty, nil
}

func (g *Gate) RecordRealizedPnL(now time.Time, pnl fixedpoint.Value) error {
	if !g.cfg.EnabledValue() {
		return nil
	}
	if g.loadErr != nil {
		return errors.Wrap(g.loadErr, "risk gate state unavailable")
	}

	now = now.UTC()
	day := utcDay(now)
	if g.state.DailyRealizedPnL == nil {
		g.state.DailyRealizedPnL = make(map[string]string)
	}
	current, err := fixedpoint.NewFromString(g.state.DailyRealizedPnL[day])
	if err != nil && g.state.DailyRealizedPnL[day] != "" {
		return errors.Wrapf(err, "invalid daily realized pnl for %s", day)
	}

	g.state.DailyRealizedPnL[day] = current.Add(pnl).String()
	g.state.CooldownUntilUTC = now.Add(time.Duration(g.cfg.CooldownDuration)).Format(time.RFC3339)

	if pnl.Sign() < 0 {
		g.state.ConsecutiveLosses++
		if g.state.ConsecutiveLosses >= g.cfg.PauseAfterConsecutiveLosses {
			g.state.PauseUntilUTC = now.Add(time.Duration(g.cfg.PauseDuration)).Format(time.RFC3339)
		}
	} else if pnl.Sign() > 0 {
		g.state.ConsecutiveLosses = 0
		g.state.PauseUntilUTC = ""
	}

	if err := g.save(); err != nil {
		g.loadErr = err
		return err
	}
	return nil
}

func (g *Gate) validateOpen(now time.Time) error {
	if g.loadErr != nil {
		return errors.Wrap(g.loadErr, "risk gate state unavailable")
	}
	if g.state.Version == 0 {
		g.state.Version = 1
	}
	if g.state.DailyRealizedPnL == nil {
		g.state.DailyRealizedPnL = make(map[string]string)
	}

	now = now.UTC()
	if until, ok, err := parseOptionalUTC(g.state.PauseUntilUTC); err != nil {
		return err
	} else if ok && now.Before(until) {
		return fmt.Errorf("risk gate hard pause active until %s", until.Format(time.RFC3339))
	}

	if until, ok, err := parseOptionalUTC(g.state.CooldownUntilUTC); err != nil {
		return err
	} else if ok && now.Before(until) {
		return fmt.Errorf("risk gate cooldown active until %s", until.Format(time.RFC3339))
	}

	day := utcDay(now)
	if g.cfg.OneTradePerUTCDayValue() && g.state.LastTradeDayUTC == day {
		return fmt.Errorf("risk gate one-trade-per-UTC-day already used for %s", day)
	}

	if loss, err := dailyLoss(g.state, day); err != nil {
		return err
	} else if loss.Compare(g.cfg.MaxDailyLoss) >= 0 {
		return fmt.Errorf("risk gate max daily quote loss reached for %s: %s >= %s", day, loss.String(), g.cfg.MaxDailyLoss.String())
	}

	return nil
}

func dailyLoss(state State, day string) (fixedpoint.Value, error) {
	raw := state.DailyRealizedPnL[day]
	if raw == "" {
		return fixedpoint.Zero, nil
	}
	pnl, err := fixedpoint.NewFromString(raw)
	if err != nil {
		return fixedpoint.Zero, errors.Wrapf(err, "invalid daily realized pnl for %s", day)
	}
	if pnl.Sign() >= 0 {
		return fixedpoint.Zero, nil
	}
	return pnl.Abs(), nil
}

func loadState(path string) (State, error) {
	state := State{
		Version:          1,
		DailyRealizedPnL: make(map[string]string),
	}

	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return State{}, err
	}
	if state.Version != 1 {
		return State{}, fmt.Errorf("unsupported risk gate state version %d", state.Version)
	}
	if state.DailyRealizedPnL == nil {
		state.DailyRealizedPnL = make(map[string]string)
	}
	return state, nil
}

func (g *Gate) save() error {
	if g.loadErr != nil {
		return errors.Wrap(g.loadErr, "risk gate state unavailable")
	}
	if g.state.Version == 0 {
		g.state.Version = 1
	}
	if g.state.DailyRealizedPnL == nil {
		g.state.DailyRealizedPnL = make(map[string]string)
	}

	if err := os.MkdirAll(filepath.Dir(g.cfg.StatePath), 0755); err != nil {
		return errors.Wrap(err, "create risk gate state directory")
	}

	body, err := json.MarshalIndent(g.state, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := g.cfg.StatePath + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0644); err != nil {
		return errors.Wrap(err, "write risk gate state")
	}
	if err := os.Rename(tmpPath, g.cfg.StatePath); err != nil {
		return errors.Wrap(err, "replace risk gate state")
	}
	return nil
}

func parseOptionalUTC(raw string) (time.Time, bool, error) {
	if raw == "" {
		return time.Time{}, false, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false, errors.Wrapf(err, "invalid risk gate timestamp %q", raw)
	}
	return t.UTC(), true, nil
}

func utcDay(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}
