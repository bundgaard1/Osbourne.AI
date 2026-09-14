package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"osbourne.local/frontend/gen/assignment"
	coursecatalogue "osbourne.local/frontend/gen/course-catalogue"
	"osbourne.local/frontend/gen/notification"
)

type enrollResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (h *Handler) HandleEnrollCourse(w http.ResponseWriter, r *http.Request) {
	courseID := r.FormValue("course_id")
	if courseID == "" {
		writeJSON(w, http.StatusBadRequest, enrollResponse{Success: false, Message: "Missing course_id"})
		return
	}

	userID := UserFromContext(r.Context()).ID

	courseRes, courseErr := h.clients.CourseCatalogue.Client.GetCourse(
		h.authCtx(r.Context()),
		&coursecatalogue.GetCourseRequest{CourseId: courseID},
	)

	_, enrollErr := h.clients.CourseCatalogue.Client.EnrollUser(h.authCtx(r.Context()),
		&coursecatalogue.EnrollUserRequest{
			UserId:   userID,
			CourseId: courseID,
		})

	if enrollErr != nil {
		log.Printf("gRPC call EnrollUser failed: %v", enrollErr)
		writeJSON(w, grpcToHTTPStatus(enrollErr), enrollResponse{Success: false, Message: "Could not complete enrollment"})
		return
	}

	title := "Enrolled successfully"
	if courseErr == nil && courseRes.GetCourse() != nil {
		title = "Enrolled in " + courseRes.GetCourse().GetTitle()
	}

	writeJSON(w, http.StatusOK, enrollResponse{Success: true, Message: title})
}

const maxUploadBytes = 10 << 20 // 10 MB

func (h *Handler) HandleSubmitAssignment(w http.ResponseWriter, r *http.Request) {
	assignmentID := chi.URLParam(r, "assignmentID")
	if assignmentID == "" {
		writeJSON(w, http.StatusBadRequest, enrollResponse{Success: false, Message: "Missing assignmentID"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	file, header, err := r.FormFile("submission_file")
	if err != nil {
		log.Printf("failed to read submission file: %v", err)
		writeJSON(w, http.StatusBadRequest, enrollResponse{Success: false, Message: "Missing submission file"})
		return
	}
	defer file.Close()

	userID := UserFromContext(r.Context()).ID

	stream, err := h.clients.Assignment.Client.SubmitAssignment(h.authCtx(r.Context()))
	if err != nil {
		log.Printf("gRPC call SubmitAssignment (open stream) failed: %v", err)
		writeJSON(w, grpcToHTTPStatus(err), enrollResponse{Success: false, Message: "Could not start upload"})
		return
	}

	if err := stream.Send(&assignment.SubmitAssignmentRequest{
		Payload: &assignment.SubmitAssignmentRequest_Metadata{
			Metadata: &assignment.SubmissionMetadata{
				AssignmentId: assignmentID,
				StudentId:    userID,
				Filename:     header.Filename,
				Size:         header.Size,
			},
		},
	}); err != nil {
		log.Printf("gRPC call SubmitAssignment (send metadata) failed: %v", err)
		writeJSON(w, http.StatusBadGateway, enrollResponse{Success: false, Message: "Could not upload file"})
		return
	}

	buf := make([]byte, 64*1024)
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&assignment.SubmitAssignmentRequest{
				Payload: &assignment.SubmitAssignmentRequest_Chunk{Chunk: buf[:n]},
			}); sendErr != nil {
				log.Printf("gRPC call SubmitAssignment (send chunk) failed: %v", sendErr)
				writeJSON(w, http.StatusBadGateway, enrollResponse{Success: false, Message: "Could not upload file"})
				return
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			log.Printf("failed while reading submission file: %v", readErr)
			writeJSON(w, http.StatusBadGateway, enrollResponse{Success: false, Message: "Could not upload file"})
			return
		}
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		log.Printf("gRPC call SubmitAssignment (finish) failed: %v", err)
		writeJSON(w, grpcToHTTPStatus(err), enrollResponse{Success: false, Message: "Could not submit assignment"})
		return
	}

	writeJSON(w, http.StatusOK, enrollResponse{Success: true, Message: "Assignment submitted successfully"})
}

func (h *Handler) HandleDownloadSubmission(w http.ResponseWriter, r *http.Request) {
	submissionID := chi.URLParam(r, "submissionID")
	if submissionID == "" {
		writeJSON(w, http.StatusBadRequest, enrollResponse{Success: false, Message: "Missing submissionID"})
		return
	}

	userID := UserFromContext(r.Context()).ID
	_ = userID

	resp, err := h.clients.Assignment.Client.DownloadSubmission(h.authCtx(r.Context()),
		&assignment.DownloadSubmissionRequest{
			SubmissionId: submissionID,
		})

	if err != nil {
		log.Printf("gRPC call DownloadSubmission failed: %v", err)
		writeJSON(w, grpcToHTTPStatus(err), enrollResponse{Success: false, Message: "Could not download submission"})
		return
	}

	msg, err := resp.Recv()

	fmt.Println(msg, err)

	if err != nil {
		if errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusNotFound, enrollResponse{Success: false, Message: "Submission has no file content"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, enrollResponse{Success: false, Message: "Could not download submission"})
		return
	}

	if metadata := msg.GetMetadata(); metadata != nil {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", metadata.GetFilename()))
		w.Header().Set("Content-Type", mimeTypeFor(metadata.GetFilename()))
		if metadata.GetSize() > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(metadata.GetSize(), 10))
		}
	}
	w.WriteHeader(http.StatusOK)

	for {
		chunk, err := resp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Printf("Error receiving chunk: %v", err)
			return
		}
		if _, writeErr := w.Write(chunk.GetChunk()); writeErr != nil {
			log.Printf("Error writing chunk to response: %v", writeErr)
			return
		}
	}

}

