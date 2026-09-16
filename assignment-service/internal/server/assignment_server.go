package server

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	assignmentpb "osbourne.local/assignment-service/gen/assignment"
	"osbourne.local/assignment-service/internal/domain"
	"osbourne.local/assignment-service/internal/repository"
	"osbourne.local/assignment-service/internal/service"
	authcommon "osbourne.local/auth-common"
)

type AssignmentServer struct {
	assignmentpb.AssignmentServiceServer
	svc *service.AssignmentService
}

func NewAssignmentServer(svc *service.AssignmentService) *AssignmentServer {
	return &AssignmentServer{
		svc: svc,
	}
}

func (s *AssignmentServer) CreateAssignment(ctx context.Context, req *assignmentpb.CreateAssignmentRequest) (*assignmentpb.CreateAssignmentResponse, error) {
	assignment := &domain.Assignment{
		CourseID:    req.GetCourseId(),
		Title:       req.GetTitle(),
		Description: req.GetDescription(),
	}
	if req.GetDueDate() != nil {
		assignment.DueDate = req.GetDueDate().AsTime()
	}

	if err := s.svc.CreateAssignment(ctx, assignment); err != nil {
		return nil, err
	}

	return &assignmentpb.CreateAssignmentResponse{
		Assignment: toProtoAssignment(assignment),
	}, nil
}

func (s *AssignmentServer) GetCourseAssignments(ctx context.Context, req *assignmentpb.GetCourseAssignmentsRequest) (*assignmentpb.GetCourseAssignmentsResponse, error) {
	assignments, err := s.svc.GetCourseAssignments(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	protoAssignments := make([]*assignmentpb.Assignment, len(assignments))
	for i, assignment := range assignments {
		protoAssignments[i] = toProtoAssignment(assignment)
	}

	return &assignmentpb.GetCourseAssignmentsResponse{
		Assignments: protoAssignments,
	}, nil
}

func (s *AssignmentServer) GetAssignment(ctx context.Context, req *assignmentpb.GetAssignmentRequest) (*assignmentpb.GetAssignmentResponse, error) {
	assignment, err := s.svc.GetAssignment(ctx, req.GetAssignmentId())
	if err != nil {
		return nil, err
	}
	if assignment == nil {
		return nil, status.Error(codes.NotFound, "assignment not found")
	}

	return &assignmentpb.GetAssignmentResponse{
		Assignment: toProtoAssignment(assignment),
	}, nil
}

func (s *AssignmentServer) SubmitAssignment(stream assignmentpb.AssignmentService_SubmitAssignmentServer) error {
	first, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "expected metadata as first message: %v", err)
	}

	meta := first.GetMetadata()
	if meta == nil {
		return status.Error(codes.InvalidArgument, "first message must contain metadata")
	}
	if meta.GetAssignmentId() == "" || meta.GetStudentId() == "" || meta.GetFilename() == "" {
		return status.Error(codes.InvalidArgument, "metadata must contain assignment_id, student_id and filename")
	}

	// Fail fast before consuming the stream so an invalid assignment does not
	// leave a stuck pipe reader in the service.
	assignment, err := s.svc.GetAssignment(stream.Context(), meta.GetAssignmentId())
	if err != nil {
		return err
	}
	if assignment == nil {
		return status.Error(codes.NotFound, "assignment not found")
	}

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		for {
			req, rerr := stream.Recv()
			if rerr == io.EOF {
				return
			}
			if rerr != nil {
				pw.CloseWithError(rerr)
				return
			}
			if _, werr := pw.Write(req.GetChunk()); werr != nil {
				pw.CloseWithError(werr)
				return
			}
		}
	}()

	submission, err := s.svc.SubmitAssignment(stream.Context(), service.SubmitAssignmentInput{
		AssignmentID: meta.GetAssignmentId(),
		StudentID:    meta.GetStudentId(),
		FileName:     meta.GetFilename(),
		Size:         meta.GetSize(),
	}, pr)
	if err != nil {
		return err
	}

	return stream.SendAndClose(&assignmentpb.SubmitAssignmentResponse{
		Submission: toProtoSubmission(submission),
	})
}

func (s *AssignmentServer) GetSubmission(ctx context.Context, req *assignmentpb.GetSubmissionRequest) (*assignmentpb.GetSubmissionResponse, error) {
	submission, err := s.svc.GetSubmission(ctx, req.GetSubmissionId())
	if err != nil {
		return nil, err
	}
	if submission == nil {
		return nil, status.Error(codes.NotFound, "submission not found")
	}

	return &assignmentpb.GetSubmissionResponse{
		Submission: toProtoSubmission(submission),
	}, nil
}

