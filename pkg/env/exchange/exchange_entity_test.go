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

// MockEventEmitter implements a simple event emitter for testing
type MockEventEmitter struct {
	events []interface{}
	mu     sync.Mutex
}

func (m *MockEventEmitter) emit(event interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *MockEventEmitter) getEvents() []interface{} {
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

func TestPositionCloseEventEmission(t *testing.T) {
	// Create a mock event emitter
	emitter := &MockEventEmitter{
		events: make([]interface{}, 0),
	}

	// Create a test position
	position := &types.Position{
		Symbol:      "BTCUSDT",
		Base:        fixedpoint.NewFromFloat(1.0),
		AverageCost: fixedpoint.NewFromFloat(50000.0),
	}

	// Create position X wrapper
	positionX := NewPositionX(position)
	positionX.AccumulatedProfit = fixedpoint.NewFromFloat(1000.0)

	// Create a KLineWindow with one kline for exit price
	kLineWindow := &types.KLineWindow{}
	kLineWindow.Add(types.KLine{
		Symbol: "BTCUSDT",
		Close:  fixedpoint.NewFromFloat(51000.0),
	})

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
	emitter.emit(event)

	// Verify the event was emitted correctly
	events := emitter.getEvents()
	assert.Len(t, events, 1)

	emittedEvent, ok := events[0].(*ttypes.Event)
	assert.True(t, ok)
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

// TestEventFromTradeCollector tests that the event is properly created when position is closed
// This simulates the OnPositionUpdate callback in the Run method
func TestEventFromTradeCollector(t *testing.T) {
	// Create a channel to receive events like in the real implementation
	eventCh := make(chan ttypes.IEvent, 10)

	// Create position closed event data similar to what would be created in the OnPositionUpdate callback
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

	// Create and emit the event like in the real implementation
	go func() {
		event := NewPositionClosedEvent(positionData)
		eventCh <- event
	}()

	// Check that an event was sent to the channel
	select {
	case event := <-eventCh:
		// Verify it's a position closed event
		assert.Equal(t, EventPositionClosed, event.GetType())
		
		data := event.GetData()
		if positionData, ok := data.(PositionClosedEventData); ok {
			assert.Equal(t, "BTCUSDT", positionData.Symbol)
			assert.Equal(t, 50000.0, positionData.EntryPrice)
			assert.Equal(t, 51000.0, positionData.ExitPrice)
			assert.Equal(t, CloseReasonManual, positionData.CloseReason)
		} else {
			t.Errorf("Event data is not PositionClosedEventData, got %T", data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("No event received within timeout")
	}
}