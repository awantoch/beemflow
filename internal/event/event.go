package event

import "sync"

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
	b.mu.Lock()
	handlers := b.handlers[topic]
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return nil
}

func (b *InProcEventBus) Subscribe(topic string, handler func(payload any)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
}
