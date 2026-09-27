package domain

import (
	"errors"
	"time"
)

// ErrNotFound is returned by the repository when a lookup matches no document.
//
// It exists as a sentinel so the service layer can tell "no such module" from
// "the database is unreachable" and map only the first to a 404. Returning a
// bare (nil, nil) for a miss - which is what this repository used to do - is
// what let a missing module reach the RPC layer as a nil pointer and panic the
// service.
var ErrNotFound = errors.New("not found")

type Module struct {
	ID        string    `json:"id" clover:"id"`
	CourseID  string    `json:"courseId" clover:"courseId"`
	Title     string    `json:"title" clover:"title"`
	Text      string    `json:"text" clover:"text"`
	UpdatedAt time.Time `json:"updatedAt" clover:"updatedAt"`
}
