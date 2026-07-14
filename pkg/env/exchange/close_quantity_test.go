package exchange

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func fp(s string) fixedpoint.Value { return fixedpoint.MustNewFromString(s) }

func TestResolveCloseBaseQuantity(t *testing.T) {
	minQty := fp("0.01")

	cases := []struct {
		name           string
		internalBase   string
		authNetBase    string
		hasAuth        bool
		percentage     string
		wantQty        string
		wantSkipDust   bool
		wantReduceOnly bool
	}{
		{
			// #89 core: internal base is gross/stale and larger than the real
			// exchange net. Sizing off internal would oversell and flip short;
			// clamping to the authoritative net closes exactly what is held.
			name:         "long_internal_gross_exceeds_exchange_net",
			internalBase: "1.6046",
			authNetBase:  "0.8",
			hasAuth:      true,
			percentage:   "1",
			wantQty:      "0.8",
			wantSkipDust: false,
		},
		{
			// #103: authoritative read comes back flat (0) while the internal
			// fill-tracker still holds a real position. Previously this skipped the
			// close and falsely reported success, leaving a live position open. Now
			// we close off the internal base as a reduce-only order: if a real
			// position exists it gets closed; if the internal base is instead the
			// stale one, reduce-only makes the order a safe no-op (no short opened).
			name:           "long_internal_but_authoritative_reads_flat_closes_reduce_only",
			internalBase:   "0.8",
			authNetBase:    "0",
			hasAuth:        true,
			percentage:     "1",
			wantQty:        "0.8",
			wantSkipDust:   false,
			wantReduceOnly: true,
		},
		{
			// #103 live incident: okex QueryPositionInfo parsed a live 63.43 long as
			// Base=0. Must close the real position (reduce-only), not skip.
			name:           "incident_103_live_long_read_as_zero",
			internalBase:   "63.4300065",
			authNetBase:    "0",
			hasAuth:        true,
			percentage:     "1",
			wantQty:        "63.4300065",
			wantSkipDust:   false,
			wantReduceOnly: true,
		},
		{
			// Authoritative net is sub-minimum dust but internal holds a real
			// position -> under-reported read; close reduce-only rather than skip.
			name:           "exchange_net_dust_internal_real_closes_reduce_only",
			internalBase:   "0.8",
			authNetBase:    "0.004",
			hasAuth:        true,
			percentage:     "1",
			wantQty:        "0.8",
			wantSkipDust:   false,
			wantReduceOnly: true,
		},
		{
			// Both signals agree the position is dust/flat -> genuinely nothing to
			// close, skip (no reversing order).
			name:         "both_signals_dust_skips",
			internalBase: "0.005",
			authNetBase:  "0",
			hasAuth:      true,
			percentage:   "1",
			wantQty:      "0",
			wantSkipDust: true,
		},
		{
			name:         "short_net_clamped_to_magnitude",
			internalBase: "-2.0",
			authNetBase:  "-1.2",
			hasAuth:      true,
			percentage:   "1",
			wantQty:      "1.2",
			wantSkipDust: false,
		},
		{
			name:         "partial_close_half_of_net",
			internalBase: "1.0",
			authNetBase:  "1.0",
			hasAuth:      true,
			percentage:   "0.5",
			wantQty:      "0.5",
			wantSkipDust: false,
		},
		{
			// No authoritative source -> fall back to internal base, sized off it
			// and reduce-only (a failed query must not enable a stale-internal
			// oversell).
			name:           "no_authoritative_falls_back_to_internal",
			internalBase:   "0.8",
			authNetBase:    "0",
			hasAuth:        false,
			percentage:     "1",
			wantQty:        "0.8",
			wantSkipDust:   false,
			wantReduceOnly: true,
		},
		{
			// Defensive: percentage>1 must never exceed the net magnitude.
			name:         "percentage_over_one_clamped",
			internalBase: "1.0",
			authNetBase:  "1.0",
			hasAuth:      true,
			percentage:   "1.5",
			wantQty:      "1.0",
			wantSkipDust: false,
		},
		{
			name:         "nonpositive_percentage_skips",
			internalBase: "1.0",
			authNetBase:  "1.0",
			hasAuth:      true,
			percentage:   "0",
			wantQty:      "0",
			wantSkipDust: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCloseBaseQuantity(
				fp(tc.internalBase),
				fp(tc.authNetBase),
				tc.hasAuth,
				fp(tc.percentage),
				minQty,
			)
			if got.SkipDust != tc.wantSkipDust {
				t.Fatalf("SkipDust = %v, want %v", got.SkipDust, tc.wantSkipDust)
			}
			if got.BaseQuantity.Compare(fp(tc.wantQty)) != 0 {
				t.Fatalf("BaseQuantity = %v, want %v", got.BaseQuantity, tc.wantQty)
			}
			if got.ReduceOnly != tc.wantReduceOnly {
				t.Fatalf("ReduceOnly = %v, want %v", got.ReduceOnly, tc.wantReduceOnly)
			}
			// Invariant: when we trust the authoritative net (not the reduce-only
			// disagreement fallback), a resolved close never exceeds that net, so a
			// market close can never cross zero and flip. In the reduce-only case
			// the reduce-only flag provides that guarantee instead of the clamp.
			if !got.SkipDust && !got.ReduceOnly && tc.hasAuth {
				netMag := fp(tc.authNetBase).Abs()
				if got.BaseQuantity.Compare(netMag) > 0 {
					t.Fatalf("close qty %v exceeds authoritative net %v (would flip)", got.BaseQuantity, netMag)
				}
			}
		})
	}
}
