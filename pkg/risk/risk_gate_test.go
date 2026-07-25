package risk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	bbgotypes "github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestGateRecentThreeLossReplayUsesQuotePnLAndReducesSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "risk-state.json")
	g := NewGate(Config{StatePath: path})

	t0 := time.Date(2026, 7, 21, 4, 0, 0, 0, time.UTC)
	require.NoError(t, g.RecordRealizedPnL(t0, fp("-0.0517139551796317")))
	require.Equal(t, 1, g.State().ConsecutiveLosses)

	qty, err := g.NextBaseQuantity(t0.Add(13 * time.Hour))
	require.NoError(t, err)
	require.Equal(t, "20", qty.String())

	require.NoError(t, g.RecordRealizedPnL(t0.Add(24*time.Hour), fp("-1.1117861741518928")))
	require.Equal(t, 2, g.State().ConsecutiveLosses)

	qty, err = g.NextBaseQuantity(t0.Add(37 * time.Hour))
	require.NoError(t, err)
	require.Equal(t, "10", qty.String())

	require.NoError(t, g.RecordRealizedPnL(t0.Add(48*time.Hour), fp("-1.0099089968405204")))
	require.Equal(t, 3, g.State().ConsecutiveLosses)

	qty, err = g.NextBaseQuantity(t0.Add(61 * time.Hour))
	require.NoError(t, err)
	require.Equal(t, "10", qty.String())
	require.Equal(t, "-0.05171395", g.State().DailyRealizedPnL["2026-07-21"])
	require.Equal(t, "-1.11178617", g.State().DailyRealizedPnL["2026-07-22"])
	require.Equal(t, "-1.00990899", g.State().DailyRealizedPnL["2026-07-23"])
}

func TestGateRestartPersistsConsecutiveLossCooldownAndOneTradeDay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "risk-state.json")
	first := NewGate(Config{StatePath: path})

	openedAt := time.Date(2026, 7, 25, 1, 0, 0, 0, time.UTC)
	qty, err := first.ReserveOpen(openedAt)
	require.NoError(t, err)
	require.Equal(t, "20", qty.String())

	restarted := NewGate(Config{StatePath: path})
	_, err = restarted.NextBaseQuantity(openedAt.Add(time.Hour))
	require.ErrorContains(t, err, "one-trade-per-UTC-day")

	require.NoError(t, restarted.RecordRealizedPnL(openedAt.Add(2*time.Hour), fp("-0.25")))

	again := NewGate(Config{StatePath: path})
	_, err = again.NextBaseQuantity(openedAt.Add(13 * time.Hour))
	require.ErrorContains(t, err, "cooldown")

	_, err = again.NextBaseQuantity(openedAt.Add(15 * time.Hour))
	require.ErrorContains(t, err, "one-trade-per-UTC-day")

	qty, err = again.NextBaseQuantity(time.Date(2026, 7, 26, 3, 1, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, "20", qty.String())
}

func TestGateUTCDayRolloverDoesNotResetLossLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "risk-state.json")
	g := NewGate(Config{StatePath: path})

	_, err := g.ReserveOpen(time.Date(2026, 7, 25, 23, 50, 0, 0, time.UTC))
	require.NoError(t, err)

	_, err = g.NextBaseQuantity(time.Date(2026, 7, 26, 0, 1, 0, 0, time.UTC))
	require.NoError(t, err)

	require.NoError(t, g.RecordRealizedPnL(time.Date(2026, 7, 26, 0, 10, 0, 0, time.UTC), fp("-0.1")))
	restarted := NewGate(Config{StatePath: path})
	require.Equal(t, "-0.1", restarted.State().DailyRealizedPnL["2026-07-26"])
	require.Equal(t, "2026-07-25", restarted.State().LastTradeDayUTC)
}

func TestGateDailyQuoteLossBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "risk-state.json")
	g := NewGate(Config{
		StatePath:        path,
		CooldownDuration: bbgotypes.Duration(time.Nanosecond),
	})
	now := time.Date(2026, 7, 25, 4, 0, 0, 0, time.UTC)

	require.NoError(t, g.RecordRealizedPnL(now, fp("-4.9999")))
	_, err := g.NextBaseQuantity(now.Add(13 * time.Hour))
	require.NoError(t, err)

	require.NoError(t, g.RecordRealizedPnL(now.Add(14*time.Hour), fp("-0.0001")))
	_, err = g.NextBaseQuantity(now.Add(14*time.Hour + time.Second))
	require.ErrorContains(t, err, "max daily quote loss")
}

func TestGateFourLossHardPause(t *testing.T) {
	path := filepath.Join(t.TempDir(), "risk-state.json")
	g := NewGate(Config{StatePath: path})
	now := time.Date(2026, 7, 25, 4, 0, 0, 0, time.UTC)

	for i := 0; i < 4; i++ {
		require.NoError(t, g.RecordRealizedPnL(now.Add(time.Duration(i)*13*time.Hour), fp("-0.1")))
	}

	_, err := g.NextBaseQuantity(now.Add(52 * time.Hour))
	require.ErrorContains(t, err, "hard pause")

	qty, err := g.NextBaseQuantity(now.Add(76 * time.Hour))
	require.NoError(t, err)
	require.Equal(t, "10", qty.String())
}

func TestGateCorruptStateFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "risk-state.json")
	require.NoError(t, os.WriteFile(path, []byte("{not-json"), 0644))

	g := NewGate(Config{StatePath: path})
	_, err := g.NextBaseQuantity(time.Date(2026, 7, 25, 4, 0, 0, 0, time.UTC))
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "risk gate state unavailable"))
}

func fp(raw string) fixedpoint.Value {
	v, err := fixedpoint.NewFromString(raw)
	if err != nil {
		panic(err)
	}
	return v
}
