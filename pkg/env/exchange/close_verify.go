package exchange

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// Defaults for the verify-after-close invariant (#103 follow-up). A market
// close normally fills near-instantly; a few short polls cover exchange-side
// propagation without stalling the strategy loop.
const (
	defaultCloseVerifyAttempts = 4
	defaultCloseVerifyInterval = 3 * time.Second
)

// closeConfirmedFlat reports whether an exchange-authoritative net base reading
// confirms the position is closed: anything below the market minimum quantity
// is dust that cannot be closed further and counts as flat.
func closeConfirmedFlat(netBase, minQuantity fixedpoint.Value) bool {
	return netBase.Abs().Compare(minQuantity) < 0
}

// verifyClosedWithRetry polls an authoritative net-base query until the
// position is confirmed flat, retrying on query failure and on still-open
// readings. It returns nil only on a positive flat confirmation — the #103
// lesson is that a close must never be reported successful on assumption; a
// notification is not a fact. queryNetBase follows the queryAuthoritativeNetBase
// contract: (netBase, true) on a successful read, (_, false) on query failure.
func verifyClosedWithRetry(
	ctx context.Context,
	queryNetBase func(ctx context.Context) (fixedpoint.Value, bool),
	minQuantity fixedpoint.Value,
	attempts int,
	interval time.Duration,
) error {
	if attempts <= 0 {
		attempts = defaultCloseVerifyAttempts
	}

	var lastNet fixedpoint.Value
	lastRead := false

	for i := 0; i < attempts; i++ {
		if i > 0 && interval > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("close verification aborted: %w", ctx.Err())
			case <-time.After(interval):
			}
		}

		net, ok := queryNetBase(ctx)
		if !ok {
			continue // query failure: retry, never assume flat
		}

		lastNet, lastRead = net, true
		if closeConfirmedFlat(net, minQuantity) {
			return nil
		}
	}

	if !lastRead {
		return fmt.Errorf("close NOT confirmed: authoritative position query failed %d times", attempts)
	}
	return fmt.Errorf("close NOT confirmed: net position still %v after %d checks", lastNet, attempts)
}
