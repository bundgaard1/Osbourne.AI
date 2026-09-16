package publisher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"osbourne.local/common"
	"osbourne.local/auth-service/gen/events"
	"osbourne.local/auth-service/internal/domain"
)

const (
	ExchangeName = "university.events"
	ExchangeKind = "topic"
)

// Publisher wraps go-rabbitmq and owns the "university.events" topic exchange
// on which account.created events are published.
type Publisher struct {
	pub *rabbitmq.Publisher
}

func New(conn *rabbitmq.Conn) (*Publisher, error) {
	pub, err := rabbitmq.NewPublisher(
		conn,
		rabbitmq.WithPublisherOptionsExchangeName(ExchangeName),
		rabbitmq.WithPublisherOptionsExchangeKind(ExchangeKind),
		rabbitmq.WithPublisherOptionsExchangeDurable,
		rabbitmq.WithPublisherOptionsExchangeDeclare,
		rabbitmq.WithPublisherOptionsLogger(rabbitmq.Logger(common.RabbitLogger{})),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rabbitmq publisher: %w", err)
	}
	return &Publisher{pub: pub}, nil
}

// PublishAccountCreated publishes the account's creation on the topic
// exchange. Messages use persistent delivery so they accumulate on the durable
// consumer queues while the consumers are offline.
func (p *Publisher) PublishAccountCreated(ctx context.Context, event domain.AccountCreatedEvent) error {
	accountEvent := &events.AccountCreatedEvent{
		AccountId: event.AccountID,
		Email:     event.Email,
		Role:      event.Role,
		FullName:  event.FullName,
	}

	payload, err := proto.Marshal(accountEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal AccountCreatedEvent: %w", err)
	}

	envelope := &events.EventEnvelope{
		Id:        uuid.NewString(),
		Type:      "account.created",
		Timestamp: timestamppb.New(time.Now()),
		Payload:   payload,
	}

	body, err := proto.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal EventEnvelope: %w", err)
	}

	err = p.pub.PublishWithContext(ctx, body, []string{"account.created"},
		rabbitmq.WithPublishOptionsExchange(ExchangeName),
		rabbitmq.WithPublishOptionsContentType("application/protobuf"),
		rabbitmq.WithPublishOptionsType("account.created"),
		rabbitmq.WithPublishOptionsMessageID(envelope.GetId()),
		rabbitmq.WithPublishOptionsTimestamp(envelope.GetTimestamp().AsTime()),
		rabbitmq.WithPublishOptionsPersistentDelivery,
	)
	if err != nil {
		return fmt.Errorf("failed to publish account.created event: %w", err)
	}

	slog.InfoContext(ctx, "published account.created event", "account_id", event.AccountID, "event_id", envelope.GetId())
	return nil
}

func (p *Publisher) Close() {
	if p.pub != nil {
		p.pub.Close()
	}
}