package grpcclient

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"osbourne.local/frontend/gen/auth"
)

type AuthClient struct {
	conn   *grpc.ClientConn
	Client auth.AuthServiceClient
}

func NewAuthClient(addr string) (*AuthClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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