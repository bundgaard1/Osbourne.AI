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
	"osbourne.local/notification-service/gen/notification"
	"osbourne.local/notification-service/internal/consumer"
	"osbourne.local/notification-service/internal/database"
	"osbourne.local/notification-service/internal/repository"
	"osbourne.local/notification-service/internal/server"
	"osbourne.local/notification-service/internal/service"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50052"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Could not listen on port :%s: %v", port, err)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./notifications.db"
	}

	db, err := database.NewGORMDB(dbPath)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}

	notificationRepo := repository.NewGORMNotificationRepository(db)
	notificationSvc := service.NewNotificationService(notificationRepo)
	notificationGrpcServer := server.NewNotificationServer(notificationSvc)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
	}

	// Create RabbitMQ connection
	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@rabbitmq:5672/"
	}
	conn, err := rabbitmq.NewConn(amqpURL)
	if err != nil {
		log.Fatalf("Error creating RabbitMQ connection: %v", err)
	}
	defer conn.Close()

	rmqConsumer, err := consumer.NewNotificationConsumer(conn, notificationSvc)
	if err != nil {
		log.Fatalf("Error creating RabbitMQ Consumer: %v", err)
	}
	defer rmqConsumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		log.Println("Starting RabbitMQ Consumer worker...")
		if err := rmqConsumer.Start(ctx); err != nil {
			log.Printf("RabbitMQ Consumer stopped with error: %v", err)
		}
	}()

	// Start gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authcommon.AuthInterceptor(jwtSecret)),
	)
	notification.RegisterNotificationServiceServer(
		grpcServer,
		notificationGrpcServer)

	go func() {
		log.Printf("notification-service (gRPC) running on port :%s...", port)
		if err := grpcServer.Serve(lis); err != nil {
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
