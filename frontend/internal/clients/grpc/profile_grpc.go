package grpcclient

import (
	"google.golang.org/grpc"
	"osbourne.local/frontend/gen/profile"
)

type ProfileClient struct {
	conn   *grpc.ClientConn
	Client profile.ProfileServiceClient
}

func NewProfileClient(addr string) (*ProfileClient, error) {
	conn, err := dialConn(addr)
	if err != nil {
		return nil, err
	}

	return &ProfileClient{
		conn:   conn,
		Client: profile.NewProfileServiceClient(conn),
	}, nil
}

func (c *ProfileClient) Close() {
	_ = c.conn.Close()
}
