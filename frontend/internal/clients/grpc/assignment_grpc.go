package grpcclient

import (
	"google.golang.org/grpc"
	assignment "osbourne.local/frontend/gen/assignment"
)

type AssignmentClient struct {
	conn   *grpc.ClientConn
	Client assignment.AssignmentServiceClient
}

func NewAssignmentClient(addr string) (*AssignmentClient, error) {
	conn, err := dialConn(addr)
	if err != nil {
		return nil, err
	}

	return &AssignmentClient{
		conn:   conn,
		Client: assignment.NewAssignmentServiceClient(conn),
	}, nil
}

func (c *AssignmentClient) Close() {
	_ = c.conn.Close()
}
