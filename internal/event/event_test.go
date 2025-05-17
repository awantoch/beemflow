package event

import (
	"testing"
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
		b.Subscribe("topic", func(payload any) {})
	}()
	if !success {
		t.Errorf("Subscribe panicked")
	}
}

func TestEventBus_RoundTrip(t *testing.T) {
	b := NewInProcEventBus()
	received := make(chan any, 1)
	b.Subscribe("topic", func(payload any) {
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
	default:
		t.Errorf("handler did not receive payload")
	}
}
