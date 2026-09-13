package server

import (
	"context"

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
	notifications, err := s.notificationSvc.GetUserNotifications(ctx, req.GetUserId())
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
	not, err := s.notificationSvc.MarkNotificationAsRead(ctx, req.GetNotificationId())
	if err != nil {
		return nil, err
	}

	return &notification.MarkNotificationAsReadResponse{
		Success: not.IsRead,
	}, nil
}

func toProtoNotification(n *domain.Notification) *notification.Notification {
	return &notification.Notification{
		Id:     n.ID,
		UserId: n.UserID,
		Title:  n.Title,
		Msg:    n.Message,
		IsRead: n.IsRead,
	}
}
