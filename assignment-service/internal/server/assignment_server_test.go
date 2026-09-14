package server_test

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	assignmentpb "osbourne.local/assignment-service/gen/assignment"
	"osbourne.local/assignment-service/internal/database"
	"osbourne.local/assignment-service/internal/domain"
	"osbourne.local/assignment-service/internal/repository"
	"osbourne.local/assignment-service/internal/server"
	"osbourne.local/assignment-service/internal/service"
	"osbourne.local/auth-common"
)

func startAssignmentServer(t *testing.T) (assignmentpb.AssignmentServiceClient, *repository.GORMSubmissionRepository, *repository.LocalFileStorage) {
	t.Helper()

	ctx := context.Background()

	db, err := database.NewGORMDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	storage, err := repository.NewLocalFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("failed to init file storage: %v", err)
	}
	assignmentRepo := repository.NewGORMAssignmentRepository(db)
	submissionRepo := repository.NewGORMSubmissionRepository(db)
	svc := service.NewAssignmentService(assignmentRepo, submissionRepo, storage, nil)

	grpcServer := grpc.NewServer()
	assignmentpb.RegisterAssignmentServiceServer(grpcServer, server.NewAssignmentServer(svc))

	bufnet := bufconn.Listen(1024 * 1024)
	go func() {
		_ = grpcServer.Serve(bufnet)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		bufnet.Close()
	})

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return bufnet.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufconn: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return assignmentpb.NewAssignmentServiceClient(conn), submissionRepo, storage
}

func createUploadedSubmission(t *testing.T, client assignmentpb.AssignmentServiceClient, content string) *assignmentpb.Submission {
	t.Helper()

	ctx := context.Background()

	created, err := client.CreateAssignment(ctx, &assignmentpb.CreateAssignmentRequest{
		CourseId: "course_1",
		Title:    "HW1",
	})
	if err != nil {
		t.Fatalf("CreateAssignment failed: %v", err)
	}

	stream, err := client.SubmitAssignment(ctx)
	if err != nil {
		t.Fatalf("SubmitAssignment failed to open stream: %v", err)
	}

	if err := stream.Send(&assignmentpb.SubmitAssignmentRequest{
		Payload: &assignmentpb.SubmitAssignmentRequest_Metadata{
			Metadata: &assignmentpb.SubmissionMetadata{
				AssignmentId: created.GetAssignment().GetId(),
				StudentId:    "student_1",
				Filename:     "answer.pdf",
				Size:         int64(len(content)),
			},
		},
	}); err != nil {
		t.Fatalf("failed to send metadata: %v", err)
	}

	chunk := []byte(content)
	for i := 0; i < len(chunk); i += 3 {
		end := i + 3
		if end > len(chunk) {
			end = len(chunk)
		}
		if err := stream.Send(&assignmentpb.SubmitAssignmentRequest{
			Payload: &assignmentpb.SubmitAssignmentRequest_Chunk{
				Chunk: chunk[i:end],
			},
		}); err != nil {
			t.Fatalf("failed to send chunk: %v", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("SubmitAssignment failed: %v", err)
	}
	return resp.GetSubmission()
}

func collectDownload(t *testing.T, client assignmentpb.AssignmentServiceClient, submissionID string) (*assignmentpb.DownloadSubmissionMetadata, []byte) {
	t.Helper()

	stream, err := client.DownloadSubmission(context.Background(),
		&assignmentpb.DownloadSubmissionRequest{SubmissionId: submissionID})
	if err != nil {
		t.Fatalf("DownloadSubmission failed to open stream: %v", err)
	}

	msg, err := stream.Recv()
	if err != nil {
		t.Fatalf("DownloadSubmission failed on first Recv: %v", err)
	}

	var metadata *assignmentpb.DownloadSubmissionMetadata
	switch m := msg.GetPayload().(type) {
	case *assignmentpb.DownloadSubmissionResponse_Metadata:
		metadata = m.Metadata
	default:
		t.Fatal("expected DownloadSubmissionMetadata as the first message")
	}

	var got []byte
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("DownloadSubmission failed while reading chunks: %v", err)
		}
		got = append(got, msg.GetChunk()...)
	}

	return metadata, got
}

func TestDownloadSubmissionEndToEnd(t *testing.T) {
	client, _, _ := startAssignmentServer(t)

	content := "hello assignment file"
	sub := createUploadedSubmission(t, client, content)

	metadata, got := collectDownload(t, client, sub.GetId())

	if metadata.GetFilename() != "answer.pdf" {
		t.Errorf("filename mismatch: %q", metadata.GetFilename())
	}
	if metadata.GetSize() != int64(len(content)) {
		t.Errorf("size mismatch: %d, want %d", metadata.GetSize(), len(content))
	}
	if string(got) != content {
		t.Errorf("downloaded content mismatch: %q, want %q", got, content)
	}
}

