package server_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	assignmentpb "osbourne.local/assignment-service/gen/assignment"
	"osbourne.local/assignment-service/internal/database"
	"osbourne.local/assignment-service/internal/repository"
	"osbourne.local/assignment-service/internal/server"
	"osbourne.local/assignment-service/internal/service"
)

func TestUploadSubmissionEndToEnd(t *testing.T) {
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
	defer bufnet.Close()
	go func() {
		_ = grpcServer.Serve(bufnet)
	}()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return bufnet.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufconn: %v", err)
	}
	defer conn.Close()

	client := assignmentpb.NewAssignmentServiceClient(conn)

	created, err := client.CreateAssignment(ctx, &assignmentpb.CreateAssignmentRequest{
		CourseId: "course_1",
		Title:    "HW1",
	})
	if err != nil {
		t.Fatalf("CreateAssignment failed: %v", err)
	}
	if created.GetAssignment().GetId() == "" {
		t.Fatal("expected a generated assignment id")
	}

	content := "hello assignment file"
	stream, err := client.SubmitAssignment(ctx)
	if err != nil {
		t.Fatalf("UploadSubmission failed to open stream: %v", err)
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

	sub := resp.GetSubmission()
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
