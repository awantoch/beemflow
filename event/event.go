package event

import (
	"context"
	"fmt"

	"github.com/awantoch/beemflow/config"
)

type EventBus interface {
	Publish(topic string, payload any) error
	Subscribe(ctx context.Context, topic string, handler func(payload any))
}

// NewInProcEventBus returns a new in-memory event bus. Used when event config driver=="memory" or omitted.
func NewInProcEventBus() *WatermillEventBus {
	return NewWatermillInMemBus()
}

// NewEventBusFromConfig returns an EventBus based on config. Supported: memory (default), nats (with url).
// Unknown drivers fail cleanly. See docs/flow_config.schema.json for config schema.
func NewEventBusFromConfig(cfg *config.EventConfig) (EventBus, error) {
	if cfg == nil || cfg.Driver == "" || cfg.Driver == "memory" {
		return NewWatermillInMemBus(), nil
	}
	switch cfg.Driver {
	case "nats":
		if cfg.URL == "" {
			return nil, fmt.Errorf("NATS driver requires url")
		}
		// Use default clusterID/clientID for now
		return NewWatermillNATSBUS("beemflow", "beemflow-client", cfg.URL), nil
	default:
		return nil, fmt.Errorf("unsupported event bus driver: %s", cfg.Driver)
	}
}
