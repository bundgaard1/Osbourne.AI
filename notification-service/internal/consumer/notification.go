package consumer

import (
	"context"
	"fmt"
	"log"

	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/protobuf/proto"
	"osbourne.local/notification-service/gen/events"
	"osbourne.local/notification-service/internal/service"
)

type NotificationConsumer struct {
	rmq *rabbitmq.Consumer
	svc *service.NotificationService
}

func NewNotificationConsumer(conn *rabbitmq.Conn, svc *service.NotificationService) (*NotificationConsumer, error) {
	nc := &NotificationConsumer{
		svc: svc,
	}

	consumer, err := rabbitmq.NewConsumer(
		conn,
		"notification_service_queue",
		rabbitmq.WithConsumerOptionsQueueDurable,
		rabbitmq.WithConsumerOptionsExchangeName("university.events"),
		rabbitmq.WithConsumerOptionsExchangeKind("topic"),
		rabbitmq.WithConsumerOptionsExchangeDurable,
		rabbitmq.WithConsumerOptionsExchangeDeclare,
		rabbitmq.WithConsumerOptionsRoutingKey("account.created"),
		rabbitmq.WithConsumerOptionsRoutingKey("course.*"),
		rabbitmq.WithConsumerOptionsRoutingKey("grade.*"),
		rabbitmq.WithConsumerOptionsConcurrency(4),
	)

	if err != nil {
		return nil, err
	}

	nc.rmq = consumer
	return nc, nil
}

func (c *NotificationConsumer) Start(ctx context.Context) error {
	log.Println("[CONSUMER] NotificationConsumer listening on RabbitMQ...")

	err := c.rmq.Run(func(d rabbitmq.Delivery) rabbitmq.Action {
		return c.processDelivery(ctx, d.Body)
	})

	return err
}

func (c *NotificationConsumer) Close() {
	if c.rmq != nil {
		c.rmq.Close()
	}
}

func (c *NotificationConsumer) processDelivery(ctx context.Context, body []byte) rabbitmq.Action {
	var envelope events.EventEnvelope
	if err := proto.Unmarshal(body, &envelope); err != nil {
		log.Printf("[CONSUMER] Invalid EventEnvelope format: %v", err)
		return rabbitmq.NackDiscard
	}

	fmt.Printf("[CONSUMER] Received event: %s", envelope.Type)

	switch envelope.Type {
	case "account.created":
		var event events.AccountCreatedEvent
		if err := proto.Unmarshal(envelope.Payload, &event); err != nil {
			log.Printf("[CONSUMER] Error on unmarshal of AccountCreatedEvent: %v", err)
			return rabbitmq.NackDiscard
		}

		// Call the business logic
		err := c.svc.CreateNotification(ctx,
			event.GetAccountId(),
			"Welcome to Osbourne!",
			"Hello "+event.GetFullName()+", welcome to Osbourne! We are excited to have you on board.")

		if err != nil {
			log.Printf("[CONSUMER] Error on creating notification: %v", err)
			return rabbitmq.NackRequeue
		}
	case "course.enrolled":
		var event events.CourseEnrolledEvent
		if err := proto.Unmarshal(envelope.Payload, &event); err != nil {
			log.Printf("[CONSUMER] Error on unmarshal of CourseEnrolledEvent: %v", err)
			return rabbitmq.NackDiscard
		}

		err := c.svc.CreateNotification(ctx,
			event.GetStudentId(),
			"Enrolled in Course: "+event.GetCourseCode(),
			"You have been enrolled in the course: "+event.GetCourseName()+".")

		if err != nil {
			log.Printf("[CONSUMER] Error on creating notification: %v", err)
			return rabbitmq.NackRequeue
		}
	case "grade.published":
		var event events.GradePublishedEvent
		if err := proto.Unmarshal(envelope.Payload, &event); err != nil {
			log.Printf("[CONSUMER] Error on unmarshal of GradePublishedEvent: %v", err)
			return rabbitmq.NackDiscard
		}

		err := c.svc.CreateNotification(ctx,
			event.GetStudentId(),
			"Grade published",
			fmt.Sprintf("Grade updated for Assignment: %s; \n Grade: %d; \n Course: %s.", event.GetAssignmentName(), event.GetGrade(), event.GetCourseId()))

		if err != nil {
			log.Printf("[CONSUMER] Error on creating notification: %v", err)
			return rabbitmq.NackRequeue
		}
	default:
		log.Printf("[CONSUMER] Ignoring unknown event-type: %s", envelope.Type)
	}

	return rabbitmq.Ack
}
