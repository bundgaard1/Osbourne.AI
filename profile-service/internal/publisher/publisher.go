package publisher

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"osbourne.local/profile-service/gen/events"
	"osbourne.local/profile-service/internal/domain"
)

const (
	ExchangeName = "university.events"
	ExchangeKind = "topic"
)

// Publisher is a wrapper around go-rabbitmq that owns the "university.events"
// topic exchange and publishes domain events produced by the profile service.
type Publisher struct {
	pub *rabbitmq.Publisher
}

// New creates a Publisher over an existing RabbitMQ connection. The exchange is
// declared durably (idempotent) so it matches the topology owned by the
// notification-service consumer.
func New(conn *rabbitmq.Conn) (*Publisher, error) {
	pub, err := rabbitmq.NewPublisher(
		conn,
		rabbitmq.WithPublisherOptionsExchangeName(ExchangeName),
		rabbitmq.WithPublisherOptionsExchangeKind(ExchangeKind),
		rabbitmq.WithPublisherOptionsExchangeDurable,
		rabbitmq.WithPublisherOptionsExchangeDeclare,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rabbitmq publisher: %w", err)
	}
	return &Publisher{pub: pub}, nil
}

// PublishStudentCreated publishes a student.created event on the topic
// exchange. Messages use persistent delivery so they accumulate on the durable
// queue while the consumer is offline.
func (p *Publisher) PublishStudentCreated(ctx context.Context, event domain.StudentCreatedEvent) error {
	studentEvent := &events.StudentCreatedEvent{
		StudentId: event.StudentID,
		Email:     event.Email,
		FullName:  event.FullName,
	}

	payload, err := proto.Marshal(studentEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal StudentCreatedEvent: %w", err)
	}

	envelope := &events.EventEnvelope{
		Id:        uuid.NewString(),
		Type:      "student.created",
		Timestamp: timestamppb.New(time.Now()),
		Payload:   payload,
	}

	body, err := proto.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal EventEnvelope: %w", err)
	}

	err = p.pub.PublishWithContext(ctx, body, []string{"student.created"},
		rabbitmq.WithPublishOptionsExchange(ExchangeName),
		rabbitmq.WithPublishOptionsContentType("application/protobuf"),
		rabbitmq.WithPublishOptionsType("student.created"),
		rabbitmq.WithPublishOptionsMessageID(envelope.GetId()),
		rabbitmq.WithPublishOptionsTimestamp(envelope.GetTimestamp().AsTime()),
		rabbitmq.WithPublishOptionsPersistentDelivery,
	)
	if err != nil {
		return fmt.Errorf("failed to publish student.created event: %w", err)
	}

	log.Printf("[PUBLISHED] Published student.created event for student=%s", event.StudentID)
	return nil
}

func (p *Publisher) Close() {
	if p.pub != nil {
		p.pub.Close()
	}
}