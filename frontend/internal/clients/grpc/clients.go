package grpcclient

import "fmt"

// Clients is an aggregate of the gRPC client connections used by the
// frontend. All connections are created lazily (grpc.NewClient does not
// dial) and are owned by this aggregate so they can be closed together.
type Clients struct {
	Auth            *AuthClient
	Profile         *ProfileClient
	Notification    *NotificationClient
	CourseCatalogue *CourseCatalogueClient
	CourseContent   *CourseContentClient
	Assignment      *AssignmentClient
}

// Dial builds all clients. grpc.NewClient is lazy and non-blocking, so this
// only fails on an invalid target string. If any client fails to construct,
// the connections already created are closed before returning the error.
func Dial(authAddr, profileAddr, notificationAddr, catalogueAddr, contentAddr, assignmentAddr string) (*Clients, error) {
	clients := &Clients{}

	var err error
	if clients.Auth, err = NewAuthClient(authAddr); err != nil {
		clients.Close()
		return nil, fmt.Errorf("failed to create auth client: %w", err)
	}
	if clients.Profile, err = NewProfileClient(profileAddr); err != nil {
		clients.Close()
		return nil, fmt.Errorf("failed to create profile client: %w", err)
	}
	if clients.Notification, err = NewNotificationClient(notificationAddr); err != nil {
		clients.Close()
		return nil, fmt.Errorf("failed to create notification client: %w", err)
	}
	if clients.CourseCatalogue, err = NewCourseCatalogueClient(catalogueAddr); err != nil {
		clients.Close()
		return nil, fmt.Errorf("failed to create course catalogue client: %w", err)
	}
	if clients.CourseContent, err = NewCourseContentClient(contentAddr); err != nil {
		clients.Close()
		return nil, fmt.Errorf("failed to create course content client: %w", err)
	}
	if clients.Assignment, err = NewAssignmentClient(assignmentAddr); err != nil {
		clients.Close()
		return nil, fmt.Errorf("failed to create assignment client: %w", err)
	}

	return clients, nil
}

func (c *Clients) Close() {
	if c == nil {
		return
	}
	if c.Auth != nil {
		c.Auth.Close()
	}
	if c.Profile != nil {
		c.Profile.Close()
	}
	if c.Notification != nil {
		c.Notification.Close()
	}
	if c.CourseCatalogue != nil {
		c.CourseCatalogue.Close()
	}
	if c.CourseContent != nil {
		c.CourseContent.Close()
	}
	if c.Assignment != nil {
		c.Assignment.Close()
	}
}
