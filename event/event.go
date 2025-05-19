package event

import (
	"sync"

	"github.com/awantoch/beemflow/logger"
)

type EventBus interface {
	Publish(topic string, payload any) error
	Subscribe(topic string, handler func(payload any))
}

// REFACTOR: Consider replacing sync.Mutex with more scalable concurrency primitives if high concurrency is expected.
// InProcEventBus is the default in-memory event bus. For production/distributed use, inject a custom EventBus implementation.
type InProcEventBus struct {
	handlers map[string][]func(payload any)
	mu       sync.RWMutex // Use RWMutex for improved concurrent read access. For context-aware cancellation, consider errgroup or context-aware primitives in the future.
}

// NewInProcEventBus returns a new in-memory event bus. For production, inject a custom EventBus.
func NewInProcEventBus() *InProcEventBus {
	return &InProcEventBus{handlers: make(map[string][]func(payload any))}
}

func (b *InProcEventBus) Publish(topic string, payload any) error {
	logger.Debug("[EVENT BUS] Publish called for topic %s with payload: %+v", topic, payload)
	b.mu.RLock()
	handlers := b.handlers[topic]
	b.mu.RUnlock()
	for _, h := range handlers {
		logger.Debug("[EVENT BUS] Invoking handler for topic %s", topic)
		h(payload)
	}
	return nil
}

func (b *InProcEventBus) Subscribe(topic string, handler func(payload any)) {
	logger.Debug("[EVENT BUS] Subscribe called for topic %s", topic)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
}

// REFACTOR: Consider making InProcEventBus pluggable or replaceable for distributed or production use cases.
