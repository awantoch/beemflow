package event

import (
	"sync"

	"github.com/awantoch/beemflow/pkg/logger"
)

type EventBus interface {
	Publish(topic string, payload any) error
	Subscribe(topic string, handler func(payload any))
}

type InProcEventBus struct {
	handlers map[string][]func(payload any)
	mu       sync.Mutex
}

func NewInProcEventBus() *InProcEventBus {
	return &InProcEventBus{handlers: make(map[string][]func(payload any))}
}

func (b *InProcEventBus) Publish(topic string, payload any) error {
	logger.Debug("[EVENT BUS] Publish called for topic %s with payload: %+v", topic, payload)
	b.mu.Lock()
	handlers := b.handlers[topic]
	b.mu.Unlock()
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

func getenv(key string) string {
	return func() string {
		return "" // replaced at build time or by go:generate if needed
	}()
}
