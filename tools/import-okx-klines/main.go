// Command import-okx-klines backfills OKX public klines into the local
// backtest sqlite database (okex_klines table) used by `bbgo backtest`.
//
// Why this exists: the vendored bbgo okex batch sync pages the OKX
// history-candles endpoint with second-precision cursors while OKX expects
// millisecond timestamps, so `backtest --sync` only ever lands the most
// recent ~100 rows per interval. This tool pages the public REST endpoint
// correctly and writes rows through bbgo's own BacktestService so the row
// format matches exactly what the backtest feed expects.
//
// It only calls PUBLIC market-data endpoints; no credentials are used.
//
// Usage:
//
//	DB_DSN=data/backtest.sqlite3 go run ./tools/import-okx-klines \
//	    -symbol SUIUSDT -intervals 1m,5m,15m,1h,1d \
//	    -from 2026-07-03 -to 2026-07-12
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/okex"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

const (
	okxBaseURL    = "https://www.okx.com"
	pageLimit     = 100
	requestPause  = 150 * time.Millisecond
	insertChunk   = 500
	klineEndSkewM = time.Millisecond
)

type okxCandleResponse struct {
	Code string     `json:"code"`
	Msg  string     `json:"msg"`
	Data [][]string `json:"data"`
}

func main() {
	var (
		dsn       = flag.String("dsn", os.Getenv("DB_DSN"), "sqlite3 DSN of the backtest database (default: env DB_DSN)")
		symbol    = flag.String("symbol", "SUIUSDT", "bbgo symbol, e.g. SUIUSDT")
		instID    = flag.String("inst-id", "", "OKX instId; derived from symbol when empty (SUIUSDT -> SUI-USDT)")
		intervals = flag.String("intervals", "1m,5m,15m,1h,1d", "comma-separated bbgo intervals to import")
		fromStr   = flag.String("from", "", "start date (YYYY-MM-DD, UTC), inclusive")
		toStr     = flag.String("to", "", "end date (YYYY-MM-DD, UTC), inclusive")
	)
	flag.Parse()

	if *dsn == "" || *fromStr == "" || *toStr == "" {
		log.Fatal("-dsn (or DB_DSN), -from and -to are required")
	}

	from, err := time.ParseInLocation("2006-01-02", *fromStr, time.UTC)
	if err != nil {
		log.WithError(err).Fatal("invalid -from")
	}

	to, err := time.ParseInLocation("2006-01-02", *toStr, time.UTC)
	if err != nil {
		log.WithError(err).Fatal("invalid -to")
	}
	// make the end date inclusive
	to = to.Add(24*time.Hour - time.Millisecond)

	if *instID == "" {
		derived, err := deriveInstID(*symbol)
		if err != nil {
			log.WithError(err).Fatal("cannot derive instId; pass -inst-id explicitly")
		}
		*instID = derived
	}

	ctx := context.Background()

	dbService := service.NewDatabaseService("sqlite3", *dsn)
	if err := dbService.Connect(); err != nil {
		log.WithError(err).Fatal("connect database")
	}
	defer dbService.Close()

	// Best effort: `bbgo backtest --sync` normally creates the schema. Running
	// Upgrade twice can fail on rockhopper version-table quirks, so tolerate
	// the error as long as the kline table exists.
	if err := dbService.Upgrade(ctx); err != nil {
		var n int
		checkErr := dbService.DB.Get(&n, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='okex_klines'")
		if checkErr != nil || n == 0 {
			log.WithError(err).Fatal("migrate database (and okex_klines table is missing; run `bbgo backtest --sync` once to create the schema)")
		}
		log.WithError(err).Warn("skip migration error; okex_klines table already exists")
	}

	backtestService := &service.BacktestService{DB: dbService.DB}

	okexExchange, err := exchange.NewPublic(types.ExchangeOKEx)
	if err != nil {
		log.WithError(err).Fatal("create okex public exchange")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, intervalStr := range strings.Split(*intervals, ",") {
		interval := types.Interval(strings.TrimSpace(intervalStr))
		bar, ok := okex.ToLocalInterval[interval]
		if !ok {
			log.Fatalf("interval %s is not supported by okex", interval)
		}

		// idempotent re-import: wipe this symbol+interval first
		if _, err := dbService.DB.Exec(
			"DELETE FROM okex_klines WHERE symbol = ? AND `interval` = ?", *symbol, interval.String()); err != nil {
			log.WithError(err).Fatal("delete existing klines")
		}

		klines, err := fetchRange(ctx, client, *instID, *symbol, interval, bar, from, to)
		if err != nil {
			log.WithError(err).Fatalf("fetch %s klines", interval)
		}

		for start := 0; start < len(klines); start += insertChunk {
			end := start + insertChunk
			if end > len(klines) {
				end = len(klines)
			}
			if err := backtestService.BatchInsert(klines[start:end], okexExchange); err != nil {
				log.WithError(err).Fatalf("insert %s klines", interval)
			}
		}

		var first, last string
		if len(klines) > 0 {
			first = klines[0].StartTime.Time().UTC().Format(time.RFC3339)
			last = klines[len(klines)-1].StartTime.Time().UTC().Format(time.RFC3339)
		}
		log.Infof("imported %d %s klines for %s (%s .. %s)", len(klines), interval, *symbol, first, last)
	}
}

// deriveInstID converts a bbgo symbol like SUIUSDT into an OKX instId like
// SUI-USDT for the common quote currencies.
func deriveInstID(symbol string) (string, error) {
	for _, quote := range []string{"USDT", "USDC", "USD", "BTC", "ETH"} {
		if strings.HasSuffix(symbol, quote) && len(symbol) > len(quote) {
			return symbol[:len(symbol)-len(quote)] + "-" + quote, nil
		}
	}
	return "", fmt.Errorf("cannot derive instId from symbol %s", symbol)
}

// fetchRange pages the OKX history-candles endpoint backwards from `to`
// until `from` and returns confirmed klines in ascending start-time order.
func fetchRange(
	ctx context.Context, client *http.Client,
	instID, symbol string, interval types.Interval, bar string,
	from, to time.Time,
) ([]types.KLine, error) {
	var klines []types.KLine

	// OKX `after` returns records with ts strictly earlier than the cursor.
	cursor := to.UnixMilli() + 1

	for {
		candles, err := fetchPage(ctx, client, instID, bar, cursor)
		if err != nil {
			return nil, err
		}

		if len(candles) == 0 {
			break
		}

		done := false
		for _, c := range candles { // newest -> oldest
			ts, o, h, l, cl, vol, volCcy, confirm, err := parseCandle(c)
			if err != nil {
				return nil, err
			}

			if ts.Before(from) {
				done = true
				break
			}

			if ts.After(to) || confirm != "1" {
				continue // unconfirmed (still-open) candle
			}

			klines = append(klines, types.KLine{
				Exchange:                 types.ExchangeOKEx,
				Symbol:                   symbol,
				StartTime:                types.Time(ts),
				EndTime:                  types.Time(ts.Add(interval.Duration() - klineEndSkewM)),
				Interval:                 interval,
				Open:                     o,
				High:                     h,
				Low:                      l,
				Close:                    cl,
				Volume:                   vol,
				QuoteVolume:              volCcy,
				TakerBuyBaseAssetVolume:  fixedpoint.Zero,
				TakerBuyQuoteAssetVolume: fixedpoint.Zero,
				Closed:                   true,
			})

			cursor = ts.UnixMilli()
		}

		if done || len(candles) < pageLimit {
			break
		}

		time.Sleep(requestPause)
	}

	// ascending order for insertion/readability
	for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 {
		klines[i], klines[j] = klines[j], klines[i]
	}

	return klines, nil
}

func fetchPage(ctx context.Context, client *http.Client, instID, bar string, afterMilli int64) ([][]string, error) {
	q := url.Values{}
	q.Set("instId", instID)
	q.Set("bar", bar)
	q.Set("limit", fmt.Sprintf("%d", pageLimit))
	q.Set("after", fmt.Sprintf("%d", afterMilli))

	reqURL := okxBaseURL + "/api/v5/market/history-candles?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("okx candles request failed: status %d", resp.StatusCode)
	}

	var body okxCandleResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	if body.Code != "0" {
		return nil, fmt.Errorf("okx candles request failed: code=%s msg=%s", body.Code, body.Msg)
	}

	return body.Data, nil
}

// parseCandle parses one OKX candle row:
// [ts, open, high, low, close, vol(base), volCcy(quote), volCcyQuote, confirm]
func parseCandle(c []string) (
	ts time.Time,
	open, high, low, closePrice, volume, quoteVolume fixedpoint.Value,
	confirm string,
	err error,
) {
	if len(c) < 9 {
		err = fmt.Errorf("unexpected candle field count %d: %v", len(c), c)
		return
	}

	var milli int64
	if _, scanErr := fmt.Sscanf(c[0], "%d", &milli); scanErr != nil {
		err = fmt.Errorf("parse candle ts %q: %w", c[0], scanErr)
		return
	}
	ts = time.UnixMilli(milli).UTC()

	fields := []*fixedpoint.Value{&open, &high, &low, &closePrice, &volume, &quoteVolume}
	for i, target := range fields {
		v, parseErr := fixedpoint.NewFromString(c[i+1])
		if parseErr != nil {
			err = fmt.Errorf("parse candle field #%d %q: %w", i+1, c[i+1], parseErr)
			return
		}
		*target = v
	}

	confirm = c[8]
	return
}
