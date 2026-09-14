package consumer

import (
	"context"
	"log"

	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/protobuf/proto"

	"osbourne.local/profile-service/gen/events"
	"osbourne.local/profile-service/internal/service"
)

// ProfileConsumer listens for account.created events and creates the profile
// master-data row that belongs to each new account.
type ProfileConsumer struct {
	rmq *rabbitmq.Consumer
	svc *service.ProfileService
}

func NewProfileConsumer(conn *rabbitmq.Conn, svc *service.ProfileService) (*ProfileConsumer, error) {
	pc := &ProfileConsumer{
		svc: svc,
	}

	consumer, err := rabbitmq.NewConsumer(
		conn,
		"profile_service_queue",
		rabbitmq.WithConsumerOptionsQueueDurable,
		rabbitmq.WithConsumerOptionsExchangeName("university.events"),
		rabbitmq.WithConsumerOptionsExchangeKind("topic"),
		rabbitmq.WithConsumerOptionsExchangeDurable,
		rabbitmq.WithConsumerOptionsExchangeDeclare,
		rabbitmq.WithConsumerOptionsRoutingKey("account.created"),
		rabbitmq.WithConsumerOptionsConcurrency(4),
	)
	if err != nil {
		return nil, err
	}

	pc.rmq = consumer
	return pc, nil
}

func (c *ProfileConsumer) Start(ctx context.Context) error {
	log.Println("[CONSUMER] ProfileConsumer listening on RabbitMQ for account.created...")

	err := c.rmq.Run(func(d rabbitmq.Delivery) rabbitmq.Action {
		return c.processDelivery(ctx, d.Body)
	})

	return err
}

func (c *ProfileConsumer) Close() {
	if c.rmq != nil {
		c.rmq.Close()
	}
}

func (c *ProfileConsumer) processDelivery(ctx context.Context, body []byte) rabbitmq.Action {
	var envelope events.EventEnvelope
	if err := proto.Unmarshal(body, &envelope); err != nil {
		log.Printf("[CONSUMER] Invalid EventEnvelope format: %v", err)
		return rabbitmq.NackDiscard
	}

	if envelope.Type != "account.created" {
		log.Printf("[CONSUMER] Ignoring unknown event-type: %s", envelope.Type)
		return rabbitmq.Ack
	}

	var event events.AccountCreatedEvent
	if err := proto.Unmarshal(envelope.Payload, &event); err != nil {
		log.Printf("[CONSUMER] Error on unmarshal of AccountCreatedEvent: %v", err)
		return rabbitmq.NackDiscard
	}

	err := c.svc.CreateProfileFromEvent(ctx, event.GetAccountId(), event.GetFullName())
	if err != nil {
		log.Printf("[CONSUMER] Error on creating profile: %v", err)
		return rabbitmq.NackRequeue
	}

	return rabbitmq.Ack
}