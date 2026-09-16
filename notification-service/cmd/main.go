package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wagslane/go-rabbitmq"
	"google.golang.org/grpc"
	authcommon "osbourne.local/auth-common"
	"osbourne.local/notification-service/gen/notification"
	"osbourne.local/notification-service/internal/consumer"
	"osbourne.local/notification-service/internal/database"
	"osbourne.local/notification-service/internal/repository"
	"osbourne.local/notification-service/internal/server"
	"osbourne.local/notification-service/internal/service"
)

func main() {
	authcommon.SetupLogging("notification-service")

	port := os.Getenv("PORT")
	if port == "" {
		port = "50052"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("could not listen", "port", port, "err", err)
		os.Exit(1)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./notifications.db"
	}

	db, err := database.NewGORMDB(dbPath)
	if err != nil {
		slog.Error("database error", "err", err)
		os.Exit(1)
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
		slog.Error("error creating RabbitMQ connection", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	rmqConsumer, err := consumer.NewNotificationConsumer(conn, notificationSvc)
	if err != nil {
		slog.Error("error creating RabbitMQ consumer", "err", err)
		os.Exit(1)
	}
	defer rmqConsumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		slog.Info("starting RabbitMQ consumer worker")
		if err := rmqConsumer.Start(ctx); err != nil {
			slog.Error("rabbitMQ consumer stopped with error", "err", err)
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
		slog.Info("notification-service (gRPC) running", "port", port)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("error while running gRPC server", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("received shutdown signal, shutting down gracefully")

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("gRPC server shut down gracefully")
	case <-time.After(5 * time.Second):
		slog.Warn("timeout exceeded, forcing shutdown")
		grpcServer.Stop()
	}
}