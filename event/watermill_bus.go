package event

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-nats/pkg/nats"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	stan "github.com/nats-io/stan.go"
)

// WatermillEventBus satisfies our EventBus interface using Watermill.
type WatermillEventBus struct {
	publisher  message.Publisher
	subscriber message.Subscriber
}

// NewWatermillInMemBus returns a Watermill-based, in-memory bus.
func NewWatermillInMemBus() *WatermillEventBus {
	logger := watermill.NewStdLogger(false, false)
	ps := gochannel.NewGoChannel(gochannel.Config{OutputChannelBuffer: 100}, logger)
	return &WatermillEventBus{publisher: ps, subscriber: ps}
}

// (Optional) NewWatermillNATSBUS returns a NATS-backed bus:.
func NewWatermillNATSBUS(clusterID, clientID, url string) *WatermillEventBus {
	logger := watermill.NewStdLogger(false, false)
	pub, _ := nats.NewStreamingPublisher(nats.StreamingPublisherConfig{
		ClusterID: clusterID,
		ClientID:  clientID,
		StanOptions: []stan.Option{
			stan.NatsURL(url),
		},
	}, logger)
	sub, _ := nats.NewStreamingSubscriber(nats.StreamingSubscriberConfig{
		ClusterID: clusterID,
		ClientID:  clientID,
		StanOptions: []stan.Option{
			stan.NatsURL(url),
		},
		CloseTimeout:   30 * time.Second,
		AckWaitTimeout: 30 * time.Second,
	}, logger)
	return &WatermillEventBus{publisher: pub, subscriber: sub}
}

func (b *WatermillEventBus) Publish(topic string, payload any) error {
	var data []byte
	switch v := payload.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	case map[string]any:
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal map payload: %w", err)
		}
	default:
		// fallback: use fmt.Sprintf for non-bytes
		data = []byte(fmt.Sprintf("%v", v))
	}
	msg := message.NewMessage(watermill.NewUUID(), data)
	return b.publisher.Publish(topic, msg)
}

func (b *WatermillEventBus) Subscribe(ctx context.Context, topic string, handler func(payload any)) {
	ch, err := b.subscriber.Subscribe(ctx, topic)
	if err != nil {
		return
	}
	go func() {
		for msg := range ch {
			data := msg.Payload // msg.Payload is []byte
			// Try to decode as int
			if i, err := strconv.Atoi(string(data)); err == nil {
				handler(i)
				msg.Ack()
				continue
			}
			// Try to decode as map[string]any (JSON)
			var m map[string]any
			if err := json.Unmarshal(data, &m); err == nil && len(m) > 0 {
				handler(m)
				msg.Ack()
				continue
			}
			// fallback: pass as string
			handler(string(data))
			msg.Ack()
		}
	}()
}
