package event

import (
	"context"
	"testing"
	"time"

	"github.com/awantoch/beemflow/config"
)

func TestNewInProcEventBus(t *testing.T) {
	b := NewInProcEventBus()
	if b == nil {
		t.Fatal("expected NewInProcEventBus not nil")
	}
}

func TestPublishSubscribeNoop(t *testing.T) {
	b := NewInProcEventBus()
	if err := b.Publish("topic", 123); err != nil {
		t.Errorf("Publish error: %v", err)
	}
	// Subscribe should not panic
	success := true
	func() {
		defer func() {
			if r := recover(); r != nil {
				success = false
			}
		}()
		b.Subscribe(context.Background(), "topic", func(payload any) {})
	}()
	if !success {
		t.Errorf("Subscribe panicked")
	}
}

func TestEventBus_RoundTrip(t *testing.T) {
	b := NewInProcEventBus()
	received := make(chan any, 1)
	b.Subscribe(context.Background(), "topic", func(payload any) {
		received <- payload
	})
	if err := b.Publish("topic", 42); err != nil {
		t.Errorf("Publish error: %v", err)
	}
	select {
	case v := <-received:
		if v != 42 {
			t.Errorf("expected 42, got %v", v)
		}
	case <-time.After(time.Second):
		t.Errorf("handler did not receive payload")
	}
}

func TestNewEventBusFromConfig_Memory(t *testing.T) {
	bus, err := NewEventBusFromConfig(nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if bus == nil {
		t.Fatal("expected non-nil bus")
	}
}

func TestNewEventBusFromConfig_NATS(t *testing.T) {
	cfg := &config.EventConfig{Driver: "nats", URL: "nats://localhost:4222"}
	bus, err := NewEventBusFromConfig(cfg)
	if err != nil {
		t.Skipf("NATS not available or error: %v", err)
	}
	if bus == nil {
		t.Skip("NATS bus not created")
	}
	// Optionally: publish/subscribe round-trip if NATS is running
}

func TestNewEventBusFromConfig_Unknown(t *testing.T) {
	cfg := &config.EventConfig{Driver: "foo"}
	_, err := NewEventBusFromConfig(cfg)
	if err == nil {
		t.Error("expected error for unknown driver")
	}
}
