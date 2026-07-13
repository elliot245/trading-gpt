# Offline Backtest Harness (KR4 paper verification)

A runnable, offline replay harness that drives the full `jarvis` strategy
decision loop against historical OKX SUIUSDT klines using bbgo's built-in
`backtest` mode — **no live trading, no exchange credentials, no LLM tokens
required**. It exists so that:

- fixed vs unfixed builds can be compared on identical market data, and
- future prompt/model variants can be A/B evaluated behind the same agent
  seam (`agents.IAgent`).

## Architecture

```
                             bbgo backtest engine
 sqlite (okex_klines) ──► kline feed ──► matching engine (spot)
                                │                ▲
                                ▼                │ market orders
                     ExchangeEntity (unchanged decision loop)
                                │ events (kline/indicator/position/update_finish)
                                ▼
                     jarvis Strategy.agentAction
                                │
              ┌─────────────────┴──────────────────┐
              ▼ (backtest)                          ▼ (live / LLM A/B)
   scripted agent (fixture replay)        trading agent (LLM via llms.Model)
              │
              ▼
   console notify channel ──► stdout + data/backtest-decisions.log
```

Three seams were added, all config-selectable and inert in live configs:

| Seam | Config | Code |
|------|--------|------|
| Agent provider | `agent.scripted.{enabled,fixture_file}` | `pkg/agents/scripted` |
| Admin chat channel | `notify.console.{enabled,log_file}` | `pkg/notify/console` |
| Order sizing | `env.exchange.backtest_spot_sizing` | `pkg/env/exchange/exchange_entity.go` |

## How to run

Prereqs: Go (`GOTOOLCHAIN=go1.22.0`), network access to OKX **public** market
data endpoints for the initial kline download only. Afterwards runs are fully
offline.

```bash
export GOTOOLCHAIN=go1.22.0
export DB_DRIVER=sqlite3
export DB_DSN=data/backtest.sqlite3

# 1. create the schema (one-off; also syncs a partial seed of klines)
go run ./main.go backtest --config config/backtest-scripted.yaml --sync --sync-only --sync-from 2026-07-03

# 2. backfill complete klines (the vendored bbgo okex sync pages incorrectly, see caveats)
go run ./tools/import-okx-klines -dsn data/backtest.sqlite3 -symbol SUIUSDT \
    -intervals 1m,5m,15m,1h,1d -from 2026-07-03 -to 2026-07-12

# 3. run the backtest (repeatable, offline)
go run ./main.go backtest --config config/backtest-scripted.yaml
```

Outputs:

- stdout: decision stream (`[console] ...` lines) + bbgo `BACK-TEST REPORT`
  (trades, PnL, Sharpe/Sortino),
- `data/backtest-decisions.log`: full decision/message stream for diffing
  between runs or build variants.

The backtest window and account balances live under `backtest:` in
`config/backtest-scripted.yaml`; scripted decisions live in
`fixtures/backtest/scripted_decisions.json` (`at` = RFC3339 activation time;
each decision fires once, on the first closed kline at/after `at`;
otherwise the agent emits `no_action`).

### A/B evaluating prompts/models with a real LLM

Because the strategy still goes through the standard `agents.IAgent` seam, an
LLM-backed backtest only needs a config variant: remove `agent.scripted`,
enable `agent.trading` + an `llm:` section (and its `LLM_*_TOKEN` env vars) in
the backtest config. Decisions will then come from the real model while
orders still go to the offline matching engine. Compare runs via
`data/backtest-decisions.log` and the backtest report. (Mind determinism:
LLM outputs vary run-to-run; the scripted agent is the deterministic control.)

## What works

- Full strategy loop end-to-end: kline events → indicators (BOLL/VR) →
  position events → prompt assembly → agent decision → command dispatch →
  order execution in the matching engine → position/PnL tracking → trade
  report.
- `open_long_position`, `open_short_position`, `close_position`, `no_action`
  with `%`-style SL/TP args; risk-gate and order-dedup code paths run
  unmodified.
- Shorting: emulated on the spot matching engine by pre-funding the backtest
  account with base currency (`SUI: 20000`); position tracking goes
  negative-base = short and PnL math is unchanged.
- Reproducibility: the scripted decision stream is deterministic
  (verified across runs).

## What is mocked / simplified

- **Agent**: fixture replay instead of an LLM (LLM variant available by
  config, see above).
- **Notify/chat**: console channel instead of Feishu; Feishu/Twitter/Coze/FNG
  entities disabled in the backtest config.
- **Exchange**: bbgo's simple price matching (taker fills at last close),
  spot-only:
  - no margin/leverage — `leverage: 3` degrades to spot sizing (position
    sizes differ from live),
  - attached TP/SL trigger prices (`StopPrice`/`TakePrice` on the order) are
    **not** simulated; exits happen only via scripted `close_position`
    decisions. `clean_position` polling is disabled because the backtest
    exchange does not implement `ExchangePositionUpdateService`.
- Memory/reflection subsystems disabled for determinism.

## Caveats

1. **±1-bar fill jitter**: the strategy's event loop is asynchronous w.r.t.
   the backtest kline dispatcher, so an order submitted after kline *N* may
   fill at the price of kline *N* or *N+1* (1m feed). The **decision stream is
   deterministic**, but realized PnL can vary slightly between runs. Treat
   PnL as indicative; compare behaviour via the decision log. A synchronous
   dispatch mode is the main follow-up.
2. **Vendored bbgo okex kline sync is broken for paging** (passes
   second-precision cursors to OKX's millisecond `history-candles` API), so
   `--sync` only lands the most recent ~100 rows per interval. Use
   `tools/import-okx-klines` (correct paging, public endpoints only) after
   the initial `--sync-only` schema creation.
3. **rockhopper sqlite re-upgrade bug**: with the pinned rockhopper
   (<= v2.0.7), a second `Upgrade()` against an already-migrated sqlite DB
   fails ("expected 2 destination arguments in Scan, not 3"). Patched
   defensively in vendored bbgo (`libs/bbgo/pkg/service/database.go`):
   the error is ignored iff the recorded migration head is already current.
   Root fix is rockhopper v2.0.8, which needs go >= 1.25.
4. **`accumulated profit: +Inf%`** appears on the first kline after an open
   (division by a not-yet-set average cost). Pre-existing live behaviour,
   cosmetic in backtests.
5. OKX kline history is fetched from public endpoints; Binance is not usable
   as a source from this region (HTTP 451-style geo block), which is fine —
   OKX data matches the live venue anyway.
6. `backtest_spot_sizing` must **never** be enabled in a live OKX config; it
   deliberately inverts the OKX quote-sizing convention for market buys.

## What remains for OKX-demo paper mode (not in scope here)

This harness is the *backtest* form of KR4. The demo-trading (paper) form —
live OKX demo endpoint (`x-simulated-trading: 1`) with demo API keys — still
needs:

- a session/exchange flag to route the vendored okex driver at the demo
  environment,
- real attached TP/SL behaviour verification (demo venue supports it),
- the LLM agent enabled with the production prompt, and
- longer-horizon soak (the backtest harness compresses days into seconds).
