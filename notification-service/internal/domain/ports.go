package domain

import (
	"context"
)

// NotificationRepository definerer alle database-operationer for notifikationer
type NotificationRepository interface {
	Create(context.Context, *Notification) error
	Get(context.Context, string) (*Notification, error)
	Update(context.Context, *Notification) (*Notification, error)
	ListByUser(context.Context, string) (*[]Notification, error)
}
