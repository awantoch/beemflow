package event

type EventBus interface {
	Publish(topic string, payload any) error
	Subscribe(topic string, handler func(payload any))
}

// REFACTOR: Consider replacing sync.Mutex with more scalable concurrency primitives if high concurrency is expected.
// InProcEventBus is the default in-memory event bus. For production/distributed use, inject a custom EventBus implementation.

// NewInProcEventBus returns a new in-memory event bus. For production, inject a custom EventBus.
func NewInProcEventBus() *WatermillEventBus {
	return NewWatermillInMemBus()
}

// REFACTOR: Consider making InProcEventBus pluggable or replaceable for distributed or production use cases.
