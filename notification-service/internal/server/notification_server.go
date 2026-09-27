package server

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"osbourne.local/common"
	"osbourne.local/notification-service/gen/notification"
	"osbourne.local/notification-service/internal/domain"
	"osbourne.local/notification-service/internal/service"
)

type NotificationServer struct {
	notification.UnimplementedNotificationServiceServer
	notificationSvc *service.NotificationService
}

func NewNotificationServer(notificationSvc *service.NotificationService) *NotificationServer {
	return &NotificationServer{
		notificationSvc: notificationSvc,
	}
}

func (s *NotificationServer) GetUserNotifications(ctx context.Context, req *notification.NotificationsRequest) (*notification.NotificationsResponse, error) {
	// GET /api/notifications has no place to put a user_id, so the token
	// decides. Trusted internal gRPC callers still pass it explicitly, which is
	// what keeps the frontend's SSR handlers working until Step 7 removes them.
	userID, err := common.UserIDFromContextOrRequest(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}

	notifications, err := s.notificationSvc.GetUserNotifications(ctx, userID)
	if err != nil {
		return nil, err
	}

	protoNotifications := make([]*notification.Notification, 0, len(*notifications))
	for _, n := range *notifications {
		protoNotifications = append(protoNotifications, toProtoNotification(&n))
	}

	return &notification.NotificationsResponse{
		Notifications: protoNotifications,
	}, nil
}

func (s *NotificationServer) MarkNotificationAsRead(ctx context.Context, req *notification.MarkNotificationAsReadRequest) (*notification.MarkNotificationAsReadResponse, error) {
	// The notification id comes from the URL, so it is attacker-controlled.
	// The owner is taken from the token and the service verifies the two match.
	claims, ok := common.ClaimsFromContext(ctx)
	if !ok || claims.UserID == "" {
		return nil, common.RequiresAuthentication()
	}

	not, err := s.notificationSvc.MarkNotificationAsRead(ctx, claims.UserID, req.GetNotificationId())
	if err != nil {
		return nil, err
	}

	return &notification.MarkNotificationAsReadResponse{
		Success: not.IsRead,
	}, nil
}

func toProtoNotification(n *domain.Notification) *notification.Notification {
	return &notification.Notification{
		Id:        n.ID,
		UserId:    n.UserID,
		Title:     n.Title,
		Msg:       n.Message,
		IsRead:    n.IsRead,
		Timestamp: timestamppb.New(n.CreatedAt),
	}
}
