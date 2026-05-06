package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-kafka/v3/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill/message"
)

// Publisher wraps watermill's message.Publisher
type Publisher struct {
	pub message.Publisher
}

// NewKafkaPublisher initializes a Kafka publisher.
func NewKafkaPublisher(brokers []string) (*Publisher, error) {
	pub, err := kafka.NewPublisher(
		kafka.PublisherConfig{
			Brokers:   brokers,
			Marshaler: kafka.DefaultMarshaler{},
		},
		watermill.NewStdLogger(false, false),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka publisher: %w", err)
	}

	return &Publisher{pub: pub}, nil
}

// PublishJSON serializes payload to JSON and sends it to the topic.
func (p *Publisher) PublishJSON(ctx context.Context, topic string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	msg := message.NewMessage(watermill.NewUUID(), b)
	// Optional: Propagate context/trace headers here if needed

	if err := p.pub.Publish(topic, msg); err != nil {
		return fmt.Errorf("publish message to %s: %w", topic, err)
	}

	log.Printf("Published event to %s: %s", topic, string(b))
	return nil
}

// Close gracefully shuts down the publisher.
func (p *Publisher) Close() error {
	return p.pub.Close()
}
