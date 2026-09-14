package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/grpc"

	"osbourne.local/auth-common"
	"osbourne.local/profile-service/gen/profile"
	"osbourne.local/profile-service/internal/consumer"
	"osbourne.local/profile-service/internal/database"
	"osbourne.local/profile-service/internal/repository"
	"osbourne.local/profile-service/internal/server"
	"osbourne.local/profile-service/internal/service"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Could not listen on port :%s: %v", port, err)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./profiles.db"
	}

	db, err := database.NewGORMDB(dbPath)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}

	profileRepo := repository.NewGORMProfileRepository(db)
	profileSvc := service.NewProfileService(profileRepo)
	profileGrpcServer := server.NewProfileServer(profileSvc)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	// RabbitMQ consumer: profiles are created reactively from account.created
	// events, so subscribe before serving gRPC.
	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@rabbitmq:5672/"
	}
	conn, err := rabbitmq.NewConn(amqpURL)
	if err != nil {
		log.Fatalf("Error creating RabbitMQ connection: %v", err)
	}
	defer conn.Close()

	profileConsumer, err := consumer.NewProfileConsumer(conn, profileSvc)
	if err != nil {
		log.Fatalf("Error creating profile consumer: %v", err)
	}
	defer profileConsumer.Close()

	go func() {
		if err := profileConsumer.Start(context.Background()); err != nil {
			log.Printf("Profile consumer stopped: %v", err)
		}
	}()

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authcommon.AuthInterceptor(jwtSecret)),
	)
	profile.RegisterProfileServiceServer(grpcServer, profileGrpcServer)

	go func() {
		log.Printf("profile-service (gRPC) running on port :%s...", port)
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Fatalf("Error while running gRPC server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Received shutdown signal. Shutting down gracefully...")

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Println("gRPC server shut down gracefully.")
	case <-time.After(5 * time.Second):
		log.Println("Timeout exceeded - forcing shutdown.")
		grpcServer.Stop()
	}
}