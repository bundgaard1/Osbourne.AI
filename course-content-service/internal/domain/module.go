package domain

import (
	"time"
)

type Module struct {
	ID        string    `json:"id" clover:"id"`
	CourseID  string    `json:"courseId" clover:"courseId"`
	Title     string    `json:"title" clover:"title"`
	Text      string    `json:"text" clover:"text"`
	UpdatedAt time.Time `json:"updatedAt" clover:"updatedAt"`
}
