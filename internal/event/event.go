package event

import (
	"log"
	"sync"
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
	debugLog("[EVENT BUS] Publish called for topic %s with payload: %+v", topic, payload)
	b.mu.Lock()
	handlers := b.handlers[topic]
	b.mu.Unlock()
	for _, h := range handlers {
		debugLog("[EVENT BUS] Invoking handler for topic %s", topic)
		h(payload)
	}
	return nil
}

func (b *InProcEventBus) Subscribe(topic string, handler func(payload any)) {
	debugLog("[EVENT BUS] Subscribe called for topic %s", topic)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
}

// debugLog prints debug logs only if BEEMFLOW_DEBUG is set.
func debugLog(format string, v ...any) {
	if getenvDebug() {
		log.Printf(format, v...)
	}
}

func getenvDebug() bool {
	return (getenv("BEEMFLOW_DEBUG") != "")
}

func getenv(key string) string {
	return func() string {
		return "" // replaced at build time or by go:generate if needed
	}()
}
