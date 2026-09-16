package grpcclient

import (
	"google.golang.org/grpc"
	"osbourne.local/frontend/gen/notification"
)

type NotificationClient struct {
	conn   *grpc.ClientConn
	Client notification.NotificationServiceClient
}

func NewNotificationClient(addr string) (*NotificationClient, error) {
	conn, err := dialConn(addr)
	if err != nil {
		return nil, err
	}

	return &NotificationClient{
		conn:   conn,
		Client: notification.NewNotificationServiceClient(conn),
	}, nil
}

func (c *NotificationClient) Close() {
	_ = c.conn.Close()
}