func (s *AssignmentServer) DownloadSubmission(req *assignmentpb.DownloadSubmissionRequest, stream assignmentpb.AssignmentService_DownloadSubmissionServer) error {
	submission, err := s.svc.GetSubmission(stream.Context(), req.GetSubmissionId())
	if err != nil {
		return err
	}
	if submission == nil {
		return status.Error(codes.NotFound, "submission not found")
	}

	rc, err := s.svc.FileStorage().Get(stream.Context(), submission.FileID)
	if err != nil {
		code := codes.Internal
		if errors.Is(err, repository.ErrNotFound) {
			code = codes.NotFound
		}
		return status.Errorf(code, "failed to open file: %v", err)
	}
	defer rc.Close()

	// Send metadata first
	if serr := stream.Send(&assignmentpb.DownloadSubmissionResponse{
		Payload: &assignmentpb.DownloadSubmissionResponse_Metadata{
			Metadata: &assignmentpb.DownloadSubmissionMetadata{
				Filename: submission.FileName,
				Size:     submission.FileSize,
			},
		},
	}); serr != nil {
		slog.ErrorContext(stream.Context(), "failed to send download metadata", "submission_id", submission.ID, "err", serr)
		return status.Errorf(codes.Internal, "failed to send metadata: %v", serr)
	}
	slog.InfoContext(stream.Context(), "sent download metadata", "submission_id", submission.ID, "filename", submission.FileName, "size", submission.FileSize)

	// Send file content in chunks
	buf := make([]byte, 1024)
	for {
		n, rerr := rc.Read(buf)
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return status.Errorf(codes.Internal, "failed to read file: %v", rerr)
		}

		if serr := stream.Send(&assignmentpb.DownloadSubmissionResponse{
			Payload: &assignmentpb.DownloadSubmissionResponse_Chunk{
				Chunk: buf[:n],
			},
		}); serr != nil {
			return status.Errorf(codes.Internal, "failed to send chunk: %v", serr)
		}
	}

	return nil
}

func (s *AssignmentServer) ListSubmissions(ctx context.Context, req *assignmentpb.ListSubmissionsRequest) (*assignmentpb.ListSubmissionsResponse, error) {
	submissions, err := s.svc.ListSubmissionsByAssignment(ctx, req.GetAssignmentId())
	if err != nil {
		return nil, err
	}

	return &assignmentpb.ListSubmissionsResponse{
		Submissions: toProtoSubmissions(submissions),
	}, nil
}

// ListMySubmissions returns only the calling student's submissions for an
// assignment. The student identity is read from the authenticated JWT claims
// injected by the auth interceptor, never from the request payload.
func (s *AssignmentServer) ListMySubmissions(ctx context.Context, req *assignmentpb.ListMySubmissionsRequest) (*assignmentpb.ListSubmissionsResponse, error) {
	claims, ok := authcommon.ClaimsFromContext(ctx)
	if !ok {
		return nil, authcommon.RequiresAuthentication()
	}

	submissions, err := s.svc.ListSubmissionsByStudentAndAssignment(ctx, claims.UserID, req.GetAssignmentId())
	if err != nil {
		return nil, err
	}

	return &assignmentpb.ListSubmissionsResponse{
		Submissions: toProtoSubmissions(submissions),
	}, nil
}

func (s *AssignmentServer) GradeSubmission(ctx context.Context, req *assignmentpb.GradeSubmissionRequest) (*assignmentpb.GradeSubmissionResponse, error) {
	if err := s.svc.GradeSubmission(ctx, req.GetSubmissionId(), int(req.GetScore()), req.GetFeedback()); err != nil {
		return nil, err
	}

	return &assignmentpb.GradeSubmissionResponse{
		Success: true,
	}, nil
}

func toProtoAssignment(assignment *domain.Assignment) *assignmentpb.Assignment {
	return &assignmentpb.Assignment{
		Id:          assignment.ID,
		CourseId:    assignment.CourseID,
		Title:       assignment.Title,
		Description: assignment.Description,
		DueDate:     timestamppb.New(assignment.DueDate),
	}
}

func toProtoSubmissions(submissions []*domain.Submission) []*assignmentpb.Submission {
	protoSubmissions := make([]*assignmentpb.Submission, len(submissions))
	for i, submission := range submissions {
		protoSubmissions[i] = toProtoSubmission(submission)
	}
	return protoSubmissions
}

func toProtoSubmission(submission *domain.Submission) *assignmentpb.Submission {
	var scorePtr *int32
	if submission.Graded { // or if your domain model has submission.Score != nil
		scorePtr = proto.Int32(int32(submission.Score))
	}

	var feedbackPtr *string
	if submission.Feedback != "" {
		feedbackPtr = proto.String(submission.Feedback)
	}

	return &assignmentpb.Submission{
		Id:           submission.ID,
		AssignmentId: submission.AssignmentID,
		StudentId:    submission.StudentID,
		Filename:     submission.FileName,
		Size:         submission.FileSize,
		SubmittedAt:  timestamppb.New(submission.SubmittedAt),
		Graded:       submission.Graded,
		Score:        scorePtr,
		Feedback:     feedbackPtr,
	}
}
