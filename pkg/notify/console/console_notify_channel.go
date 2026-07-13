package console

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yubing744/trading-gpt/pkg/config"
	"github.com/yubing744/trading-gpt/pkg/types"
)

var log = logrus.WithField("notify", "console")

// ConsoleNotifyChannel is a notify channel for offline backtest/paper runs.
// It fills the role the Feishu channels play in live mode: the strategy
// creates its admin chat session on top of it, which is what drives the
// agent decision loop. Every reply is printed to stdout and, when
// configured, appended to a log file so decision streams of different runs
// (fixed vs unfixed build, prompt/model variants) can be diffed.
type ConsoleNotifyChannel struct {
	mu      sync.Mutex
	logFile *os.File
}

func NewConsoleNotifyChannel(cfg *config.NotifyConsoleConfig) (*ConsoleNotifyChannel, error) {
	ch := &ConsoleNotifyChannel{}

	if cfg != nil && cfg.LogFile != "" {
		if dir := filepath.Dir(cfg.LogFile); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create console log dir: %w", err)
			}
		}

		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open console log file: %w", err)
		}

		ch.logFile = f
		log.WithField("log_file", cfg.LogFile).Info("console notify channel logging to file")
	}

	return ch, nil
}

func (ch *ConsoleNotifyChannel) GetID() string {
	return "console"
}

func (ch *ConsoleNotifyChannel) Reply(ctx context.Context, msg *types.Message) error {
	if msg == nil {
		return nil
	}

	line := fmt.Sprintf("[console] %s | %s", time.Now().Format(time.RFC3339), strings.TrimRight(msg.Text, "\n"))

	ch.mu.Lock()
	defer ch.mu.Unlock()

	// stdout so it is visible even when backtest mode silences logrus
	fmt.Println(line)

	if ch.logFile != nil {
		if _, err := ch.logFile.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("write console log file: %w", err)
		}
	}

	return nil
}

// Close releases the underlying log file (best effort; the channel is used
// for the whole process lifetime in practice).
func (ch *ConsoleNotifyChannel) Close() error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.logFile != nil {
		err := ch.logFile.Close()
		ch.logFile = nil
		return err
	}

	return nil
}
