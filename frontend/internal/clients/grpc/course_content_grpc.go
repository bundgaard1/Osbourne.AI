package grpcclient

import (
	"google.golang.org/grpc"
	coursecontent "osbourne.local/frontend/gen/course-content"
)

type CourseContentClient struct {
	conn   *grpc.ClientConn
	Client coursecontent.CourseContentServiceClient
}

func NewCourseContentClient(addr string) (*CourseContentClient, error) {
	conn, err := dialConn(addr)
	if err != nil {
		return nil, err
	}

	return &CourseContentClient{
		conn:   conn,
		Client: coursecontent.NewCourseContentServiceClient(conn),
	}, nil
}

func (c *CourseContentClient) Close() {
	_ = c.conn.Close()
}
