package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	coursecontent "osbourne.local/course-content-service/gen/course-content"
	"osbourne.local/course-content-service/internal/database"
	"osbourne.local/course-content-service/internal/repository"
	"osbourne.local/course-content-service/internal/server"
	"osbourne.local/course-content-service/internal/service"
)

func main() {

	port := "50054"
	if port == "" {
		port = "50054"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Could not listen on port %s: %v", port, err)
	}

	dbDir := "content-db"
	if envDbDir := os.Getenv("NOSQL_PATH"); envDbDir != "" {
		dbDir = envDbDir
	}

	cloverdb, err := database.NewCloverDB(dbDir)
	if err != nil {
		log.Fatalf("Failed to connect to CloverDB: %v", err)
	}
	err = database.SeedCloverData(cloverdb)
	if err != nil {
		log.Fatalf("Failed to seed CloverDB: %v", err)
	}

	courseContentRepo := repository.NewCloverModuleRepository(cloverdb, "modules")
	courseContentSvc := service.NewModuleService(courseContentRepo)
	courseContentGrpcServer := server.NewContentServer(courseContentSvc)

	grpcServer := grpc.NewServer()

	coursecontent.RegisterCourseContentServiceServer(
		grpcServer,
		courseContentGrpcServer)

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
