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

	authcommon "osbourne.local/auth-common"
	"osbourne.local/course-catalogue-service/gen/events"
)

const (
	ExchangeName = "university.events"
	ExchangeKind = "topic"
)

// Publisher is a small, reusable wrapper around go-rabbitmq that owns the
// "university.events" topic exchange and publishes domain events produced by
// the course catalogue service.
type Publisher struct {
	pub *rabbitmq.Publisher
}

// New creates a Publisher over an existing RabbitMQ connection. The exchange is
// declared durably (idempotent), so the consumer and the publisher agree on the
// same topology regardless of which one starts first.
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

// PublishCourseEnrolled publishes a course.enrolled event on the topic
// exchange. Messages use persistent delivery so they survive broker restarts
// and accumulate on the durable queue while consumers are offline.
func (p *Publisher) PublishCourseEnrolled(ctx context.Context, studentID, courseID, courseCode, courseName string) error {
	event := &events.CourseEnrolledEvent{
		StudentId:  studentID,
		CourseId:   courseID,
		CourseCode: courseCode,
		CourseName: courseName,
	}

	payload, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal CourseEnrolledEvent: %w", err)
	}

	envelope := &events.EventEnvelope{
		Id:        uuid.NewString(),
		Type:      "course.enrolled",
		Timestamp: timestamppb.New(time.Now()),
		Payload:   payload,
	}

	body, err := proto.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal EventEnvelope: %w", err)
	}

	err = p.pub.PublishWithContext(ctx, body, []string{"course.enrolled"},
		rabbitmq.WithPublishOptionsExchange(ExchangeName),
		rabbitmq.WithPublishOptionsContentType("application/protobuf"),
		rabbitmq.WithPublishOptionsType("course.enrolled"),
		rabbitmq.WithPublishOptionsMessageID(envelope.GetId()),
		rabbitmq.WithPublishOptionsTimestamp(envelope.GetTimestamp().AsTime()),
		rabbitmq.WithPublishOptionsPersistentDelivery,
	)
	if err != nil {
		return fmt.Errorf("failed to publish course.enrolled event: %w", err)
	}

	slog.InfoContext(ctx, "published course.enrolled event", "student_id", studentID, "course_id", courseID, "event_id", envelope.GetId())
	return nil
}

func (p *Publisher) Close() {
	if p.pub != nil {
		p.pub.Close()
	}
}
