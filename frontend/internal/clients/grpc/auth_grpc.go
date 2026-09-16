package grpcclient

import (
	"google.golang.org/grpc"

	"osbourne.local/frontend/gen/auth"
)

type AuthClient struct {
	conn   *grpc.ClientConn
	Client auth.AuthServiceClient
}

func NewAuthClient(addr string) (*AuthClient, error) {
	conn, err := dialConn(addr)
	if err != nil {
		return nil, err
	}

	return &AuthClient{
		conn:   conn,
		Client: auth.NewAuthServiceClient(conn),
	}, nil
}

func (c *AuthClient) Close() {
	_ = c.conn.Close()
}