func TestDownloadSubmissionNotFound(t *testing.T) {
	client, _, _ := startAssignmentServer(t)

	stream, err := client.DownloadSubmission(context.Background(),
		&assignmentpb.DownloadSubmissionRequest{SubmissionId: "does-not-exist"})
	if err != nil {
		t.Fatalf("DownloadSubmission failed to open stream: %v", err)
	}

	_, err = stream.Recv()
	statusErr, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if statusErr.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", statusErr.Code())
	}
}

func TestDownloadSubmissionMissingFile(t *testing.T) {
	client, submissionRepo, storage := startAssignmentServer(t)

	content := "file to be deleted"
	sub := createUploadedSubmission(t, client, content)

	domainSubmission, err := submissionRepo.GetByID(context.Background(), sub.GetId())
	if err != nil || domainSubmission == nil {
		t.Fatalf("expected seeded submission row, got %v / %v", domainSubmission, err)
	}
	if err := storage.Delete(context.Background(), domainSubmission.FileID); err != nil {
		t.Fatalf("failed to remove file from storage: %v", err)
	}

	stream, err := client.DownloadSubmission(context.Background(),
		&assignmentpb.DownloadSubmissionRequest{SubmissionId: sub.GetId()})
	if err != nil {
		t.Fatalf("DownloadSubmission failed to open stream: %v", err)
	}

	_, err = stream.Recv()
	if err == nil {
		t.Fatal("expected an error when the stored file cannot be opened")
	}
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound when the file is missing, got %v", status.Code(err))
	}
}

func TestUploadSubmissionEndToEnd(t *testing.T) {
	client, _, _ := startAssignmentServer(t)

	content := "hello assignment file"
	sub := createUploadedSubmission(t, client, content)

	if sub.GetId() == "" {
		t.Error("expected a submission id")
	}
	if sub.GetFilename() != "answer.pdf" {
		t.Errorf("file name mismatch: %q", sub.GetFilename())
	}
	if sub.GetSize() != int64(len(content)) {
		t.Errorf("file size mismatch: %d", sub.GetSize())
	}
}

func TestListMySubmissionsReturnsOnlyCallerSubmissions(t *testing.T) {
	ctx := context.Background()

	db, err := database.NewGORMDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	storage, err := repository.NewLocalFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("failed to init file storage: %v", err)
	}
	assignmentRepo := repository.NewGORMAssignmentRepository(db)
	submissionRepo := repository.NewGORMSubmissionRepository(db)
	svc := service.NewAssignmentService(assignmentRepo, submissionRepo, storage, nil)
	grpcServerImpl := server.NewAssignmentServer(svc)

	if err := svc.CreateAssignment(ctx, &domain.Assignment{ID: "a1", CourseID: "c1", Title: "HW1"}); err != nil {
		t.Fatalf("CreateAssignment failed: %v", err)
	}

	makeSubmission := func(studentID string) {
		t.Helper()
		if _, err := svc.SubmitAssignment(ctx, service.SubmitAssignmentInput{
			AssignmentID: "a1",
			StudentID:    studentID,
			FileName:     "answer.pdf",
		}, strings.NewReader("file "+studentID)); err != nil {
			t.Fatalf("SubmitAssignment failed: %v", err)
		}
	}

	// Submit three times as the caller and once as another student.
	makeSubmission("student_1")
	makeSubmission("student_2")
	makeSubmission("student_1")
	makeSubmission("student_1")

	resp, err := grpcServerImpl.ListMySubmissions(
		authcommon.WithClaims(ctx, &authcommon.Claims{UserID: "student_1", Role: "student"}),
		&assignmentpb.ListMySubmissionsRequest{AssignmentId: "a1"},
	)
	if err != nil {
		t.Fatalf("ListMySubmissions failed: %v", err)
	}

	if len(resp.GetSubmissions()) != 3 {
		t.Fatalf("expected 3 submissions for student_1, got %d", len(resp.GetSubmissions()))
	}
	for _, s := range resp.GetSubmissions() {
		if s.GetStudentId() != "student_1" {
			t.Errorf("expected only student_1 submissions, got %q", s.GetStudentId())
		}
	}

	// Newest first.
	if got := resp.GetSubmissions()[0].GetSubmittedAt().AsTime(); got.Before(resp.GetSubmissions()[2].GetSubmittedAt().AsTime()) {
		t.Errorf("expected newest-first order, got first=%v before last=%v", got, resp.GetSubmissions()[2].GetSubmittedAt().AsTime())
	}
}

func TestListMySubmissionsRequiresClaims(t *testing.T) {
	ctx := context.Background()

	db, err := database.NewGORMDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	storage, err := repository.NewLocalFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("failed to init file storage: %v", err)
	}
	svc := service.NewAssignmentService(
		repository.NewGORMAssignmentRepository(db),
		repository.NewGORMSubmissionRepository(db),
		storage,
		nil,
	)
	grpcServerImpl := server.NewAssignmentServer(svc)

	_, err = grpcServerImpl.ListMySubmissions(ctx, &assignmentpb.ListMySubmissionsRequest{AssignmentId: "a1"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without claims, got %v", err)
	}
}
