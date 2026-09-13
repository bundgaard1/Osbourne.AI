package seed

import (
	"context"
	"log/slog"
	"strings"

	"osbourne.local/assignment-service/internal/domain"
)

// row ties a seeded submission to its physical file so the database seed and
// the file seed can never drift apart.
type row struct {
	SubmissionID string
	AssignmentID string
	StudentID    string
	Filename     string
	Path         string
	Content      string
}

var rows = []row{
	{
		SubmissionID: "11111111-1111-4111-8111-111111111111",
		AssignmentID: "1",
		StudentID:    "12345",
		Filename:     "submission-1.txt",
		Path:         "11111111-1111-4111-8111-111111111111.txt",
		Content:      "sample submission content for assignment 1",
	},
	{
		SubmissionID: "22222222-2222-4222-8222-222222222222",
		AssignmentID: "1",
		StudentID:    "12345",
		Filename:     "submission-2.txt",
		Path:         "22222222-2222-4222-8222-222222222222.txt",
		Content:      "sample submission content for assignment 2",
	},
}

// Submissions returns the demo submission rows, complete with their flat
// UUID file paths, for the database seed.
func Submissions() []domain.Submission {
	out := make([]domain.Submission, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Submission{
			ID:           r.SubmissionID,
			AssignmentID: r.AssignmentID,
			StudentID:    r.StudentID,
			FileID:      r.Path,
			FileName:     r.Filename,
			FileSize:     int64(len(r.Content)),
		})
	}
	return out
}

// SeedFiles writes the seeded submission files via the domain.FileStorage
// contract, so downloads resolve for the database-seeded submissions.
func SeedFiles(ctx context.Context, fs domain.FileStorage) error {
	for _, r := range rows {
		if _, err := fs.Save(ctx, r.Path, strings.NewReader(r.Content)); err != nil {
			return err
		}
		slog.Info("seeded file", "path", r.Path)
	}
	return nil
}