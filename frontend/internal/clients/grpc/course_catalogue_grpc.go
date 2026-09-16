package grpcclient

import (
	"google.golang.org/grpc"
	coursecatalogue "osbourne.local/frontend/gen/course-catalogue"
)

type CourseCatalogueClient struct {
	conn   *grpc.ClientConn
	Client coursecatalogue.CourseCatalogueServiceClient
}

func NewCourseCatalogueClient(addr string) (*CourseCatalogueClient, error) {
	conn, err := dialConn(addr)
	if err != nil {
		return nil, err
	}

	return &CourseCatalogueClient{
		conn:   conn,
		Client: coursecatalogue.NewCourseCatalogueServiceClient(conn),
	}, nil
}

func (c *CourseCatalogueClient) Close() {
	_ = c.conn.Close()
}
