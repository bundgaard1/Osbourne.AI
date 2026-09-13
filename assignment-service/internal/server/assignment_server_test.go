package server_test

import (
	"context"
	"io"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	assignmentpb "osbourne.local/assignment-service/gen/assignment"
	"osbourne.local/assignment-service/internal/database"
	"osbourne.local/assignment-service/internal/repository"
	"osbourne.local/assignment-service/internal/server"
	"osbourne.local/assignment-service/internal/service"
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
	svc := service.NewAssignmentService(assignmentRepo, submissionRepo, storage)

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
