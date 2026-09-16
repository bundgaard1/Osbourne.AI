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

	"osbourne.local/assignment-service/gen/events"
	"osbourne.local/assignment-service/internal/domain"
	authcommon "osbourne.local/auth-common"
)

const (
	ExchangeName = "university.events"
	ExchangeKind = "topic"
)

// Publisher is a wrapper around go-rabbitmq that owns the "university.events"
// topic exchange and publishes domain events produced by the assignment service.
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
		rabbitmq.WithPublisherOptionsLogger(rabbitmq.Logger(authcommon.RabbitLogger{})),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rabbitmq publisher: %w", err)
	}
	return &Publisher{pub: pub}, nil
}

// PublishGradePublished publishes a grade.published event on the topic
// exchange. Messages use persistent delivery so they accumulate on the durable
// queue while the consumer is offline.
func (p *Publisher) PublishGradePublished(ctx context.Context, event domain.GradePublishedEvent) error {
	gradeEvent := &events.GradePublishedEvent{
		SubmissionId:   event.SubmissionID,
		AssignmentId:   event.AssignmentID,
		AssignmentName: event.AssignmentName,
		StudentId:      event.StudentID,
		CourseId:       event.CourseID,
		Grade:          int32(event.Score),
	}

	payload, err := proto.Marshal(gradeEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal GradePublishedEvent: %w", err)
	}

	envelope := &events.EventEnvelope{
		Id:        uuid.NewString(),
		Type:      "grade.published",
		Timestamp: timestamppb.New(time.Now()),
		Payload:   payload,
	}

	body, err := proto.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal EventEnvelope: %w", err)
	}

	err = p.pub.PublishWithContext(ctx, body, []string{"grade.published"},
		rabbitmq.WithPublishOptionsExchange(ExchangeName),
		rabbitmq.WithPublishOptionsContentType("application/protobuf"),
		rabbitmq.WithPublishOptionsType("grade.published"),
		rabbitmq.WithPublishOptionsMessageID(envelope.GetId()),
		rabbitmq.WithPublishOptionsTimestamp(envelope.GetTimestamp().AsTime()),
		rabbitmq.WithPublishOptionsPersistentDelivery,
	)
	if err != nil {
		return fmt.Errorf("failed to publish grade.published event: %w", err)
	}

	slog.InfoContext(ctx, "published grade.published event", "student_id", event.StudentID, "course_id", event.CourseID, "event_id", envelope.GetId())
	return nil
}

func (p *Publisher) Close() {
	if p.pub != nil {
		p.pub.Close()
	}
}
