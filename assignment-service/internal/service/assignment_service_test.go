package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"osbourne.local/assignment-service/internal/domain"
	"osbourne.local/assignment-service/internal/testutils"
)

var uuidLike = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func setupService(t *testing.T) (
	*AssignmentService,
	*testutils.FakeAssignemntsRepository,
	*testutils.FakeSubmissionRepository,
	*testutils.FakeStorage,
) {
	t.Helper() // Marks this function as a test helper for cleaner stack traces

	assignmentsRepo := testutils.NewFakeAssignmentsRepository()
	submissionRepo := testutils.NewFakeSubmissionRepository()
	storage := testutils.NewFakeFileStorage()

	svc := NewAssignmentService(assignmentsRepo, submissionRepo, storage)

	return svc, assignmentsRepo, submissionRepo, storage
}

func TestSubmitAssignment_SavesUnderUuidPathAndPersistsMetadata(t *testing.T) {
	svc, assignmentRepo, submissionRepo, storage := setupService(t)

	assignmentRepo.Assignments["assn_1"] = &domain.Assignment{ID: "assn_1"}

	submission, err := svc.SubmitAssignment(context.Background(), SubmitAssignmentInput{
		AssignmentID: "assn_1",
		StudentID:    "student_1",
		FileName:     "report.pdf",
		Size:         10,
	}, strings.NewReader("filecontent"))
	if err != nil {
		t.Fatalf("SubmitAssignment failed: %v", err)
	}

	// Path must be built only from server-side IDs + a UUID name
	parts := strings.Split(submission.FileURL, "/")
	if len(parts) != 4 || parts[0] != "submissions" || parts[1] != "assn_1" {
		t.Fatalf("unexpected file url: %q", submission.FileURL)
	}
	if parts[2] != submission.ID {
		t.Errorf("directory should be the submission id, got %q", parts[2])
	}
	name := strings.TrimSuffix(parts[3], ".pdf")
	if !uuidLike.MatchString(name) {
		t.Errorf("stored filename should be a uuid, got %q", parts[3])
	}
	if strings.Contains(parts[3], "report") {
		t.Errorf("original filename leaked into storage path: %q", submission.FileURL)
	}

	if submission.FileName != "report.pdf" || submission.FileSize != int64(len("filecontent")) {
		t.Errorf("metadata mismatch: %+v", submission)
	}

	if got := string(storage.Files[submission.FileURL]); got != "filecontent" {
		t.Errorf("stored content mismatch: %q", got)
	}

	if len(submissionRepo.Submissions) != 1 {
		t.Fatalf("expected 1 submission in repo, got %d", len(submissionRepo.Submissions))
	}
}

func TestSubmitAssignment_RemovesFileWhenDbFails(t *testing.T) {
	svc, assignmentRepo, submissionRepo, storage := setupService(t)

	assignmentRepo.Assignments["assn_1"] = &domain.Assignment{ID: "assn_1"}
	submissionRepo.FailCreate = true

	_, err := svc.SubmitAssignment(context.Background(), SubmitAssignmentInput{
		AssignmentID: "assn_1",
		StudentID:    "student_1",
		FileName:     "report.pdf",
		Size:         10,
	}, strings.NewReader("filecontent"))
	if err == nil {
		t.Fatal("expected error when DB write fails")
	}

	if len(storage.Files) != 0 {
		t.Errorf("file should have been rolled back, still stored: %v", storage.Files)
	}
	if len(storage.Deleted) != 1 {
		t.Errorf("expected exactly 1 delete call, got %d", len(storage.Deleted))
	}
}

func TestSubmitAssignment_AssignmentNotFound(t *testing.T) {
	svc, _, _, storage := setupService(t)

	_, err := svc.SubmitAssignment(context.Background(), SubmitAssignmentInput{
		AssignmentID: "missing",
		StudentID:    "student_1",
		FileName:     "report.pdf",
		Size:         10,
	}, strings.NewReader("filecontent"))
	if !errors.Is(err, ErrAssignmentNotFound) {
		t.Fatalf("expected ErrAssignmentNotFound, got %v", err)
	}

	if len(storage.Files) != 0 {
		t.Errorf("no file should be stored for a missing assignment: %v", storage.Files)
	}
}

func TestSanitizedExtension(t *testing.T) {
	cases := map[string]string{
		"report.pdf":         ".pdf",
		"report.PDF":         ".pdf",
		"../evil/report.PDF": ".pdf",
		"noext":              "",
		"report.":            "",
		"report.exe;rm":      "",
		"report.tar.gz":      ".gz",
		"a.pdf  ":            "",
		"report.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": ".aaaaaaaaaaaaaaa", // capped at 16 chars incl. dot
	}
	for in, want := range cases {
		if got := sanitizedExtension(in); got != want {
			t.Errorf("sanitizedExtension(%q) = %q, want %q", in, got, want)
		}
	}
}
