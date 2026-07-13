package exchange

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/yubing744/trading-gpt/pkg/config"
)

func fpf(f float64) fixedpoint.Value { return fixedpoint.NewFromFloat(f) }

func boolPtr(b bool) *bool { return &b }

// baseCfg is a fully-configured (all limits set) gate config used by most tests.
func baseCfg() config.RiskControlConfig {
	return config.RiskControlConfig{
		Enabled:          boolPtr(true),
		MaxLeverage:      fpf(3),
		MaxOrderQuote:    fpf(100),
		MaxPositionQuote: fpf(150),
		MaxDailyLoss:     fpf(50),
	}
}

func TestEvaluateRisk_AllowsWithinAllLimits(t *testing.T) {
	in := RiskEvalInput{
		OrderNotional:    fpf(45),
		PositionNotional: fpf(0),
		Leverage:         fpf(3),
		DailyRealizedPnL: fpf(-10),
	}
	d := EvaluateRisk(in, baseCfg())
	if !d.Allow {
		t.Fatalf("expected allow, got deny: %s", d.Reason)
	}
	if d.Code != "" {
		t.Fatalf("expected empty code on allow, got %q", d.Code)
	}
}

func TestEvaluateRisk_DisabledMasterSwitchAllowsEverything(t *testing.T) {
	cfg := baseCfg()
	cfg.Enabled = boolPtr(false)
	in := RiskEvalInput{
		OrderNotional:     fpf(10000),
		PositionNotional:  fpf(10000),
		Leverage:          fpf(100),
		DailyRealizedPnL:  fpf(-9999),
		KillSwitchLatched: true,
	}
	if d := EvaluateRisk(in, cfg); !d.Allow {
		t.Fatalf("disabled gate must allow, got deny: %s", d.Reason)
	}
}

func TestEvaluateRisk_ZeroThresholdsDisableIndividualChecks(t *testing.T) {
	// Enabled but no thresholds set -> everything allowed (no behaviour change).
	cfg := config.RiskControlConfig{Enabled: boolPtr(true)}
	in := RiskEvalInput{
		OrderNotional:    fpf(1e9),
		PositionNotional: fpf(1e9),
		Leverage:         fpf(125),
		DailyRealizedPnL: fpf(-1e9),
	}
	if d := EvaluateRisk(in, cfg); !d.Allow {
		t.Fatalf("zero thresholds must allow, got deny: %s", d.Reason)
	}
}

func TestEvaluateRisk_MaxLeverage(t *testing.T) {
	cfg := baseCfg()
	// exactly at limit -> allowed
	if d := EvaluateRisk(RiskEvalInput{Leverage: fpf(3), OrderNotional: fpf(1)}, cfg); !d.Allow {
		t.Fatalf("leverage at limit must be allowed")
	}
	// just above -> denied
	d := EvaluateRisk(RiskEvalInput{Leverage: fpf(3.0001), OrderNotional: fpf(1)}, cfg)
	if d.Allow || d.Code != RiskCodeMaxLeverage {
		t.Fatalf("expected max_leverage deny, got allow=%v code=%s", d.Allow, d.Code)
	}
}

func TestEvaluateRisk_MaxOrderQuote(t *testing.T) {
	cfg := baseCfg()
	if d := EvaluateRisk(RiskEvalInput{OrderNotional: fpf(100), Leverage: fpf(1)}, cfg); !d.Allow {
		t.Fatalf("order notional at limit must be allowed")
	}
	d := EvaluateRisk(RiskEvalInput{OrderNotional: fpf(100.01), Leverage: fpf(1)}, cfg)
	if d.Allow || d.Code != RiskCodeMaxOrder {
		t.Fatalf("expected max_order deny, got allow=%v code=%s", d.Allow, d.Code)
	}
}

func TestEvaluateRisk_MaxPositionQuote(t *testing.T) {
	cfg := baseCfg()
	// existing 120 + order 30 = 150 == limit -> allowed
	if d := EvaluateRisk(RiskEvalInput{PositionNotional: fpf(120), OrderNotional: fpf(30), Leverage: fpf(1)}, cfg); !d.Allow {
		t.Fatalf("total notional at limit must be allowed")
	}
	// existing 120 + order 31 = 151 > 150 -> denied
	d := EvaluateRisk(RiskEvalInput{PositionNotional: fpf(120), OrderNotional: fpf(31), Leverage: fpf(1)}, cfg)
	if d.Allow || d.Code != RiskCodeMaxPosition {
		t.Fatalf("expected max_position deny, got allow=%v code=%s", d.Allow, d.Code)
	}
}

func TestEvaluateRisk_KillSwitchFromDailyLoss(t *testing.T) {
	cfg := baseCfg() // MaxDailyLoss = 50
	// loss of 49 -> still allowed
	if d := EvaluateRisk(RiskEvalInput{DailyRealizedPnL: fpf(-49), OrderNotional: fpf(1), Leverage: fpf(1)}, cfg); !d.Allow {
		t.Fatalf("loss below limit must be allowed")
	}
	// loss of exactly 50 -> tripped
	d := EvaluateRisk(RiskEvalInput{DailyRealizedPnL: fpf(-50), OrderNotional: fpf(1), Leverage: fpf(1)}, cfg)
	if d.Allow || d.Code != RiskCodeKillSwitch {
		t.Fatalf("loss at limit must trip kill-switch, got allow=%v code=%s", d.Allow, d.Code)
	}
	// profit -> not tripped
	if d := EvaluateRisk(RiskEvalInput{DailyRealizedPnL: fpf(1000), OrderNotional: fpf(1), Leverage: fpf(1)}, cfg); !d.Allow {
		t.Fatalf("profit must never trip kill-switch")
	}
}

