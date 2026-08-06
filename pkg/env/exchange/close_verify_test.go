package exchange

import (
	"context"
	"errors"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestCloseConfirmedFlat(t *testing.T) {
	minQty := fp("1")
	if !closeConfirmedFlat(fp("0"), minQty) {
		t.Fatal("zero net must confirm flat")
	}
	if !closeConfirmedFlat(fp("0.5"), minQty) {
		t.Fatal("sub-minimum dust must confirm flat")
	}
	if !closeConfirmedFlat(fp("-0.5"), minQty) {
		t.Fatal("negative dust must confirm flat")
	}
	if closeConfirmedFlat(fp("63.4300065"), minQty) {
		t.Fatal("a live position must never confirm flat")
	}
}

func TestVerifyClosedWithRetry(t *testing.T) {
	ctx := context.Background()
	minQty := fp("1")

	t.Run("confirmed on first read", func(t *testing.T) {
		calls := 0
		err := verifyClosedWithRetry(ctx, func(context.Context) (fixedpoint.Value, bool) {
			calls++
			return fp("0"), true
		}, minQty, 4, 0)
		if err != nil || calls != 1 {
			t.Fatalf("want immediate confirm (1 call), got err=%v calls=%d", err, calls)
		}
	})

	t.Run("confirmed after fill propagation", func(t *testing.T) {
		reads := []struct {
			net fixedpoint.Value
			ok  bool
		}{{fp("63.43"), true}, {fp("63.43"), true}, {fp("0"), true}}
		i := 0
		err := verifyClosedWithRetry(ctx, func(context.Context) (fixedpoint.Value, bool) {
			r := reads[i]
			i++
			return r.net, r.ok
		}, minQty, 4, 0)
		if err != nil {
			t.Fatalf("want confirm after retries, got %v", err)
		}
	})

	t.Run("still open after all attempts errors", func(t *testing.T) {
		err := verifyClosedWithRetry(ctx, func(context.Context) (fixedpoint.Value, bool) {
			return fp("63.43"), true
		}, minQty, 3, 0)
		if err == nil {
			t.Fatal("an unclosed position must not verify")
		}
	})

	t.Run("query failures never assume flat", func(t *testing.T) {
		err := verifyClosedWithRetry(ctx, func(context.Context) (fixedpoint.Value, bool) {
			return fp("0"), false
		}, minQty, 3, 0)
		if err == nil {
			t.Fatal("all-failed queries must not confirm a close")
		}
	})

	t.Run("query failure then flat read confirms", func(t *testing.T) {
		i := 0
		err := verifyClosedWithRetry(ctx, func(context.Context) (fixedpoint.Value, bool) {
			i++
			if i == 1 {
				return fp("0"), false
			}
			return fp("0"), true
		}, minQty, 3, 0)
		if err != nil {
			t.Fatalf("want confirm on second read, got %v", err)
		}
	})

	t.Run("cancelled context aborts", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		err := verifyClosedWithRetry(cctx, func(context.Context) (fixedpoint.Value, bool) {
			return fp("63.43"), true // never confirms, forcing a wait
		}, minQty, 3, 1)
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	})
}
