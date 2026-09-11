package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	coursecatalogue "osbourne.local/course-catalogue-service/gen/course-catalogue"
	"osbourne.local/course-catalogue-service/internal/database"
	"osbourne.local/course-catalogue-service/internal/repository"
	"osbourne.local/course-catalogue-service/internal/server"
	"osbourne.local/course-catalogue-service/internal/service"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50053"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Could not listen on port %s: %v", port, err)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./courses.db"
	}

	db, err := database.NewGORMDB(dbPath)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}

	database.SeedData(db)

	coursecatalogueRepo := repository.NewGORMCourseCatalogueRepository(db)
	coursecatalogueSvc := service.NewCourseService(coursecatalogueRepo)
	coursecatalogueGrpcServer := server.NewCourseServer(coursecatalogueSvc)

	grpcServer := grpc.NewServer()

	coursecatalogue.RegisterCourseCatalogueServiceServer(
		grpcServer,
		coursecatalogueGrpcServer)

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
