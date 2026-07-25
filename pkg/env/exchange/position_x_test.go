package exchange

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestPositionXUpdateProfitDoesNotClobberAuthoritativeRealizedPnL(t *testing.T) {
	market := types.Market{
		Symbol:        "SUIUSDT",
		BaseCurrency:  "SUI",
		QuoteCurrency: "USDT",
	}
	pos := types.NewPositionFromMarket(market)
	pos.AccumulatedProfit = fixedpoint.NewFromFloat(-1.25)

	x := NewPositionX(pos)
	x.UpdateProfit(fixedpoint.NewFromFloat(-3.54648), fixedpoint.NewFromFloat(-1.0099))

	require.Equal(t, "-1.25", pos.AccumulatedProfit.String())
	require.Equal(t, "-3.54648", x.AccumulatedProfit.String())
	require.Equal(t, "-1.0099", x.AccumulatedProfitValue.String())
}