func (h *Handler) HandleGradeSubmission(w http.ResponseWriter, r *http.Request) {
	submissionID := chi.URLParam(r, "submissionID")
	if submissionID == "" {
		writeJSON(w, http.StatusBadRequest, enrollResponse{Success: false, Message: "Missing submissionID"})
		return
	}

	gradeStr := r.FormValue("grade")
	if gradeStr == "" {
		writeJSON(w, http.StatusBadRequest, enrollResponse{Success: false, Message: "Missing grade"})
		return
	}

	grade, err := strconv.Atoi(gradeStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, enrollResponse{Success: false, Message: "Grade must be a number"})
		return
	}

	feedback := r.FormValue("feedback")

	_, err = h.clients.Assignment.Client.GradeSubmission(h.authCtx(r.Context()),
		&assignment.GradeSubmissionRequest{
			SubmissionId: submissionID,
			Score:        int32(grade),
			Feedback:     feedback,
		})

	if err != nil {
		log.Printf("gRPC call GradeSubmission failed: %v", err)
		writeJSON(w, grpcToHTTPStatus(err), enrollResponse{Success: false, Message: "Could not grade submission"})
		return
	}

	writeJSON(w, http.StatusOK, enrollResponse{Success: true, Message: "Submission graded successfully"})
}

func (h *Handler) HandleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	notificationID := chi.URLParam(r, "notificationID")
	if notificationID == "" {
		writeJSON(w, http.StatusBadRequest, enrollResponse{Success: false, Message: "Missing notificationID"})
		return
	}

	resp, err := h.clients.Notification.Client.MarkNotificationAsRead(h.authCtx(r.Context()),
		&notification.MarkNotificationAsReadRequest{NotificationId: notificationID})
	if err != nil {
		log.Printf("gRPC call MarkNotificationAsRead failed: %v", err)
		writeJSON(w, grpcToHTTPStatus(err), enrollResponse{Success: false, Message: "Could not mark notification as read"})
		return
	}

	if !resp.GetSuccess() {
		writeJSON(w, http.StatusNotFound, enrollResponse{Success: false, Message: "Notification not found"})
		return
	}

	writeJSON(w, http.StatusOK, enrollResponse{Success: true, Message: "Notification marked as read"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode JSON response: %v", err)
	}
}

// mimeTypeFor resolves a Content-Type from a filename's extension, falling
// back to octet-stream for unknown or extension-less names.
func mimeTypeFor(filename string) string {
	if ct := mime.TypeByExtension(filepath.Ext(filename)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
