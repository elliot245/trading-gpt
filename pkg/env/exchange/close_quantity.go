package exchange

import "github.com/c9s/bbgo/pkg/fixedpoint"

// closeSizing is the resolved sizing decision for closing (part of) a position.
type closeSizing struct {
	// BaseQuantity is the absolute base-asset quantity to submit for the close
	// order. It is always <= the authoritative net position magnitude, so a
	// market close can never cross zero and open an unintended opposite position.
	BaseQuantity fixedpoint.Value
	// SkipDust is true when every available position signal (authoritative net
	// and internal base) is below the market minimum quantity. In that case no
	// closing order should be placed, because a reversing order for a dust
	// remainder is exactly the #89 oversell/flip bug.
	SkipDust bool
	// ReduceOnly, when true, requires the caller to submit the close as a
	// reduce-only order. It is set when the authoritative net read disagreed with
	// a non-dust internal base (#103): the authoritative read said flat/dust while
	// the fill-tracker still held a real position (observed: okex QueryPositionInfo
	// returned Base=0 for a live 63.43 long, so ClosePosition skipped the close and
	// falsely reported success). The close is then sized off the internal base so a
	// real position still gets closed, while reduce-only guarantees the order can
	// never open an opposite position if the internal base is the stale one.
	ReduceOnly bool
}

// resolveCloseBaseQuantity decides how much base asset to sell/buy to close a
// position, using the exchange-authoritative net position when it is available
// instead of the (potentially stale or gross) internally tracked base.
//
// Background (#89): handleCleanPosition queries the exchange for the real net
// position to evaluate TP/SL triggers, but ClosePosition then sized the close
// order off the internal ent.position base. When the internal base was larger
// than the exchange net (gross vs net divergence, or leftover dust from a prior
// round), the market close sold more than was actually held and flipped the
// account into an unintended opposite position (observed: a stop-loss close
// oversold and stayed short ~22h).
//
// The fix keeps direction handling in the caller and only governs magnitude:
//   - prefer the authoritative net magnitude when hasAuthoritative is true;
//   - never size larger than that magnitude (clamp defends against percentage>1);
//   - treat a sub-minimum remainder as dust and skip the order entirely.
func resolveCloseBaseQuantity(
	internalBase fixedpoint.Value,
	authoritativeNetBase fixedpoint.Value,
	hasAuthoritative bool,
	percentage fixedpoint.Value,
	minQuantity fixedpoint.Value,
) closeSizing {
	effective := internalBase
	// Default to reduce-only whenever the close is sized off the internal base
	// (no authoritative net available, or an authoritative net that is not a
	// credible non-dust reading). Reduce-only prevents a stale/gross internal base
	// from oversizing a market close and flipping the account into an opposite
	// position, which is the guarantee #89 needs; and it lets us still close a real
	// position when the authoritative read wrongly under-reports it as flat (#103,
	// observed: okex QueryPositionInfo returned Base=0 for a live 63.43 long).
	reduceOnly := true

	if hasAuthoritative && authoritativeNetBase.Abs().Compare(minQuantity) >= 0 {
		// Credible authoritative net (#89): size off the real net and rely on the
		// clamp below to bound the close, preserving the existing close path.
		effective = authoritativeNetBase
		reduceOnly = false
	}

	magnitude := effective.Abs()

	// Nothing (or only dust) actually held per every available signal.
	if magnitude.Sign() == 0 || magnitude.Compare(minQuantity) < 0 {
		return closeSizing{BaseQuantity: fixedpoint.Zero, SkipDust: true}
	}

	pct := percentage
	if pct.Sign() <= 0 {
		return closeSizing{BaseQuantity: fixedpoint.Zero, SkipDust: true}
	}
	if pct.Compare(fixedpoint.One) > 0 {
		pct = fixedpoint.One
	}

	qty := magnitude.Mul(pct)
	// Defensive clamp: a close order must never exceed the chosen magnitude.
	if qty.Compare(magnitude) > 0 {
		qty = magnitude
	}

	return closeSizing{BaseQuantity: qty, SkipDust: false, ReduceOnly: reduceOnly}
}