func TestEvaluateRisk_LatchedKillSwitchDeniesEvenWithoutLoss(t *testing.T) {
	cfg := baseCfg()
	d := EvaluateRisk(RiskEvalInput{
		DailyRealizedPnL:  fpf(0),
		KillSwitchLatched: true,
		OrderNotional:     fpf(1),
		Leverage:          fpf(1),
	}, cfg)
	if d.Allow || d.Code != RiskCodeKillSwitch {
		t.Fatalf("latched kill-switch must deny, got allow=%v code=%s", d.Allow, d.Code)
	}
}

func TestEvaluateRisk_KillSwitchTakesPrecedence(t *testing.T) {
	// Even if leverage/notional are fine, a tripped kill-switch denies first.
	cfg := baseCfg()
	d := EvaluateRisk(RiskEvalInput{
		DailyRealizedPnL: fpf(-60),
		OrderNotional:    fpf(1),
		Leverage:         fpf(1),
	}, cfg)
	if d.Code != RiskCodeKillSwitch {
		t.Fatalf("kill-switch must take precedence, got code=%s", d.Code)
	}
}

// --- Stateful RiskGate tests ---

func newGateAt(cfg config.RiskControlConfig, clock *time.Time) *RiskGate {
	g := NewRiskGate(cfg)
	g.now = func() time.Time { return *clock }
	*clock = time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	g.day = truncateDay(*clock)
	return g
}

func TestRiskGate_RecordAndTripKillSwitch(t *testing.T) {
	clock := time.Now()
	g := newGateAt(baseCfg(), &clock)

	g.RecordRealizedPnL(fpf(-20))
	if g.KillSwitchActive() {
		t.Fatalf("should not be tripped after -20 (< 50)")
	}
	g.RecordRealizedPnL(fpf(-35)) // cumulative -55 >= 50
	if !g.KillSwitchActive() {
		t.Fatalf("should be tripped after cumulative -55")
	}

	d := g.Evaluate(fpf(10), fpf(0), fpf(1))
	if d.Allow || d.Code != RiskCodeKillSwitch {
		t.Fatalf("gate must deny while kill-switch latched, got allow=%v code=%s", d.Allow, d.Code)
	}
}

func TestRiskGate_KillSwitchLatchesDespiteRecovery(t *testing.T) {
	clock := time.Now()
	g := newGateAt(baseCfg(), &clock)

	g.RecordRealizedPnL(fpf(-60)) // trip
	g.RecordRealizedPnL(fpf(100)) // recover to +40 same day
	if !g.KillSwitchActive() {
		t.Fatalf("kill-switch must stay latched for the rest of the day despite recovery")
	}
	if d := g.Evaluate(fpf(10), fpf(0), fpf(1)); d.Allow {
		t.Fatalf("gate must keep denying while latched same day")
	}
}

func TestRiskGate_ResetsOnNewTradingDay(t *testing.T) {
	clock := time.Now()
	g := newGateAt(baseCfg(), &clock)

	g.RecordRealizedPnL(fpf(-60)) // trip today
	if !g.KillSwitchActive() {
		t.Fatalf("expected tripped on day 1")
	}

	// advance to next day
	clock = time.Date(2026, 7, 14, 0, 0, 1, 0, time.UTC)

	if g.KillSwitchActive() {
		t.Fatalf("kill-switch must reset on new trading day")
	}
	if g.DailyRealizedPnL().Sign() != 0 {
		t.Fatalf("daily realized PnL must reset to zero on new day, got %v", g.DailyRealizedPnL().Float64())
	}
	if d := g.Evaluate(fpf(45), fpf(0), fpf(3)); !d.Allow {
		t.Fatalf("gate must allow normal order on the new day, got deny: %s", d.Reason)
	}
}

func TestRiskGate_NotionalChecksThroughGate(t *testing.T) {
	clock := time.Now()
	g := newGateAt(baseCfg(), &clock)

	if d := g.Evaluate(fpf(45), fpf(0), fpf(3)); !d.Allow {
		t.Fatalf("normal order must be allowed")
	}
	if d := g.Evaluate(fpf(200), fpf(0), fpf(3)); d.Allow || d.Code != RiskCodeMaxOrder {
		t.Fatalf("oversized single order must be denied, got allow=%v code=%s", d.Allow, d.Code)
	}
	if d := g.Evaluate(fpf(40), fpf(120), fpf(3)); d.Allow || d.Code != RiskCodeMaxPosition {
		t.Fatalf("oversized total must be denied, got allow=%v code=%s", d.Allow, d.Code)
	}
}

func TestRiskGate_DefaultEnabledWhenNil(t *testing.T) {
	cfg := config.RiskControlConfig{MaxDailyLoss: fpf(50)} // Enabled nil -> true
	clock := time.Now()
	g := newGateAt(cfg, &clock)
	g.RecordRealizedPnL(fpf(-60))
	if d := g.Evaluate(fpf(1), fpf(0), fpf(1)); d.Allow {
		t.Fatalf("nil Enabled must default to enabled; kill-switch should deny")
	}
}

func TestRiskControlConfig_IsEnabled(t *testing.T) {
	if !(config.RiskControlConfig{}).IsEnabled() {
		t.Fatalf("nil Enabled must default to true")
	}
	if (config.RiskControlConfig{Enabled: boolPtr(false)}).IsEnabled() {
		t.Fatalf("explicit false must disable")
	}
}
