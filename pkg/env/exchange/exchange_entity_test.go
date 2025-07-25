package exchange

import (
	"sync"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	ttypes "github.com/yubing744/trading-gpt/pkg/types"
	"github.com/stretchr/testify/assert"
)

// MockEventEmitter implements EventEmitter for testing
type MockEventEmitter struct {
	events []ttypes.IEvent
	mu     sync.Mutex
}

func (m *MockEventEmitter) Emit(event ttypes.IEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *MockEventEmitter) GetEvents() []ttypes.IEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.events
}

func TestPositionCloseEvent(t *testing.T) {
	// Create position closed event data
	positionData := PositionClosedEventData{
		StrategyID:    "test_strategy",
		Symbol:        "BTCUSDT",
		EntryPrice:    50000.0,
		ExitPrice:     51000.0,
		Quantity:      1.0,
		ProfitAndLoss: 1000.0,
		CloseReason:   "TakeProfit",
		Timestamp:     time.Now(),
	}

	// Create the event
	event := NewPositionClosedEvent(positionData)

	// Verify event properties
	assert.Equal(t, EventPositionClosed, event.GetType())
	
	data := event.GetData()
	positionClosedData, ok := data.(PositionClosedEventData)
	assert.True(t, ok)
	
	assert.Equal(t, "BTCUSDT", positionClosedData.Symbol)
	assert.Equal(t, 50000.0, positionClosedData.EntryPrice)
	assert.Equal(t, 51000.0, positionClosedData.ExitPrice)
	assert.Equal(t, 1.0, positionClosedData.Quantity)
	assert.Equal(t, 1000.0, positionClosedData.ProfitAndLoss)
	assert.Equal(t, "TakeProfit", positionClosedData.CloseReason)
}

func TestPositionClosedEventDataToPrompts(t *testing.T) {
	// Test with profit
	positionData := PositionClosedEventData{
		StrategyID:    "test_strategy",
		Symbol:        "BTCUSDT",
		EntryPrice:    50000.0,
		ExitPrice:     51000.0,
		Quantity:      1.0,
		ProfitAndLoss: 1000.0,
		CloseReason:   "TakeProfit",
		Timestamp:     time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	prompts := positionData.ToPrompts()
	assert.Len(t, prompts, 1)
	assert.Contains(t, prompts[0], "Position closed for BTCUSDT:")
	assert.Contains(t, prompts[0], "profit: 1000.00")
	assert.Contains(t, prompts[0], "Close Reason: TakeProfit")

	// Test with loss
	positionDataLoss := PositionClosedEventData{
		StrategyID:    "test_strategy",
		Symbol:        "BTCUSDT",
		EntryPrice:    50000.0,
		ExitPrice:     49000.0,
		Quantity:      1.0,
		ProfitAndLoss: -1000.0,
		CloseReason:   "StopLoss",
		Timestamp:     time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	promptsLoss := positionDataLoss.ToPrompts()
	assert.Len(t, promptsLoss, 1)
	assert.Contains(t, promptsLoss[0], "Position closed for BTCUSDT:")
	assert.Contains(t, promptsLoss[0], "loss: -1000.00")
	assert.Contains(t, promptsLoss[0], "Close Reason: StopLoss")
}

// TestEventEmitter tests that our event emitter works correctly
func TestEventEmitter(t *testing.T) {
	// Create a mock event emitter
	emitter := &MockEventEmitter{
		events: make([]ttypes.IEvent, 0),
	}

	// Create position closed event data
	positionData := PositionClosedEventData{
		StrategyID:    "test_strategy",
		Symbol:        "BTCUSDT",
		EntryPrice:    50000.0,
		ExitPrice:     51000.0,
		Quantity:      1.0,
		ProfitAndLoss: 1000.0,
		CloseReason:   CloseReasonManual,
		Timestamp:     time.Now(),
	}

	// Emit the event through our mock emitter
	event := NewPositionClosedEvent(positionData)
	emitter.Emit(event)

	// Verify the event was emitted correctly
	events := emitter.GetEvents()
	assert.Len(t, events, 1)

	emittedEvent := events[0]
	assert.Equal(t, EventPositionClosed, emittedEvent.GetType())

	data := emittedEvent.GetData()
	positionClosedData, ok := data.(PositionClosedEventData)
	assert.True(t, ok)
	
	assert.Equal(t, "BTCUSDT", positionClosedData.Symbol)
	assert.Equal(t, 50000.0, positionClosedData.EntryPrice)
	assert.Equal(t, 51000.0, positionClosedData.ExitPrice)
	assert.Equal(t, 1.0, positionClosedData.Quantity)
	assert.Equal(t, 1000.0, positionClosedData.ProfitAndLoss)
	assert.Equal(t, CloseReasonManual, positionClosedData.CloseReason)
}

// TestCreatePositionClosedEvent tests the createPositionClosedEvent method
func TestCreatePositionClosedEvent(t *testing.T) {
	// Create a minimal ExchangeEntity for testing
	entity := &ExchangeEntity{
		symbol: "BTCUSDT",
		position: &PositionX{
			Position: &types.Position{
				Symbol:      "BTCUSDT",
				Base:        fixedpoint.NewFromFloat(1.0),
				AverageCost: fixedpoint.NewFromFloat(50000.0),
			},
		},
	}

	// Set accumulated profit
	entity.position.AccumulatedProfit = fixedpoint.NewFromFloat(1000.0)

	// Create a KLineWindow with one kline for exit price
	kLineWindow := &types.KLineWindow{}
	kLineWindow.Add(types.KLine{
		Symbol: "BTCUSDT",
		Close:  fixedpoint.NewFromFloat(51000.0),
	})
	entity.KLineWindow = kLineWindow

	// Create a position
	position := &types.Position{
		Symbol:             "BTCUSDT",
		Base:               fixedpoint.NewFromFloat(1.0),
		AverageCost:        fixedpoint.NewFromFloat(50000.0),
		StrategyInstanceID: "test_strategy",
	}

	// Call createPositionClosedEvent method
	event := entity.createPositionClosedEvent(position, CloseReasonTakeProfit)

	// Verify the event
	assert.Equal(t, EventPositionClosed, event.GetType())
	
	data := event.GetData()
	positionClosedData, ok := data.(PositionClosedEventData)
	assert.True(t, ok)
	
	assert.Equal(t, "BTCUSDT", positionClosedData.Symbol)
	assert.Equal(t, 50000.0, positionClosedData.EntryPrice)
	assert.Equal(t, 51000.0, positionClosedData.ExitPrice)
	assert.Equal(t, 1.0, positionClosedData.Quantity)
	assert.Equal(t, 1000.0, positionClosedData.ProfitAndLoss)
	assert.Equal(t, CloseReasonTakeProfit, positionClosedData.CloseReason)
	assert.Equal(t, "test_strategy", positionClosedData.StrategyID)
	
	// Check that timestamp is set
	assert.NotZero(t, positionClosedData.Timestamp)
}