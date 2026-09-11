package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	assignmentpb "osbourne.local/assignment-service/gen/assignment"
	"osbourne.local/assignment-service/internal/database"
	"osbourne.local/assignment-service/internal/repository"
	"osbourne.local/assignment-service/internal/server"
	"osbourne.local/assignment-service/internal/service"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50055"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Could not listen on port %s: %v", port, err)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./assignment.db"
	}

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./data/uploads"
	}

	db, err := database.NewGORMDB(dbPath)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	err = database.SeedGORMData(db)
	if err != nil {
		log.Fatalf("Could not seed database: %v", err)
	}

	fileStore, err := repository.NewLocalFileStorage(uploadDir)
	if err != nil {
		log.Fatalf("Could not initialize file storage: %v", err)
	}

	assignmentRepo := repository.NewGORMAssignmentRepository(db)
	submissionRepo := repository.NewGORMSubmissionRepository(db)
	assignmentSvc := service.NewAssignmentService(assignmentRepo, submissionRepo, fileStore)
	assignmentGrpcServer := server.NewAssignmentServer(assignmentSvc)

	grpcServer := grpc.NewServer()

	assignmentpb.RegisterAssignmentServiceServer(grpcServer, assignmentGrpcServer)

	go func() {
		log.Printf("Starting gRPC server on port %s...", port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down gRPC server...")

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Println("gRPC server stopped gracefully.")
	case <-time.After(5 * time.Second):
		log.Println("Timeout reached. Forcing gRPC server shutdown.")
		grpcServer.Stop()
	}
}
